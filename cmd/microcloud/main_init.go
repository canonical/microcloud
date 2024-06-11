package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared"
	lxdAPI "github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/lxd/shared/validate"
	cephTypes "github.com/canonical/microceph/microceph/api/types"
	cephClient "github.com/canonical/microceph/microceph/client"
	"github.com/canonical/microcluster/client"
	ovnClient "github.com/canonical/microovn/microovn/client"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/mdns"
	"github.com/canonical/microcloud/microcloud/service"
)

// InitSystem represents the configuration passed to individual systems that join via the Handler.
type InitSystem struct {
	// ServerInfo contains the data reported by mDNS about this system.
	ServerInfo mdns.ServerInfo
	// A map of services and their cluster members, if initialized.
	InitializedServices map[types.ServiceType]map[string]string
	// AvailableDisks contains the disks as reported by LXD.
	AvailableDisks []lxdAPI.ResourcesStorageDisk
	// MicroCephDisks contains the disks intended to be passed to MicroCeph.
	MicroCephDisks []cephTypes.DisksPost
	// MicroCephClusterNetworkSubnet is an optional the subnet (IPv4/IPv6 CIDR notation) for the Ceph cluster network.
	MicroCephInternalNetworkSubnet string
	// TargetNetworks contains the network configuration for the target system.
	TargetNetworks []lxdAPI.NetworksPost
	// TargetStoragePools contains the storage pool configuration for the target system.
	TargetStoragePools []lxdAPI.StoragePoolsPost
	// Networks is the cluster-wide network configuration.
	Networks []lxdAPI.NetworksPost
	// StoragePools is the cluster-wide storage pool configuration.
	StoragePools []lxdAPI.StoragePoolsPost
	// StorageVolumes is the cluster-wide storage volume configuration.
	StorageVolumes map[string][]lxdAPI.StorageVolumesPost
	// JoinConfig is the LXD configuration for joining members.
	JoinConfig []lxdAPI.ClusterMemberConfigKey
}

type cmdInit struct {
	common *CmdControl

	flagAutoSetup    bool
	flagWipeAllDisks bool
	flagAddress      string
	flagPreseed      bool
}

func (c *cmdInit) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "init",
		Aliases: []string{"bootstrap"},
		Short:   "Initialize the network endpoint and create a new cluster",
		RunE:    c.Run,
	}

	cmd.Flags().BoolVar(&c.flagAutoSetup, "auto", false, "Automatic setup with default configuration")
	cmd.Flags().BoolVar(&c.flagWipeAllDisks, "wipe", false, "Wipe disks to add to MicroCeph")
	cmd.Flags().StringVar(&c.flagAddress, "address", "", "Address to use for MicroCloud")
	cmd.Flags().BoolVar(&c.flagPreseed, "preseed", false, "Expect Preseed YAML for configuring MicroCloud in stdin")

	return cmd
}

func (c *cmdInit) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
	}

	if c.flagPreseed {
		return c.common.RunPreseed(cmd, true)
	}

	return c.RunInteractive(cmd, args)
}

