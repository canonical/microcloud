package main

import (
	"context"
	"fmt"
	"time"

	"github.com/canonical/microcluster/v2/microcluster"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/mdns"
	"github.com/canonical/microcloud/microcloud/service"
)

type cmdAdd struct {
	common *CmdControl

	flagAutoSetup     bool
	flagWipe          bool
	flagPreseed       bool
	flagLookupTimeout int64
}

func (c *cmdAdd) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Scan for new cluster members to add",
		RunE:  c.Run,
	}

	cmd.Flags().BoolVar(&c.flagAutoSetup, "auto", false, "Automatic setup with default configuration")
	cmd.Flags().BoolVar(&c.flagWipe, "wipe", false, "Wipe disks to add to MicroCeph")
	cmd.Flags().BoolVar(&c.flagPreseed, "preseed", false, "Expect Preseed YAML for configuring MicroCloud in stdin")
	cmd.Flags().Int64Var(&c.flagLookupTimeout, "lookup-timeout", 0, "Amount of seconds to wait for systems to show up. Defaults: 60s for interactive, 5s for automatic and preseed")

	return cmd
}

func (c *cmdAdd) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
	}

	cfg := initConfig{
		bootstrap:    false,
		setupMany:    true,
		autoSetup:    c.flagAutoSetup,
		wipeAllDisks: c.flagWipe,
		common:       c.common,
		asker:        &c.common.asker,
		systems:      map[string]InitSystem{},
		state:        map[string]service.SystemInformation{},
	}

	cfg.lookupTimeout = DefaultLookupTimeout
	if c.flagLookupTimeout > 0 {
		cfg.lookupTimeout = time.Duration(c.flagLookupTimeout) * time.Second
	} else if c.flagAutoSetup || c.flagPreseed {
		cfg.lookupTimeout = DefaultAutoLookupTimeout
	}

	if c.flagPreseed {
		return cfg.RunPreseed(cmd)
	}

	cloudApp, err := microcluster.App(microcluster.Args{StateDir: c.common.FlagMicroCloudDir})
	if err != nil {
		return err
	}

	status, err := cloudApp.Status(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to get MicroCloud status: %w", err)
	}

	if !status.Ready {
		return fmt.Errorf("MicroCloud is uninitialized, run 'microcloud init' first")
	}

	cfg.name = status.Name
	cfg.address = status.Address.Addr().String()
	err = cfg.askAddress()
	if err != nil {
		return err
	}

	services := []types.ServiceType{types.MicroCloud, types.LXD}
	optionalServices := map[types.ServiceType]string{
		types.MicroCeph: api.MicroCephDir,
		types.MicroOVN:  api.MicroOVNDir,
	}

	services, err = cfg.askMissingServices(services, optionalServices)
	if err != nil {
		return err
	}

	s, err := service.NewHandler(cfg.name, cfg.address, c.common.FlagMicroCloudDir, services...)
	if err != nil {
		return err
	}

	err = cfg.lookupPeers(s, nil)
	if err != nil {
		return err
	}

	state, err := s.CollectSystemInformation(context.Background(), mdns.ServerInfo{Name: cfg.name, Address: cfg.address, Services: services})
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
		if newSystem && info.ServiceClustered(types.LXD) {
			return fmt.Errorf("%s is already clustered on %q, aborting setup", types.LXD, info.ClusterName)
		}
	}

	// Ensure there are no existing cluster conflicts.
	conflict, serviceType := service.ClustersConflict(cfg.state, []types.ServiceType{types.MicroOVN, types.MicroCloud})
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
			ServerInfo: mdns.ServerInfo{
				Name:     name,
				Address:  address,
				Services: services,
			},
		}

		if name == cfg.name {
			continue
		}

		state, err := s.CollectSystemInformation(context.Background(), mdns.ServerInfo{Name: name, Address: address})
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

	return cfg.setupCluster(s)
}
