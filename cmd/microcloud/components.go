package main

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/canonical/lxd/client"
	lxdAPI "github.com/canonical/lxd/shared/api"
	cli "github.com/canonical/lxd/shared/cmd"
	"github.com/canonical/microcluster/v2/client"
	"github.com/canonical/microcluster/v2/microcluster"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/cmd/tui"
	"github.com/canonical/microcloud/microcloud/component"
	"github.com/canonical/microcloud/microcloud/multicast"
)

type cmdComponents struct {
	common *CmdControl
}

// Command returns the subcommand to manage MicroCloud components.
func (c *cmdComponents) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "component",
		Short: "Manage MicroCloud components",
		RunE:  func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}

	var cmdComponentList = cmdComponentList{common: c.common}
	cmd.AddCommand(cmdComponentList.Command())

	var cmdComponentAdd = cmdComponentAdd{common: c.common}
	cmd.AddCommand(cmdComponentAdd.Command())

	return cmd
}

type cmdComponentList struct {
	common *CmdControl
}

// Command returns the subcommand to list MicroCloud components.
func (c *cmdComponentList) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List MicroCloud components and their cluster members",
		RunE:  c.Run,
	}

	return cmd
}

// Run runs the subcommand to list MicroCloud components.
func (c *cmdComponentList) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
	}

	// Get a microcluster client so we can get state information.
	cloudApp, err := microcluster.App(microcluster.Args{StateDir: c.common.FlagMicroCloudDir})
	if err != nil {
		return err
	}

	err = cloudApp.Ready(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to wait for MicroCloud to get ready: %w", err)
	}

	// Fetch the name and address, and ensure we're initialized.
	status, err := cloudApp.Status(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to get MicroCloud status: %w", err)
	}

	if !status.Ready {
		return fmt.Errorf("MicroCloud is uninitialized, run 'microcloud init' first")
	}

	components := []types.ComponentType{types.MicroCloud, types.LXD}
	optionalComponents := map[types.ComponentType]string{
		types.MicroCeph: api.MicroCephDir,
		types.MicroOVN:  api.MicroOVNDir,
	}

	cfg := initConfig{
		autoSetup: true,
		bootstrap: false,
		common:    c.common,
		asker:     c.common.asker,
		systems:   map[string]InitSystem{},
		state:     map[string]component.SystemInformation{},
	}

	cfg.name = status.Name
	cfg.address = status.Address.Addr().String()

	components, err = cfg.askMissingComponents(components, optionalComponents)
	if err != nil {
		return err
	}

	// Instantiate a handler for the components.
	s, err := component.NewHandler(status.Name, status.Address.Addr().String(), c.common.FlagMicroCloudDir, components...)
	if err != nil {
		return err
	}

	mu := sync.Mutex{}
	header := []string{"NAME", "ADDRESS", "ROLE", "STATUS"}
	allClusters := map[types.ComponentType][][]string{}
	err = s.RunConcurrent("", "", func(s component.Component) error {
		var err error
		var data [][]string
		var microClient *client.Client
		var lxd lxd.InstanceServer
		switch s.Type() {
		case types.LXD:
			lxd, err = s.(*component.LXDComponent).Client(context.Background())
		case types.MicroCeph:
			microClient, err = s.(*component.CephComponent).Client("")
		case types.MicroOVN:
			microClient, err = s.(*component.OVNComponent).Client()
		case types.MicroCloud:
			microClient, err = s.(*component.CloudComponent).Client()
		}

		if err != nil {
			return err
		}

		if microClient != nil {
			clusterMembers, err := microClient.GetClusterMembers(context.Background())
			if err != nil && !lxdAPI.StatusErrorCheck(err, http.StatusServiceUnavailable) {
				return err
			}

			if len(clusterMembers) != 0 {
				data = make([][]string, len(clusterMembers))
				for i, clusterMember := range clusterMembers {
					data[i] = []string{clusterMember.Name, clusterMember.Address.String(), clusterMember.Role, string(clusterMember.Status)}
				}

				sort.Sort(cli.SortColumnsNaturally(data))
			}
		} else if lxd != nil {
			server, _, err := lxd.GetServer()
			if err != nil {
				return err
			}

			if server.Environment.ServerClustered {
				clusterMembers, err := lxd.GetClusterMembers()
				if err != nil {
					return err
				}

				data = make([][]string, len(clusterMembers))
				for i, clusterMember := range clusterMembers {
					data[i] = []string{clusterMember.ServerName, clusterMember.URL, strings.Join(clusterMember.Roles, "\n"), string(clusterMember.Status)}
				}

				sort.Sort(cli.SortColumnsNaturally(data))
			}
		}

		mu.Lock()
		allClusters[s.Type()] = data
		mu.Unlock()

		return nil
	})
	if err != nil {
		return err
	}

	for componentType, data := range allClusters {
		if len(data) == 0 {
			fmt.Printf("%s: Not initialized\n", componentType)
		} else {
			fmt.Printf("%s:\n", componentType)
			fmt.Println(tui.NewTable(header, data))
		}
	}

	return nil
}