func (c *cmdInit) RunInteractive(cmd *cobra.Command, args []string) error {
	// Initially restart LXD so that the correct MicroCloud service related state is set by the LXD snap.
	fmt.Println("Waiting for LXD to start...")
	lxdService, err := service.NewLXDService("", "", c.common.FlagMicroCloudDir)
	if err != nil {
		return err
	}

	err = lxdService.Restart(context.Background(), 30)
	if err != nil {
		return err
	}

	systems := map[string]InitSystem{}

	addr, iface, subnet, err := c.common.askAddress(c.flagAutoSetup, c.flagAddress)
	if err != nil {
		return err
	}

	name, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Failed to retrieve system hostname: %w", err)
	}

	systems[name] = InitSystem{
		ServerInfo: mdns.ServerInfo{
			Name:    name,
			Address: addr,
		},
	}

	services := []types.ServiceType{types.MicroCloud, types.LXD}
	optionalServices := map[types.ServiceType]string{
		types.MicroCeph: api.MicroCephDir,
		types.MicroOVN:  api.MicroOVNDir,
	}

	services, err = c.common.askMissingServices(services, optionalServices, c.flagAutoSetup)
	if err != nil {
		return err
	}

	s, err := service.NewHandler(name, addr, c.common.FlagMicroCloudDir, c.common.FlagLogDebug, c.common.FlagLogVerbose, services...)
	if err != nil {
		return err
	}

	err = lookupPeers(s, c.flagAutoSetup, iface, subnet, nil, systems)
	if err != nil {
		return err
	}

	err = c.common.askClustered(s, c.flagAutoSetup, systems)
	if err != nil {
		return err
	}

	err = c.common.askDisks(s, systems, c.flagAutoSetup, c.flagWipeAllDisks, true)
	if err != nil {
		return err
	}

	err = c.common.askNetwork(s, systems, subnet, c.flagAutoSetup, true)
	if err != nil {
		return err
	}

	err = validateSystems(s, systems, true)
	if err != nil {
		return err
	}

	err = setupCluster(s, true, systems)
	if err != nil {
		return err
	}

	return nil
}

// lookupPeers attempts to find eligible systems over mDNS, optionally limiting lookup to the given subnet if not nil.
// Found systems will be progressively added to a table, and the user selection is added to the `systems` map.
//
// - If `autoSetup` is true, all systems found in the first 5s will be recorded, and no other input is required.
// - `expectedSystems` is a list of expected hostnames. If given, the behaviour is similar to `autoSetup`,
// except it will wait up to a minute for exclusively these systems to be recorded.
func lookupPeers(s *service.Handler, autoSetup bool, iface *net.Interface, subnet *net.IPNet, expectedSystems []string, systems map[string]InitSystem) error {
	header := []string{"NAME", "IFACE", "ADDR"}
	var table *SelectableTable
	var answers []string

	if len(expectedSystems) > 0 {
		autoSetup = true
	}

	tableCh := make(chan error)
	selectionCh := make(chan error)
	if !autoSetup {
		go func() {
			err := <-tableCh
			if err != nil {
				selectionCh <- err
				return
			}

			answers, err = table.GetSelections()
			selectionCh <- err
		}()
	}

	var timeAfter <-chan time.Time
	if autoSetup {
		timeAfter = time.After(5 * time.Second)
	}

	if len(expectedSystems) > 0 {
		timeAfter = time.After(1 * time.Minute)
	}

	expectedSystemsMap := make(map[string]bool, len(expectedSystems))
	for _, system := range expectedSystems {
		expectedSystemsMap[system] = true
	}

	fmt.Println("Scanning for eligible servers ...")
	totalPeers := map[string]mdns.ServerInfo{}
	done := false
	for !done {
		select {
		case <-timeAfter:
			done = true
		case err := <-selectionCh:
			if err != nil {
				return err
			}

			done = true
		default:
			// If we have found all expected systems, the map will be empty and we can return right away.
			if len(expectedSystemsMap) == 0 && len(expectedSystems) > 0 {
				done = true

				break
			}

			peers, err := mdns.LookupPeers(context.Background(), iface, mdns.Version, s.Name)
			if err != nil {
				return err
			}

			skipPeers := map[string]bool{}
			for key, info := range peers {
				_, ok := totalPeers[key]
				if !ok {
					serviceMap := make(map[types.ServiceType]bool, len(info.Services))
					for _, service := range info.Services {
						serviceMap[service] = true
					}

					// Skip any peers that are missing our services.
					for service := range s.Services {
						if !serviceMap[service] {
							skipPeers[info.Name] = true
							logger.Infof("Skipping peer %q due to missing services (%s)", info.Name, string(service))
							break
						}
					}

					// If given a subnet, skip any peers that are broadcasting from a different subnet.
					if subnet != nil && !subnet.Contains(net.ParseIP(info.Address)) {
						continue
					}

					if !skipPeers[info.Name] {
						totalPeers[key] = info

						if len(expectedSystems) > 0 {
							if expectedSystemsMap[info.Name] {
								delete(expectedSystemsMap, info.Name)
							} else {
								delete(totalPeers, key)
							}
						}

						if autoSetup {
							continue
						}

						if len(totalPeers) == 1 {
							table = NewSelectableTable(header, [][]string{{info.Name, info.Interface, info.Address}})
							err := table.Render(table.rows)
							if err != nil {
								return err
							}

							time.Sleep(100 * time.Millisecond)
							tableCh <- nil
						} else {
							table.Update([]string{info.Name, info.Interface, info.Address})
						}
					}
				}
			}
		}
	}

	if len(totalPeers) == 0 {
		return fmt.Errorf("Found no available systems")
	}

	for _, answer := range answers {
		peer := table.SelectionValue(answer, "NAME")
		addr := table.SelectionValue(answer, "ADDR")
		iface := table.SelectionValue(answer, "IFACE")
		for _, info := range totalPeers {
			if info.Name == peer && info.Address == addr && info.Interface == iface {
				systems[peer] = InitSystem{
					ServerInfo: info,
				}
			}
		}
	}

	if autoSetup {
		for _, info := range totalPeers {
			systems[info.Name] = InitSystem{
				ServerInfo: info,
			}
		}

		if len(expectedSystems) > 0 {
			return nil
		}

		// Add a space between the CLI and the response.
		fmt.Println("")
	}

	for _, info := range systems {
		fmt.Printf(" Selected %q at %q\n", info.ServerInfo.Name, info.ServerInfo.Address)
	}

	// Add a space between the CLI and the response.
	fmt.Println("")

	return nil
}

