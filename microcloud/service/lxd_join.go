package service

import (
	"fmt"
	"net/http"

	"github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared"
	"github.com/lxc/lxd/shared/api"
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
