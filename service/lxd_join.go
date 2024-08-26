package service

import (
	"context"
	"encoding/pem"
	"fmt"

	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/lxd/shared/version"
)

func (s *LXDService) configFromToken(token string) (*api.ClusterPut, error) {
	joinToken, err := shared.JoinTokenDecode(token)
	if err != nil {
		return nil, fmt.Errorf("Invalid cluster join token: %w", err)
	}

	config := &api.ClusterPut{
		Cluster:       api.Cluster{ServerName: s.name, Enabled: true},
		ServerAddress: util.CanonicalNetworkAddress(s.address, s.port),
	}

	ok, err := s.HasExtension(context.Background(), s.Name(), s.Address(), nil, "explicit_trust_token")
	if err != nil {
		return nil, err
	}

	if ok {
		config.ClusterToken = token
	} else {
		config.ClusterPassword = token
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