// waitForJoin requests a system to join each service's respective cluster,
// and then waits for the request to either complete or time out.
// If the request was successful, it additionally waits until the cluster appears in the database.
func waitForJoin(sh *service.Handler, clusterSizes map[types.ServiceType]int, secret string, peer string, cfg types.ServicesPut) error {
	cloud := sh.Services[types.MicroCloud].(*service.CloudService)
	err := cloud.RequestJoin(context.Background(), secret, peer, cfg)
	if err != nil {
		return fmt.Errorf("System %q failed to join the cluster: %w", peer, err)
	}

	clustered := make(map[types.ServiceType]bool, len(sh.Services))
	for _, tokenInfo := range cfg.Tokens {
		clustered[tokenInfo.Service] = false
	}

	// Iterate over all services until the database is updated with the new node across all of them.
	now := time.Now()
	for len(clustered) != 0 {
		if time.Since(now) >= time.Second*30 {
			return fmt.Errorf("Timed out waiting for cluster member %q to appear", peer)
		}

		// Check the size of the cluster for each service.
		for service := range clustered {
			systems, err := sh.Services[service].ClusterMembers(context.Background())
			if err != nil {
				return err
			}

			// If the size of the cluster has been incremented by 1 from its initial value,
			// then we don't need to check the corresponding service anymore.
			// So remove the service from consideration and update the current cluster size for the next node.
			if len(systems) == clusterSizes[service]+1 {
				delete(clustered, service)
				clusterSizes[service] = clusterSizes[service] + 1
			}
		}
	}

	fmt.Printf(" Peer %q has joined the cluster\n", peer)

	return nil
}

