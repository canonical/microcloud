package api

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/trust"
	"github.com/canonical/lxd/shared/version"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/database"
	"github.com/canonical/microcloud/microcloud/service"
)

// HMACClusterManager10 is the HMAC format version used for registering with a join token in cluster manager.
const HMACClusterManager10 trust.HMACVersion = "ClusterManager-1.0"

// The ClusterManagerClient struct is used to interact with the cluster manager.
type ClusterManagerClient struct {
	config *database.ClusterManager
}

// NewClusterManagerClient returns a new ClusterManagerClient.
func NewClusterManagerClient(config *database.ClusterManager) *ClusterManagerClient {
	return &ClusterManagerClient{
		config: config,
	}
}

// PostStatus sends the status of MicroCloud to the cluster manager.
func (c *ClusterManagerClient) PostStatus(sh *service.Handler, status types.ClusterManagerStatusPost) error {
	reqBody, err := json.Marshal(status)
	if err != nil {
		return errors.New("Failed to marshal status message")
	}

	req, err := c.craftRequest("POST", "/1.0/remote-cluster/status", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}

	err = c.sendRequest(sh, req)
	if err != nil {
		return err
	}

	return nil
}

// Delete removes the server from the cluster.
func (c *ClusterManagerClient) Delete(sh *service.Handler) error {
	req, err := c.craftRequest("DELETE", "/1.0/remote-cluster", nil)
	if err != nil {
		return err
	}

	err = c.sendRequest(sh, req)
	if err != nil {
		return err
	}

	return nil
}

// PostJoin registers MicroCloud in cluster manager.
func (c *ClusterManagerClient) PostJoin(sh *service.Handler, serverName string, secret string) error {
	localCert, err := c.getLocalCert(sh)
	if err != nil {
		return err
	}

	payload := types.ClusterManagerJoinPost{
		ClusterName:        serverName,
		ClusterCertificate: string(localCert.PublicKey()),
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := c.craftRequest("POST", "/1.0/remote-cluster", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}

	// Sign the payload with a hmac, using the secret from the join token.
	h := trust.NewHMAC([]byte(secret), trust.NewDefaultHMACConf(HMACClusterManager10))
	hmacHeader, err := trust.HMACAuthorizationHeader(h, payload)
	if err != nil {
		return fmt.Errorf("Failed to create HMAC: %w", err)
	}

	req.Header.Set("Authorization", hmacHeader)

	err = c.sendRequest(sh, req)
	if err != nil {
		return err
	}

	return nil
}

func (c *ClusterManagerClient) craftRequest(method string, path string, reqBody io.Reader) (*http.Request, error) {
	// host name will be replaced with a real value in sendRequest method below
	url := "https://remote" + path
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	return req, nil
}

func (c *ClusterManagerClient) sendRequest(sh *service.Handler, req *http.Request) error {
	client, hostAddress, err := c.getHTTPClient(sh)
	if err != nil {
		return err
	}

	req.URL.Host = hostAddress

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		content := new(bytes.Buffer)
		_, err = content.ReadFrom(resp.Body)
		if err != nil {
			return fmt.Errorf("Failed to read response body: %s", err)
		}

		return fmt.Errorf("Failed to send request to cluster manager: %s, body: %s", resp.Status, content.String())
	}

	return nil
}

func (c *ClusterManagerClient) getHTTPClient(sh *service.Handler) (*http.Client, string, error) {
	client := &http.Client{}

	var address string
	var remoteCert *x509.Certificate
	var err error

	addresses := strings.Split(c.config.Addresses, ",")
	if len(addresses) == 0 {
		return nil, "", errors.New("No cluster manager addresses provided.")
	}

	// fetch remote cert and pick the first address that responds without error
	for _, address = range addresses {
		remoteCert, err = shared.GetRemoteCertificate("https://"+address, version.UserAgent)
		// found a working address, exit loop
		if err == nil {
			break
		}

		// ignore errors if we have a next address to try
		if address != addresses[len(addresses)-1] {
			err = nil
		}
	}

	if err != nil {
		return nil, "", err
	}

	// verify remote cert
	remoteFingerprint := shared.CertFingerprint(remoteCert)
	if !strings.EqualFold(remoteFingerprint, c.config.Fingerprint) {
		return nil, "", fmt.Errorf("Invalid cluster manager certificate fingerprint, expected %s, got %s", c.config.Fingerprint, remoteFingerprint)
	}

	localCert, err := c.getLocalCert(sh)
	if err != nil {
		return nil, "", err
	}

	cert := localCert.KeyPair()
	tlsConfig := shared.InitTLSConfig()
	tlsConfig.GetClientCertificate = func(info *tls.CertificateRequestInfo) (*tls.Certificate, error) {
		// GetClientCertificate is called if not nil instead of performing the default selection of an appropriate
		// certificate from the `Certificates` list. We only have one-key pair to send, and we always want to send it
		// because this is what uniquely identifies the caller to the server.
		return &cert, nil
	}

	remoteCert.IsCA = true
	remoteCert.KeyUsage = x509.KeyUsageCertSign

	tlsConfig.RootCAs = x509.NewCertPool()
	tlsConfig.RootCAs.AddCert(remoteCert)

	client.Transport = &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	return client, address, nil
}

func (c *ClusterManagerClient) getLocalCert(sh *service.Handler) (*shared.CertInfo, error) {
	cloud := sh.Services[types.MicroCloud].(*service.CloudService)
	localCert, err := cloud.ClusterCert()
	if err != nil {
		return nil, err
	}

	return localCert, nil
}
