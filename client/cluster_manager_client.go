package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/version"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/database"
)

// RemoteClusterPath is the path to the cluster manager API.
const RemoteClusterPath = "/1.0/remote-cluster"

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

// Join registers MicroCloud in cluster manager.
func (c *ClusterManagerClient) Join(clusterCert *shared.CertInfo, clusterName string, encodedToken string) error {
	payload := types.ClusterManagerJoin{
		ClusterName:        clusterName,
		ClusterCertificate: string(clusterCert.PublicKey()),
		Token:              encodedToken,
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := c.craftRequest("POST", RemoteClusterPath, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}

	return c.sendRequest(clusterCert, req)
}

// PostStatus sends the status of MicroCloud to the cluster manager.
func (c *ClusterManagerClient) PostStatus(clusterCert *shared.CertInfo, status types.ClusterManagerPostStatus) error {
	reqBody, err := json.Marshal(status)
	if err != nil {
		return errors.New("Failed to marshal status message")
	}

	req, err := c.craftRequest("POST", RemoteClusterPath+"/status", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}

	err = c.sendRequest(clusterCert, req)

	return err
}

// Delete removes a MicroCloud from cluster manager.
func (c *ClusterManagerClient) Delete(clusterCert *shared.CertInfo) error {
	req, err := c.craftRequest("DELETE", RemoteClusterPath, nil)
	if err != nil {
		return err
	}

	err = c.sendRequest(clusterCert, req)

	return err
}

func (c *ClusterManagerClient) craftRequest(method string, path string, reqBody io.Reader) (*http.Request, error) {
	url := "https://remote" + path // remote is a placeholder, real address will be set in sendRequest
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	return req, nil
}

func (c *ClusterManagerClient) sendRequest(clusterCert *shared.CertInfo, req *http.Request) error {
	client, hostAddress, err := c.getHTTPClient(clusterCert)
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

func (c *ClusterManagerClient) getHTTPClient(clusterCert *shared.CertInfo) (*http.Client, string, error) {
	client := &http.Client{}

	var address string
	var remoteCert *x509.Certificate
	var err error

	addresses := strings.Split(c.config.Addresses, ",")
	if len(addresses) == 0 {
		return nil, "", errors.New("No cluster manager addresses")
	}

	// fetch remote cert and pick the first address that succeeds a connection
	for _, address = range addresses {
		remoteCert, err = shared.GetRemoteCertificate(context.TODO(), "https://"+address, version.UserAgent)
		// found a succeeding address, exit loop
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
	if !strings.EqualFold(remoteFingerprint, c.config.CertificateFingerprint) {
		return nil, "", fmt.Errorf("Invalid cluster manager certificate fingerprint, expected %s, got %s", c.config.CertificateFingerprint, remoteFingerprint)
	}

	remoteCert.IsCA = true
	remoteCert.KeyUsage = x509.KeyUsageCertSign

	tlsConfig := shared.InitTLSConfig()
	tlsConfig.RootCAs = x509.NewCertPool()
	tlsConfig.RootCAs.AddCert(remoteCert)

	cert := clusterCert.KeyPair()
	tlsConfig.GetClientCertificate = func(info *tls.CertificateRequestInfo) (*tls.Certificate, error) {
		// GetClientCertificate is called if not nil instead of performing the default selection of an appropriate
		// certificate from the `Certificates` list. We only have one-key pair to send, and we always want to send it
		// because this is what uniquely identifies the caller to the server.
		return &cert, nil
	}

	client.Transport = &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	return client, address, nil
}