func AddPeers(sh *service.Handler, systems map[string]InitSystem, bootstrap bool) error {
	// Grab the systems that are clustered from the InitSystem map.
	initializedServices := map[types.ServiceType]string{}
	existingSystems := map[types.ServiceType]map[string]string{}
	for serviceType := range sh.Services {
		for peer, system := range systems {
			if system.InitializedServices != nil && system.InitializedServices[serviceType] != nil {
				initializedServices[serviceType] = peer
				existingSystems[serviceType] = system.InitializedServices[serviceType]
				break
			}
		}
	}

	// Prepare a JoinConfig to send to each joiner.
	joinConfig := make(map[string]types.ServicesPut, len(systems))
	for peer, info := range systems {
		joinConfig[peer] = types.ServicesPut{
			Tokens:     []types.ServiceToken{},
			Address:    info.ServerInfo.Address,
			LXDConfig:  info.JoinConfig,
			CephConfig: info.MicroCephDisks,
		}
	}

	clusterSize := map[types.ServiceType]int{}
	if bootstrap {
		for serviceType, clusterMembers := range existingSystems {
			clusterSize[serviceType] = len(clusterMembers)
		}
	}

	// Concurrently issue a token for each joiner.
	for peer := range systems {
		mut := sync.Mutex{}
		err := sh.RunConcurrent(false, false, func(s service.Service) error {
			// Only issue a token if the system isn't already part of that cluster.
			if existingSystems[s.Type()][peer] == "" {
				clusteredSystem := systems[initializedServices[s.Type()]]

				var token string
				var err error

				// If the local node is part of the pre-existing cluster, or if we are growing the cluster, issue the token locally.
				// Otherwise, use the MicroCloud proxy to ask an existing cluster member to issue the token.
				if clusteredSystem.ServerInfo.Name == sh.Name || clusteredSystem.ServerInfo.Name == "" {
					token, err = s.IssueToken(context.Background(), peer)
					if err != nil {
						return fmt.Errorf("Failed to issue %s token for peer %q: %w", s.Type(), peer, err)
					}
				} else {
					cloud := sh.Services[types.MicroCloud].(*service.CloudService)
					token, err = cloud.RemoteIssueToken(context.Background(), clusteredSystem.ServerInfo.Address, clusteredSystem.ServerInfo.AuthSecret, peer, s.Type())
					if err != nil {
						return err
					}
				}

				// Fetch the current cluster sizes if we are adding a new node.
				var currentCluster map[string]string
				if !bootstrap {
					currentCluster, err = s.ClusterMembers(context.Background())
					if err != nil {
						return fmt.Errorf("Failed to check for existing %s cluster size: %w", s.Type(), err)
					}
				}

				mut.Lock()

				if !bootstrap {
					clusterSize[s.Type()] = len(currentCluster)
				}

				cfg := joinConfig[peer]
				cfg.Tokens = append(cfg.Tokens, types.ServiceToken{Service: s.Type(), JoinToken: token})
				joinConfig[peer] = cfg
				mut.Unlock()
			}

			return nil
		})
		if err != nil {
			return err
		}
	}

	fmt.Println("Awaiting cluster formation ...")

	// If the local node needs to join an existing cluster, do it first so we can proceed as normal.
	if len(joinConfig[sh.Name].Tokens) > 0 {
		cfg := joinConfig[sh.Name]
		err := waitForJoin(sh, clusterSize, "", sh.Name, cfg)
		if err != nil {
			return err
		}
	}

	for peer, cfg := range joinConfig {
		if len(cfg.Tokens) == 0 || peer == sh.Name {
			continue
		}

		logger.Debug("Initiating sequential request for cluster join", logger.Ctx{"peer": peer})
		err := waitForJoin(sh, clusterSize, systems[peer].ServerInfo.AuthSecret, peer, cfg)
		if err != nil {
			return err
		}
	}

	return nil
}

