package service

import (
	"encoding/pem"
	"fmt"
	"net/http"

	"github.com/lxc/lxd/client"
	"github.com/lxc/lxd/lxd/util"
	"github.com/lxc/lxd/shared"
	"github.com/lxc/lxd/shared/api"
	"github.com/lxc/lxd/shared/logger"
	"github.com/lxc/lxd/shared/version"
)

// SetupTrust is a convenience around InstanceServer.CreateCertificate that adds the given server certificate to
// the trusted pool of the cluster at the given address, using the given password. The certificate is added as
// type CertificateTypeServer to allow intra-member communication. If a certificate with the same fingerprint
// already exists with a different name or type, then no error is returned.
func SetupTrust(serverCert *shared.CertInfo, serverName string, targetAddress string, targetCert string, targetPassword string) error {
	// Connect to the target cluster node.
	args := &lxd.ConnectionArgs{
		TLSServerCert: targetCert,
		UserAgent:     version.UserAgent,
	}

	target, err := lxd.ConnectLXD(fmt.Sprintf("https://%s", targetAddress), args)
	if err != nil {
		return fmt.Errorf("Failed to connect to target cluster node %q: %w", targetAddress, err)
	}

	cert, err := shared.GenerateTrustCertificate(serverCert, serverName)
	if err != nil {
		return fmt.Errorf("Failed generating trust certificate: %w", err)
	}

	post := api.CertificatesPost{
		CertificatePut: cert.CertificatePut,
		Password:       targetPassword,
	}

	err = target.CreateCertificate(post)
	if err != nil && !api.StatusErrorCheck(err, http.StatusConflict) {
		return fmt.Errorf("Failed to add server cert to cluster: %w", err)
	}

	return nil
}

func (s *LXDService) configFromToken(token string) (*api.ClusterPut, error) {
	joinToken, err := shared.JoinTokenDecode(token)
	if err != nil {
		return nil, fmt.Errorf("Invalid cluster join token: %w", err)
	}
	config := &api.ClusterPut{
		Cluster:       api.Cluster{ServerName: s.name, Enabled: true},
		ServerAddress: util.CanonicalNetworkAddress(s.address, s.port),
	}

	// Attempt to find a working cluster member to use for joining by retrieving the
	// cluster certificate from each address in the join token until we succeed.
	for _, clusterAddress := range joinToken.Addresses {
		// Cluster URL
		config.ClusterAddress = util.CanonicalNetworkAddress(clusterAddress, s.port)

		// Cluster certificate
		cert, err := shared.GetRemoteCertificate(fmt.Sprintf("https://%s", config.ClusterAddress), version.UserAgent)
		if err != nil {
			logger.Warnf("Error connecting to existing cluster member %q: %v\n", clusterAddress, err)
			continue
		}

		certDigest := shared.CertFingerprint(cert)
		if joinToken.Fingerprint != certDigest {
			return nil, fmt.Errorf("Certificate fingerprint mismatch between join token and cluster member %q", clusterAddress)
		}

		config.ClusterCertificate = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))

		break // We've found a working cluster member.
	}

	if config.ClusterCertificate == "" {
		return nil, fmt.Errorf("Unable to connect to any of the cluster members specified in join token")
	}

	return config, nil
}
