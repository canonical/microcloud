package main

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/canonical/lxd/shared"

	"github.com/canonical/microcloud/microcloud/api/types"
	cloudClient "github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcloud/microcloud/mdns"
	"github.com/canonical/microcloud/microcloud/service"
)

type SessionFunc func(gw *cloudClient.WebsocketGateway) error

func (c *initConfig) runSession(ctx context.Context, s *service.Handler, role types.SessionRole, timeout time.Duration, f SessionFunc) error {
	cloud := s.Services[types.MicroCloud].(*service.CloudService)
	conn, err := cloud.StartSession(ctx, string(role), timeout)
	if err != nil {
		return err
	}

	defer conn.Close()

	return f(cloudClient.NewWebsocketGateway(ctx, conn))
}

func (c *initConfig) initiatingSession(gw *cloudClient.WebsocketGateway, sh *service.Handler, services []types.ServiceType, passphrase string, expectedSystems []string) error {
	session := types.Session{
		Address:    c.address,
		Interface:  c.lookupIface.Name,
		Services:   services,
		Passphrase: passphrase,
	}

	err := gw.Write(session)
	if err != nil {
		return fmt.Errorf("Failed to send session start: %w", err)
	}

	err = gw.ReceiveWithContext(gw.Context(), &session)
	if err != nil {
		return fmt.Errorf("Failed to read session reply: %w", err)
	}

	if !c.autoSetup {
		cloud := sh.Services[types.MicroCloud].(*service.CloudService)

		// If the cluster is already bootstrapped the cluster certificate is used
		// instead for the server.
		// If a joiner sends it's intent to join our existing cluster it extracts
		// the certificates fingerprint out of the underlying connection.
		// When adding new systems the fingerprint displayed here has to be the
		// one from the cluster certificate as this one is used as soon as the
		// cluster is bootstrapped.
		var cert *shared.CertInfo
		if c.bootstrap {
			cert, err = cloud.ServerCert()
			if err != nil {
				return err
			}
		} else {
			cert, err = cloud.ClusterCert()
			if err != nil {
				return err
			}
		}

		fingerprint, err := c.shortFingerprint(cert.Fingerprint())
		if err != nil {
			return fmt.Errorf("Failed to shorten fingerprint: %w", err)
		}

		fmt.Printf("Use the following command on systems that you want to join the cluster:\n\n microcloud join\n\n")
		fmt.Printf("When requested enter the passphrase:\n\n %s\n\n", session.Passphrase)
		fmt.Printf("Verify the fingerprint %q is displayed on joining systems.\n", fingerprint)
		fmt.Println("Waiting to detect systems ...")
	}

	confirmedIntents, err := c.askJoinIntents(gw, expectedSystems)
	if err != nil {
		return err
	}

	err = gw.Write(types.Session{
		ConfirmedIntents: confirmedIntents,
	})
	if err != nil {
		return fmt.Errorf("Failed to send join intents: %w", err)
	}

	err = gw.ReceiveWithContext(gw.Context(), &session)
	if err != nil {
		return fmt.Errorf("Failed to read confirmation errors: %w", err)
	}

	if !session.Accepted {
		return fmt.Errorf("Join confirmations didn't get accepted on all systems")
	}

	for _, joinIntent := range confirmedIntents {
		certBlock, _ := pem.Decode([]byte(joinIntent.Certificate))
		if certBlock == nil {
			return fmt.Errorf("Invalid certificate file")
		}

		remoteCert, err := x509.ParseCertificate(certBlock.Bytes)
		if err != nil {
			return fmt.Errorf("Failed to parse certificate: %w", err)
		}

		// Register init system
		c.systems[joinIntent.Name] = InitSystem{
			ServerInfo: mdns.ServerInfo{
				Version:  joinIntent.Version,
				Name:     joinIntent.Name,
				Address:  joinIntent.Address,
				Services: joinIntent.Services,
				// Store the peers certificate to allow mTLS server validation
				// for requests after the trust establishment.
				Certificate: remoteCert,
			},
		}
	}

	if !c.autoSetup {
		for _, info := range c.systems {
			fmt.Printf(" Selected %q at %q\n", info.ServerInfo.Name, info.ServerInfo.Address)
		}

		// Add a space between the CLI and the response.
		fmt.Println("")
	}

	return nil
}

func (c *initConfig) joiningSession(gw *cloudClient.WebsocketGateway, sh *service.Handler, services []types.ServiceType, initiatorAddress string, passphrase string) error {
	session := types.Session{
		Passphrase:       passphrase,
		Address:          sh.Address,
		InitiatorAddress: initiatorAddress,
		Interface:        c.lookupIface.Name,
		Services:         services,
		LookupTimeout:    c.lookupTimeout,
	}

	err := gw.Write(session)
	if err != nil {
		return fmt.Errorf("Failed to send session start: %w", err)
	}

	if !c.autoSetup && initiatorAddress == "" {
		fmt.Println("Searching for an eligible system ...")
	}

	// The server confirms the target regardless whether or not one was provided.
	err = gw.ReceiveWithContext(gw.Context(), &session)
	if err != nil {
		return fmt.Errorf("Failed to find an eligible system: %w", err)
	}

	if !c.autoSetup {
		fingerprint, err := c.shortFingerprint(session.InitiatorFingerprint)
		if err != nil {
			return err
		}

		fmt.Printf("\n Found system %q at %q using fingerprint %q\n\n", session.InitiatorName, session.InitiatorAddress, fingerprint)
		fmt.Printf("Select %q on %q to let it join the cluster\n", sh.Name, session.InitiatorName)
	}

	return c.askJoinConfirmation(gw, services)
}