// validateGatewayNet ensures that the ipv{4,6} gateway in a network's `config`
// is a valid CIDR address and that any ovn.ranges if present fall within the
// gateway. `ipPrefix` is one of "ipv4" or "ipv6".
func validateGatewayNet(config map[string]string, ipPrefix string, cidrValidator func(string) error) (ovnIPRanges []*shared.IPRange, err error) {
	gateway, hasGateway := config[ipPrefix+".gateway"]
	ovnRanges, hasOVNRanges := config[ipPrefix+".ovn.ranges"]

	var gatewayIP net.IP
	var gatewayNet *net.IPNet

	if hasGateway {
		err = cidrValidator(gateway)
		if err != nil {
			return nil, fmt.Errorf("Invalid %s.gateway %q: %w", ipPrefix, gateway, err)
		}

		gatewayIP, gatewayNet, err = net.ParseCIDR(gateway)
		if err != nil {
			return nil, fmt.Errorf("Invalid %s.gateway %q: %w", ipPrefix, gateway, err)
		}
	}

	if hasGateway && hasOVNRanges {
		ovnIPRanges, err = shared.ParseIPRanges(ovnRanges, gatewayNet)
		if err != nil {
			return nil, fmt.Errorf("Invalid %s.ovn.ranges %q: %w", ipPrefix, ovnRanges, err)
		}
	}

	for _, ipRange := range ovnIPRanges {
		if ipRange.ContainsIP(gatewayIP) {
			return nil, fmt.Errorf("%s %s.ovn.ranges must not include gateway address %q", service.DefaultUplinkNetwork, ipPrefix, gatewayNet.IP)
		}
	}

	return ovnIPRanges, nil
}

func validateSystems(s *service.Handler, systems map[string]InitSystem, bootstrap bool) (err error) {
	curSystem, _ := systems[s.Name]
	if !bootstrap {
		return nil
	}

	// Assume that the UPLINK network on each system is the same, so grab just
	// the gateways from the current node's UPLINK to verify against the other
	// systems' management addrs
	var ip4OVNRanges, ip6OVNRanges []*shared.IPRange

	for _, network := range curSystem.Networks {
		if network.Type == "physical" && network.Name == service.DefaultUplinkNetwork {
			ip4OVNRanges, err = validateGatewayNet(network.Config, "ipv4", validate.IsNetworkAddressCIDRV4)
			if err != nil {
				return err
			}

			ip6OVNRanges, err = validateGatewayNet(network.Config, "ipv6", validate.IsNetworkAddressCIDRV6)
			if err != nil {
				return err
			}

			nameservers, hasNameservers := network.Config["dns.nameservers"]
			if hasNameservers {
				isIP := func(s string) error {
					ip := net.ParseIP(s)
					if ip == nil {
						return fmt.Errorf("Invalid IP %q", s)
					} else {
						return nil
					}
				}

				err = validate.IsListOf(isIP)(nameservers)
				if err != nil {
					return fmt.Errorf("Invalid dns.nameservers: %w", err)
				}
			}

			break
		}
	}

	// Ensure that no system's management address falls within the OVN ranges
	// to prevent OVN from allocating an IP that's already in use.
	for systemName, system := range systems {
		// If the system is ourselves, we don't have an mDNS payload so grab the address locally.
		addr := system.ServerInfo.Address
		if systemName == s.Name {
			addr = s.Address
		}

		systemAddr := net.ParseIP(addr)
		if systemAddr == nil {
			return fmt.Errorf("Invalid address %q for system %q", addr, systemName)
		}

		for _, ipRange := range ip4OVNRanges {
			if ipRange.ContainsIP(systemAddr) {
				return fmt.Errorf("%s ipv4.ovn.ranges must not include system address %q", service.DefaultUplinkNetwork, systemAddr)
			}
		}

		for _, ipRange := range ip6OVNRanges {
			if ipRange.ContainsIP(systemAddr) {
				return fmt.Errorf("%s ipv6.ovn.ranges must not include system address %q", service.DefaultUplinkNetwork, systemAddr)
			}
		}
	}

	return nil
}

