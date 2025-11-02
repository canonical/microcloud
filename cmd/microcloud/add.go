package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/lxd/shared/revert"
	"github.com/canonical/microcluster/v3/microcluster"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/api/types"
	cloudClient "github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcloud/microcloud/multicast"
	"github.com/canonical/microcloud/microcloud/service"
)

type cmdAdd struct {
	common *CmdControl

	flagSessionTimeout int64
}

// command returns the subcommand to add new systems to MicroCloud.
func (c *cmdAdd) command() *cobra.Command {
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

	fmt.Println("Waiting for services to start ...")
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
		state:     map[string]service.SystemInformation{},
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

	installedServices := []types.ServiceType{types.MicroCloud, types.LXD}
	optionalServices := map[types.ServiceType]string{
		types.MicroCeph: api.MicroCephDir,
		types.MicroOVN:  api.MicroOVNDir,
	}

	installedServices, err = cfg.askMissingServices(installedServices, optionalServices)
	if err != nil {
		return err
	}

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

	err = cfg.runSession(context.Background(), s, types.SessionInitiating, cfg.sessionTimeout, func(gw *cloudClient.WebsocketGateway) error {
		return cfg.initiatingSession(gw, s, services, "", nil)
	})
	if err != nil {
		return err
	}

	// Exit early if no new systems got selected during the trust establishment session.
	if len(cfg.systems) == 0 {
		return errors.New("At least one new system has to be selected")
	}

	reverter := revert.New()
	defer reverter.Fail()

	reverter.Add(func() {
		// Stop each joiner member session.
		cloud := s.Services[types.MicroCloud].(*service.CloudService)
		for peer, system := range cfg.systems {
			if system.ServerInfo.Name == "" || system.ServerInfo.Name == cfg.name {
				continue
			}

			if system.ServerInfo.Address == "" {
				logger.Error("No joiner address provided to stop the session")
				continue
			}

			remoteClient, err := cloud.RemoteClient(system.ServerInfo.Certificate, util.CanonicalNetworkAddress(system.ServerInfo.Address, service.CloudPort))
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

	state, err := s.CollectSystemInformation(context.Background(), multicast.ServerInfo{Name: cfg.name, Address: cfg.address, Services: services})
	if err != nil {
		return err
	}

	cfg.state[cfg.name] = *state

	fmt.Println("Gathering system information...")
	for peer, system := range cfg.systems {
		if system.ServerInfo.Name == "" || system.ServerInfo.Name == cfg.name {
			continue
		}

		state, err := s.CollectSystemInformation(context.Background(), system.ServerInfo)
		if err != nil {
			return err
		}

		cfg.state[system.ServerInfo.Name] = *state

		err = populateMicroCloudNetworkFromState(state, peer, &system, cfg.lookupSubnet)
		if err != nil {
			return err
		}
	}

	// Ensure LXD is not already clustered if we are running `microcloud init`.
	for name, info := range cfg.state {
		_, newSystem := cfg.systems[name]
		if newSystem && info.ServiceClustered(types.LXD) {
			return fmt.Errorf("%s is already clustered on %q, aborting setup", types.LXD, info.ClusterName)
		}
	}

	// Ensure there are no existing cluster conflicts.
	conflictableServices := map[types.ServiceType]string{}
	for service, version := range services {
		if service == types.LXD {
			continue
		}

		conflictableServices[service] = version
	}

	conflict, serviceType := service.ClustersConflict(cfg.state, conflictableServices)
	if conflict {
		return fmt.Errorf("Some systems are already part of different %s clusters. Aborting initialization", serviceType)
	}

	// Ask to reuse existing clusters.
	err = cfg.askClustered(s, services)
	if err != nil {
		return err
	}

	// Also populate system information for existing cluster members. This is so we can potentially set up storage and networks if they haven't been set up before.
	for name, address := range state.ExistingServices[types.MicroCloud] {
		_, ok := cfg.systems[name]
		if ok {
			continue
		}

		cfg.systems[name] = InitSystem{
			ServerInfo: multicast.ServerInfo{
				Name:     name,
				Address:  address,
				Services: services,
			},
		}

		if name == cfg.name {
			continue
		}

		state, err := s.CollectSystemInformation(context.Background(), multicast.ServerInfo{Name: name, Address: address})
		if err != nil {
			return err
		}

		// Populate MicroCloud Internal network also for existing systems.
		system := cfg.systems[name]
		err = populateMicroCloudNetworkFromState(state, name, &system, cfg.lookupSubnet)
		if err != nil {
			return err
		}

		cfg.state[name] = *state
	}

	microCloudNetworkFromStateSystem := cfg.systems[cfg.name]
	microCloudNetworkFromStateSystem.MicroCloudInternalNetwork = &NetworkInterfaceInfo{Interface: *cfg.lookupIface, Subnet: cfg.lookupSubnet, IP: net.IP(cfg.address)}
	cfg.systems[cfg.name] = microCloudNetworkFromStateSystem

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
