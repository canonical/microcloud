package main

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"

	"github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared"
	cli "github.com/canonical/lxd/shared/cmd"
	"github.com/canonical/microcluster/client"
	"github.com/canonical/microcluster/microcluster"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/mdns"
	"github.com/canonical/microcloud/microcloud/service"
)

type cmdServices struct {
	common *CmdControl
}

func (c *cmdServices) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage MicroCloud services",
		RunE:  func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}

	var cmdServiceList = cmdServiceList{common: c.common}
	cmd.AddCommand(cmdServiceList.Command())

	var cmdServiceAdd = cmdServiceAdd{common: c.common}
	cmd.AddCommand(cmdServiceAdd.Command())

	return cmd
}

type cmdServiceList struct {
	common *CmdControl
}

func (c *cmdServiceList) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List MicroCloud services and their cluster members",
		RunE:  c.Run,
	}

	return cmd
}

func (c *cmdServiceList) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
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

	if !status.Ready {
		return fmt.Errorf("MicroCloud is uninitialized, run 'microcloud init' first")
	}

	services := []types.ServiceType{types.MicroCloud, types.LXD}
	optionalServices := map[types.ServiceType]string{
		types.MicroCeph: api.MicroCephDir,
		types.MicroOVN:  api.MicroOVNDir,
	}

	services, err = c.common.askMissingServices(services, optionalServices, true)
	if err != nil {
		return err
	}

	// Instantiate a handler for the services.
	s, err := service.NewHandler(status.Name, status.Address.Addr().String(), c.common.FlagMicroCloudDir, c.common.FlagLogDebug, c.common.FlagLogVerbose, services...)
	if err != nil {
		return err
	}

	mu := sync.Mutex{}
	header := []string{"NAME", "ADDRESS", "ROLE", "STATUS"}
	allClusters := map[types.ServiceType][][]string{}
	err = s.RunConcurrent(false, false, func(s service.Service) error {
		var err error
		var data [][]string
		var microClient *client.Client
		var lxd lxd.InstanceServer
		switch s.Type() {
		case types.LXD:
			lxd, err = s.(*service.LXDService).Client(context.Background(), "")
		case types.MicroCeph:
			microClient, err = s.(*service.CephService).Client("", "")
		case types.MicroOVN:
			microClient, err = s.(*service.OVNService).Client()
		case types.MicroCloud:
			microClient, err = s.(*service.CloudService).Client()
		}

		if err != nil {
			return err
		}

		if microClient != nil {
			clusterMembers, err := microClient.GetClusterMembers(context.Background())
			if err != nil && err.Error() != "Daemon not yet initialized" {
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

		mu.Lock()
		allClusters[s.Type()] = data
		mu.Unlock()

		return nil
	})
	if err != nil {
		return err
	}

	for serviceType, data := range allClusters {
		if len(data) == 0 {
			fmt.Printf("%s: Not initialized\n", serviceType)
		} else {
			fmt.Printf("%s:\n", serviceType)
			err = cli.RenderTable(cli.TableFormatTable, header, data, nil)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

type cmdServiceAdd struct {
	common *CmdControl
}

func (c *cmdServiceAdd) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Set up new services on the existing MicroCloud",
		RunE:  c.Run,
	}

	return cmd
}

func (c *cmdServiceAdd) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
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

	if !status.Ready {
		return fmt.Errorf("MicroCloud is uninitialized, run 'microcloud init' first")
	}

	services := []types.ServiceType{types.MicroCloud, types.LXD}
	optionalServices := map[types.ServiceType]string{
		types.MicroCeph: api.MicroCephDir,
		types.MicroOVN:  api.MicroOVNDir,
	}

	// Set the auto flag to true so that we automatically omit any services that aren't installed.
	services, err = c.common.askMissingServices(services, optionalServices, true)
	if err != nil {
		return err
	}

	// Instantiate a handler for the services.
	s, err := service.NewHandler(status.Name, status.Address.Addr().String(), c.common.FlagMicroCloudDir, c.common.FlagLogDebug, c.common.FlagLogVerbose, services...)
	if err != nil {
		return err
	}

	// Fetch the cluster members for services we want to ignore.
	cloudCluster, err := s.Services[types.MicroCloud].ClusterMembers(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to inspect existing cluster: %w", err)
	}

	lxdCluster, err := s.Services[types.LXD].ClusterMembers(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to inspect existing cluster: %w", err)
	}

	// Create an InitSystem map to carry through the interactive setup.
	systems := make(map[string]InitSystem, len(cloudCluster))
	for name, address := range cloudCluster {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return fmt.Errorf("Failed to parse cluster member address %q: %w", address, err)
		}

		systems[name] = InitSystem{
			ServerInfo: mdns.ServerInfo{
				Name:     name,
				Address:  host,
				Services: services,
			},
			InitializedServices: map[types.ServiceType]map[string]string{
				types.LXD:        lxdCluster,
				types.MicroCloud: cloudCluster,
			},
		}
	}

	// Check if there are any pre-existing clusters that we can re-use for each optional service.
	availableServices := map[types.ServiceType]string{}
	for _, service := range services {
		if service == types.LXD || service == types.MicroCloud {
			continue
		}

		// Get the first system that has initialized an optional service, and its list of cluster members. We may or may not already be in this cluster.
		firstSystem, clusterMembers, err := checkClustered(s, false, service, systems)
		if err != nil {
			return err
		}

		// If no system is clustered yet, record that too so we can try to set it up.
		if firstSystem == "" {
			availableServices[service] = ""
			continue
		}

		// If any service has all of the cluster members recorded on the MicroCloud daemon already,
		// then it can be considered part of the microcloud already, so we can ignore it.
		allMembersExist := true
		for name := range cloudCluster {
			_, ok := clusterMembers[name]
			if !ok {
				allMembersExist = false
				break
			}
		}

		if !allMembersExist {
			availableServices[service] = firstSystem
		}
	}

	// Ask to reuse or skip existing clusters.
	for serviceType, system := range availableServices {
		question := fmt.Sprintf("%q is already part of a %s cluster. Use this cluster with MicroCloud, or skip %s? (reuse/skip) [default=reuse]", system, serviceType, serviceType)
		validator := func(s string) error {
			if !shared.ValueInSlice(s, []string{"reuse", "skip"}) {
				return fmt.Errorf("Invalid input, expected one of (reuse,skip) but got %q", s)
			}

			return nil
		}

		if system == "" {
			continue
		}

		reuseOrSkip, err := c.common.asker.AskString(question, "reuse", validator)
		if err != nil {
			return err
		}

		if reuseOrSkip != "reuse" {
			delete(s.Services, serviceType)
			delete(availableServices, serviceType)
		}
	}

	// Go through the normal setup for disks and networks if necessary.
	_, ok := availableServices[types.MicroCeph]
	if ok {
		err = c.common.askDisks(s, systems, false, false)
		if err != nil {
			return err
		}
	}

	_, _, subnet, err := c.common.askAddress(true, status.Address.Addr().String())
	if err != nil {
		return err
	}

	_, ok = availableServices[types.MicroOVN]
	if ok {
		err = c.common.askNetwork(s, systems, subnet, false)
		if err != nil {
			return err
		}
	}

	return setupCluster(s, systems)
}