// checkClustered checks whether any of the selected systems have already initialized a service.
// Returns the first system we find that is initialized for the given service, along with all of that system's existing cluster members.
func checkClustered(s *service.Handler, autoSetup bool, serviceType types.ServiceType, systems map[string]InitSystem) (firstInitializedSystem string, existingMembers map[string]string, err error) {
	// LXD should always be uninitialized at this point, so we can just return default values that consider LXD uninitialized.
	if serviceType == types.LXD {
		return "", nil, nil
	}

	for peer, system := range systems {
		var remoteClusterMembers map[string]string
		var err error

		// If the peer in question is ourselves, we can just use the unix socket.
		if peer == s.Name {
			remoteClusterMembers, err = s.Services[serviceType].ClusterMembers(context.Background())
		} else {
			remoteClusterMembers, err = s.Services[serviceType].RemoteClusterMembers(context.Background(), system.ServerInfo.AuthSecret, system.ServerInfo.Address)
		}

		if err != nil && err.Error() != "Daemon not yet initialized" {
			return "", nil, fmt.Errorf("Failed to reach %s on system %q: %w", serviceType, peer, err)
		}

		// If we failed to retrieve cluster members due to the system not being initialized, we can ignore it.
		if err != nil {
			continue
		}

		clusterMembers := map[string]string{}
		for k, v := range remoteClusterMembers {
			host, _, err := net.SplitHostPort(v)
			if err != nil {
				return "", nil, err
			}

			clusterMembers[k] = host
		}

		if autoSetup {
			return "", nil, fmt.Errorf("System %q is already clustered on %s", peer, serviceType)
		}

		// If this is the first clustered system we found, then record its cluster members.
		if firstInitializedSystem == "" {
			// Record that this system has initialized the service.
			existingMembers = clusterMembers
			if system.InitializedServices == nil {
				system.InitializedServices = map[types.ServiceType]map[string]string{}
			}

			system.InitializedServices[serviceType] = clusterMembers
			systems[peer] = system
			firstInitializedSystem = peer

			if clusterMembers[peer] != systems[peer].ServerInfo.Address && clusterMembers[peer] != "" {
				return "", nil, fmt.Errorf("%s is already set up on %q on a different network", serviceType, peer)
			}

			continue
		}

		// If we've already encountered a clustered system, check if there's a mismatch in cluster members.
		for k, v := range existingMembers {
			if clusterMembers[k] != v {
				return "", nil, fmt.Errorf("%q and %q are already part of different %s clusters. Aborting initialization", firstInitializedSystem, peer, serviceType)
			}
		}

		// Ensure the maps are identical.
		if len(clusterMembers) != len(existingMembers) {
			return "", nil, fmt.Errorf("Some systems are already part of different %s clusters. Aborting initialization", serviceType)
		}
	}

	return firstInitializedSystem, existingMembers, nil
}

