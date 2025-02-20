package main

import (
	"context"
	"fmt"
	"time"

	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/lxd/shared/revert"
	"github.com/canonical/microcluster/v2/microcluster"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/api/types"
	cloudClient "github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcloud/microcloud/component"
	"github.com/canonical/microcloud/microcloud/multicast"
)

type cmdAdd struct {
	common *CmdControl

	flagSessionTimeout int64
}

// Command returns the subcommand to add new systems to MicroCloud.
func (c *cmdAdd) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add new systems to an existing MicroCloud cluster",
		RunE:  c.Run,
	}

	cmd.Flags().Int64Var(&c.flagSessionTimeout, "session-timeout", 0, "Amount of seconds to wait for the trust establishment session. Defaults: 60m")

	return cmd
}

// Run runs the subcommand to add new systems to MicroCloud.
func (c *cmdAdd) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
	}

	fmt.Println("Waiting for components to start ...")
	err := checkInitialized(c.common.FlagMicroCloudDir, true, false)
	if err != nil {
		return err
	}

	cfg := initConfig{
		bootstrap: false,
		setupMany: true,
		common:    c.common,
		asker:     c.common.asker,
		systems:   map[string]InitSystem{},
		state:     map[string]component.SystemInformation{},
	}

	cfg.sessionTimeout = DefaultSessionTimeout
	if c.flagSessionTimeout > 0 {
		cfg.sessionTimeout = time.Duration(c.flagSessionTimeout) * time.Second
	}

	cloudApp, err := microcluster.App(microcluster.Args{StateDir: c.common.FlagMicroCloudDir})
	if err != nil {
		return err
	}

	status, err := cloudApp.Status(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to get MicroCloud status: %w", err)
	}

	cfg.name = status.Name
	cfg.address = status.Address.Addr().String()
	err = cfg.askAddress("")
	if err != nil {
		return err
	}

	installedComponents := []types.ComponentType{types.MicroCloud, types.LXD}
	optionalComponents := map[types.ComponentType]string{
		types.MicroCeph: api.MicroCephDir,
		types.MicroOVN:  api.MicroOVNDir,
	}

	installedComponents, err = cfg.askMissingComponents(installedComponents, optionalComponents)
	if err != nil {
		return err
	}

	s, err := component.NewHandler(cfg.name, cfg.address, c.common.FlagMicroCloudDir, installedComponents...)
	if err != nil {
		return err
	}

	components := make(map[types.ComponentType]string, len(installedComponents))
	for _, s := range s.Components {
		version, err := s.GetVersion(context.Background())
		if err != nil {
			return err
		}

		components[s.Type()] = version
	}

	err = cfg.runSession(context.Background(), s, types.SessionInitiating, cfg.sessionTimeout, func(gw *cloudClient.WebsocketGateway) error {
		return cfg.initiatingSession(gw, s, components, "", nil)
	})
	if err != nil {
		return err
	}

	reverter := revert.New()
	defer reverter.Fail()

	reverter.Add(func() {
		// Stop each joiner member session.
		cloud := s.Components[types.MicroCloud].(*component.CloudComponent)
		for peer, system := range cfg.systems {
			if system.ServerInfo.Name == "" || system.ServerInfo.Name == cfg.name {
				continue
			}

			if system.ServerInfo.Address == "" {
				logger.Error("No joiner address provided to stop the session")
				continue
			}

			remoteClient, err := cloud.RemoteClient(system.ServerInfo.Certificate, util.CanonicalNetworkAddress(system.ServerInfo.Address, component.CloudPort))
			if err != nil {
				logger.Error("Failed to create remote client", logger.Ctx{"address": system.ServerInfo.Address, "error": err})
				continue
			}

			err = cloudClient.StopSession(context.Background(), remoteClient, "Initiator aborted the setup")
			if err != nil {
				logger.Error("Failed to stop joiner session", logger.Ctx{"joiner": peer, "error": err})
			}
		}
	})

	state, err := s.CollectSystemInformation(context.Background(), multicast.ServerInfo{Name: cfg.name, Address: cfg.address, Components: components})
	if err != nil {
		return err
	}

	cfg.state[cfg.name] = *state
	fmt.Println("Gathering system information...")
	for _, system := range cfg.systems {
		if system.ServerInfo.Name == "" || system.ServerInfo.Name == cfg.name {
			continue
		}

		state, err := s.CollectSystemInformation(context.Background(), system.ServerInfo)
		if err != nil {
			return err
		}

		cfg.state[system.ServerInfo.Name] = *state
	}

	// Ensure LXD is not already clustered if we are running `microcloud init`.
	for name, info := range cfg.state {
		_, newSystem := cfg.systems[name]
		if newSystem && info.ComponentClustered(types.LXD) {
			return fmt.Errorf("%s is already clustered on %q, aborting setup", types.LXD, info.ClusterName)
		}
	}

	// Ensure there are no existing cluster conflicts.
	conflictableComponents := map[types.ComponentType]string{}
	for component, version := range components {
		if component == types.LXD {
			continue
		}

		conflictableComponents[component] = version
	}

	conflict, componentType := component.ClustersConflict(cfg.state, conflictableComponents)
	if conflict {
		return fmt.Errorf("Some systems are already part of different %s clusters. Aborting initialization", componentType)
	}

	// Ask to reuse existing clusters.
	err = cfg.askClustered(s, components)
	if err != nil {
		return err
	}

	// Also populate system information for existing cluster members. This is so we can potentially set up storage and networks if they haven't been set up before.
	for name, address := range state.ExistingComponents[types.MicroCloud] {
		_, ok := cfg.systems[name]
		if ok {
			continue
		}

		cfg.systems[name] = InitSystem{
			ServerInfo: multicast.ServerInfo{
				Name:       name,
				Address:    address,
				Components: components,
			},
		}

		if name == cfg.name {
			continue
		}

		state, err := s.CollectSystemInformation(context.Background(), multicast.ServerInfo{Name: name, Address: address})
		if err != nil {
			return err
		}

		cfg.state[name] = *state
	}

	err = cfg.askDisks(s)
	if err != nil {
		return err
	}

	err = cfg.askNetwork(s)
	if err != nil {
		return err
	}

	err = cfg.setupCluster(s)
	if err != nil {
		return err
	}

	reverter.Success()
	return nil
}