type cmdComponentAdd struct {
	common *CmdControl
}

// Command returns the subcommand to add components to MicroCloud.
func (c *cmdComponentAdd) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add new components to the existing MicroCloud",
		RunE:  c.Run,
	}

	return cmd
}

// Run runs the subcommand to add components to MicroCloud.
func (c *cmdComponentAdd) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
	}

	fmt.Println("Waiting for components to start ...")
	err := checkInitialized(c.common.FlagMicroCloudDir, true, false)
	if err != nil {
		return err
	}

	cfg := initConfig{
		// Set bootstrap to true because we are setting up a new cluster for new components.
		bootstrap: true,
		setupMany: true,
		common:    c.common,
		asker:     c.common.asker,
		systems:   map[string]InitSystem{},
		state:     map[string]component.SystemInformation{},
	}

	// Get a microcluster client so we can get state information.
	cloudApp, err := microcluster.App(microcluster.Args{StateDir: c.common.FlagMicroCloudDir})
	if err != nil {
		return err
	}

	// Fetch the name and address, and ensure we're initialized.
	status, err := cloudApp.Status(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to get MicroCloud status: %w", err)
	}

	cfg.name = status.Name
	cfg.address = status.Address.Addr().String()
	// enable auto setup to skip lookup related questions.
	cfg.autoSetup = true
	err = cfg.askAddress("")
	if err != nil {
		return err
	}

	cfg.autoSetup = false
	installedComponents := []types.ComponentType{types.MicroCloud, types.LXD}
	optionalComponents := map[types.ComponentType]string{
		types.MicroCeph: api.MicroCephDir,
		types.MicroOVN:  api.MicroOVNDir,
	}

	// Set the auto flag to true so that we automatically omit any components that aren't installed.
	installedComponents, err = cfg.askMissingComponents(installedComponents, optionalComponents)
	if err != nil {
		return err
	}

	// Instantiate a handler for the components.
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

	state, err := s.CollectSystemInformation(context.Background(), multicast.ServerInfo{Name: cfg.name, Address: cfg.address, Components: components})
	if err != nil {
		return err
	}

	cfg.state[cfg.name] = *state
	// Create an InitSystem map to carry through the interactive setup.
	clusters := cfg.state[cfg.name].ExistingComponents
	for name, address := range clusters[types.MicroCloud] {
		cfg.systems[name] = InitSystem{
			ServerInfo: multicast.ServerInfo{
				Name:       name,
				Address:    address,
				Components: components,
			},
		}
	}

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

	askClusteredComponents := map[types.ComponentType]string{}
	componentMap := map[types.ComponentType]bool{}
	for _, state := range cfg.state {
		localState := cfg.state[s.Name]
		if len(state.ExistingComponents[types.LXD]) != len(localState.ExistingComponents[types.LXD]) || len(state.ExistingComponents[types.LXD]) <= 0 {
			return fmt.Errorf("Unable to add components. Some systems are not part of the LXD cluster")
		}

		if len(state.ExistingComponents[types.MicroCeph]) <= 0 && !componentMap[types.MicroCeph] {
			askClusteredComponents[types.MicroCeph] = components[types.MicroCeph]
			componentMap[types.MicroCeph] = true
		}

		if len(state.ExistingComponents[types.MicroOVN]) <= 0 && !componentMap[types.MicroOVN] {
			askClusteredComponents[types.MicroOVN] = components[types.MicroOVN]
			componentMap[types.MicroOVN] = true
		}
	}

	if len(askClusteredComponents) == 0 {
		return fmt.Errorf("All components have already been set up")
	}

	err = cfg.askClustered(s, askClusteredComponents)
	if err != nil {
		return err
	}

	// Go through the normal setup for disks and networks if necessary.
	if askClusteredComponents[types.MicroCeph] != "" {
		err := cfg.askDisks(s)
		if err != nil {
			return err
		}
	}

	if askClusteredComponents[types.MicroOVN] != "" {
		err := cfg.askNetwork(s)
		if err != nil {
			return err
		}
	}

	return cfg.setupCluster(s)
}