// setupCluster Bootstraps the cluster if necessary, adds all peers to the cluster, and completes any post cluster
// configuration.
func setupCluster(s *service.Handler, bootstrap bool, systems map[string]InitSystem) error {
	initializedServices := map[types.ServiceType]string{}
	bootstrapSystem := systems[s.Name]
	if bootstrap {
		for serviceType := range s.Services {
			for peer, system := range systems {
				if system.InitializedServices[serviceType] != nil {
					initializedServices[serviceType] = peer
					break
				}
			}
		}

		fmt.Println("Initializing a new cluster")
		mu := sync.Mutex{}
		err := s.RunConcurrent(true, false, func(s service.Service) error {
			// If there's already an initialized system for this service, we don't need to bootstrap it.
			if initializedServices[s.Type()] != "" {
				return nil
			}

			if s.Type() == types.MicroCeph {
				microCephBootstrapConf := make(map[string]string)
				if bootstrapSystem.MicroCephInternalNetworkSubnet != "" {
					microCephBootstrapConf["ClusterNet"] = bootstrapSystem.MicroCephInternalNetworkSubnet
				}

				if len(microCephBootstrapConf) > 0 {
					s.SetConfig(microCephBootstrapConf)
				}
			}

			// set a 2 minute timeout to bootstrap a service in case the node is slow.
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			err := s.Bootstrap(ctx)
			if err != nil {
				return fmt.Errorf("Failed to bootstrap local %s: %w", s.Type(), err)
			}

			mu.Lock()
			clustered := systems[s.Name()]
			if clustered.InitializedServices == nil {
				clustered.InitializedServices = map[types.ServiceType]map[string]string{}
			}

			clustered.InitializedServices[s.Type()] = map[string]string{s.Name(): s.Address()}
			systems[s.Name()] = clustered
			mu.Unlock()

			fmt.Printf(" Local %s is ready\n", s.Type())

			return nil
		})
		if err != nil {
			return err
		}
	}

	err := AddPeers(s, systems, bootstrap)
	if err != nil {
		return err
	}

	if bootstrap {
		// Joiners will add their disks as part of the join process, so only add disks here for the system we bootstrapped, or already existed.
		peer := s.Name
		microCeph := initializedServices[types.MicroCeph]
		if microCeph != "" {
			peer = microCeph
		}

		for name := range systems[peer].InitializedServices[types.MicroCeph] {
			// There may be existing cluster members that are not a part of MicroCloud, so ignore those.
			if systems[name].ServerInfo.Name == "" {
				continue
			}

			var c *client.Client
			for _, disk := range systems[name].MicroCephDisks {
				if c == nil {
					c, err = s.Services[types.MicroCeph].(*service.CephService).Client(name, systems[name].ServerInfo.AuthSecret)
					if err != nil {
						return err
					}
				}

				logger.Debug("Adding disk to MicroCeph", logger.Ctx{"name": name, "disk": disk.Path})
				_, err = cephClient.AddDisk(context.Background(), c, &disk)
				if err != nil {
					return err
				}
			}
		}
	}

	fmt.Println("Configuring cluster-wide devices ...")

	var ovnConfig string
	if s.Services[types.MicroOVN] != nil {
		ovn := s.Services[types.MicroOVN].(*service.OVNService)
		c, err := ovn.Client()
		if err != nil {
			return err
		}

		services, err := ovnClient.GetServices(context.Background(), c)
		if err != nil {
			return err
		}

		clusterMap := map[string]string{}
		if bootstrap {
			for peer, system := range systems {
				clusterMap[peer] = system.ServerInfo.Address
			}
		} else {
			cloud := s.Services[types.MicroCloud].(*service.CloudService)
			clusterMap, err = cloud.ClusterMembers(context.Background())
			if err != nil {
				return err
			}
		}

		conns := []string{}
		for _, service := range services {
			if service.Service == "central" {
				addr := s.Address
				if service.Location != s.Name {
					addr = clusterMap[service.Location]
				}

				conns = append(conns, fmt.Sprintf("ssl:%s", util.CanonicalNetworkAddress(addr, 6641)))
			}
		}

		ovnConfig = strings.Join(conns, ",")
	}

	config := map[string]string{"network.ovn.northbound_connection": ovnConfig}
	lxd := s.Services[types.LXD].(*service.LXDService)
	lxdClient, err := lxd.Client(context.Background(), "")
	if err != nil {
		return err
	}

	// Update LXD's global config.
	server, _, err := lxdClient.GetServer()
	if err != nil {
		return err
	}

	newServer := server.Writable()
	changed := false
	for k, v := range config {
		if newServer.Config[k] != v {
			changed = true
		}

		newServer.Config[k] = v
	}

	if changed {
		err = lxdClient.UpdateServer(newServer, "")
		if err != nil {
			return err
		}
	}

	// Create preliminary networks & storage pools on each target.
	for name, system := range systems {
		lxdClient, err := lxd.Client(context.Background(), system.ServerInfo.AuthSecret)
		if err != nil {
			return err
		}

		targetClient := lxdClient.UseTarget(name)
		for _, network := range system.TargetNetworks {
			err = targetClient.CreateNetwork(network)
			if err != nil {
				return err
			}
		}

		for _, pool := range system.TargetStoragePools {
			err = targetClient.CreateStoragePool(pool)
			if err != nil {
				return err
			}
		}
	}

	// If bootstrapping, finalize setup of storage pools & networks, and update the default profile accordingly.
	system, _ := systems[s.Name]
	if bootstrap {
		lxd := s.Services[types.LXD].(*service.LXDService)
		lxdClient, err := lxd.Client(context.Background(), system.ServerInfo.AuthSecret)
		if err != nil {
			return err
		}

		profile := lxdAPI.ProfilesPost{ProfilePut: lxdAPI.ProfilePut{Devices: map[string]map[string]string{}}, Name: "default"}

		for _, network := range system.Networks {
			if network.Name == service.DefaultOVNNetwork || profile.Devices["eth0"] == nil {
				profile.Devices["eth0"] = map[string]string{"name": "eth0", "network": network.Name, "type": "nic"}
			}

			err = lxdClient.CreateNetwork(network)
			if err != nil {
				return err
			}
		}

		cephFSPool := lxdAPI.StoragePoolsPost{}
		for _, pool := range system.StoragePools {
			if pool.Driver == "ceph" || profile.Devices["root"] == nil {
				profile.Devices["root"] = map[string]string{"path": "/", "pool": pool.Name, "type": "disk"}
			}

			// Ensure the cephfs pool is created after the ceph pool so we set up crush rules.
			if pool.Driver == "cephfs" {
				cephFSPool = pool
				continue
			}

			err = lxdClient.CreateStoragePool(pool)
			if err != nil {
				return err
			}
		}

		if cephFSPool.Driver != "" {
			err = lxdClient.CreateStoragePool(cephFSPool)
			if err != nil {
				return err
			}
		}

		profiles, err := lxdClient.GetProfileNames()
		if err != nil {
			return err
		}

		if !shared.ValueInSlice(profile.Name, profiles) {
			err = lxdClient.CreateProfile(profile)
		} else {
			err = lxdClient.UpdateProfile(profile.Name, profile.ProfilePut, "")
		}

		if err != nil {
			return err
		}
	}

	// With storage pools set up, add some volumes for images & backups.
	for name, system := range systems {
		lxdClient, err := lxd.Client(context.Background(), system.ServerInfo.AuthSecret)
		if err != nil {
			return err
		}

		poolNames := []string{}
		if bootstrap {
			for _, pool := range system.TargetStoragePools {
				poolNames = append(poolNames, pool.Name)
			}
		} else {
			for _, cfg := range system.JoinConfig {
				if cfg.Name == "local" || cfg.Name == "remote" {
					if cfg.Entity == "storage-pool" && cfg.Key == "source" {
						poolNames = append(poolNames, cfg.Name)
					}
				}
			}
		}

		targetClient := lxdClient.UseTarget(name)
		for _, pool := range poolNames {
			if pool == "local" {
				err = targetClient.CreateStoragePoolVolume("local", lxdAPI.StorageVolumesPost{Name: "images", Type: "custom"})
				if err != nil {
					return err
				}

				err = targetClient.CreateStoragePoolVolume("local", lxdAPI.StorageVolumesPost{Name: "backups", Type: "custom"})
				if err != nil {
					return err
				}

				server, _, err := targetClient.GetServer()
				if err != nil {
					return err
				}

				newServer := server.Writable()
				newServer.Config["storage.backups_volume"] = "local/backups"
				newServer.Config["storage.images_volume"] = "local/images"
				err = targetClient.UpdateServer(newServer, "")
				if err != nil {
					return err
				}
			}
		}
	}

	fmt.Println("MicroCloud is ready")

	return nil
}
