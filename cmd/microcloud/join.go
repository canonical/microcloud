package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/api/types"
	cloudClient "github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcloud/microcloud/service"
)

type cmdJoin struct {
	common *CmdControl

	flagLookupTimeout    int64
	flagSessionTimeout   int64
	flagInitiatorAddress string
}

// command returns the subcommand for joining a MicroCloud.
func (c *cmdJoin) command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "join",
		Short: "Join an existing MicroCloud cluster",
		RunE:  c.Run,
	}

	cmd.Flags().Int64Var(&c.flagLookupTimeout, "lookup-timeout", 0, "Amount of seconds to wait when finding systems on the network. Defaults: 60s")
	cmd.Flags().Int64Var(&c.flagSessionTimeout, "session-timeout", 0, "Amount of seconds to wait for the trust establishment session. Defaults: 10m")
	cmd.Flags().StringVar(&c.flagInitiatorAddress, "initiator-address", "", "Address of the trust establishment session's initiator")

	return cmd
}

// Run runs the subcommand for joining a MicroCloud.
func (c *cmdJoin) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
	}

	fmt.Println("Waiting for services to start ...")
	err := checkInitialized(c.common.FlagMicroCloudDir, false, false)
	if err != nil {
		return err
	}

	cfg := initConfig{
		bootstrap: false,
		common:    c.common,
		asker:     c.common.asker,
		systems:   map[string]InitSystem{},
		state:     map[string]service.SystemInformation{},
	}

	cfg.lookupTimeout = DefaultLookupTimeout
	if c.flagLookupTimeout > 0 {
		cfg.lookupTimeout = time.Duration(c.flagLookupTimeout) * time.Second
	}

	cfg.sessionTimeout = DefaultSessionTimeout
	if c.flagSessionTimeout > 0 {
		cfg.sessionTimeout = time.Duration(c.flagSessionTimeout) * time.Second
	}

	// Gather hostname before calling askAddress, as it will be used to locate the bootstrap system.
	cfg.name, err = os.Hostname()
	if err != nil {
		return fmt.Errorf("Failed to retrieve system hostname: %w", err)
	}

	err = cfg.askAddress(c.flagInitiatorAddress)
	if err != nil {
		return err
	}

	installedServices := []types.ServiceType{types.MicroCloud, types.LXD}
	optionalServices := map[types.ServiceType]string{
		types.MicroCeph: api.MicroCephDir,
		types.MicroOVN:  api.MicroOVNDir,
	}

	// Enable auto setup to skip service related questions.
	cfg.autoSetup = true
	installedServices, err = cfg.askMissingServices(installedServices, optionalServices)
	if err != nil {
		return err
	}

	cfg.autoSetup = false

	s, err := service.NewHandler(cfg.name, cfg.address, c.common.FlagMicroCloudDir, installedServices...)
	if err != nil {
		return err
	}

	services := make(map[types.ServiceType]string, len(installedServices))
	for _, s := range s.Services {
		version, err := s.GetVersion(context.Background())
		if err != nil {
			return err
		}

		services[s.Type()] = version
	}

	passphrase, err := cfg.askPassphrase()
	if err != nil {
		return err
	}

	return cfg.runSession(context.Background(), s, types.SessionJoining, cfg.sessionTimeout, func(gw *cloudClient.WebsocketGateway) error {
		return cfg.joiningSession(gw, s, services, c.flagInitiatorAddress, passphrase)
	})
}
