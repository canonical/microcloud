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
	cli "github.com/canonical/lxd/shared/cmd"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/lxd/shared/validate"
	cephTypes "github.com/canonical/microceph/microceph/api/types"
	cephClient "github.com/canonical/microceph/microceph/client"
	clientv1 "github.com/canonical/microcluster/client"
	ovnClient "github.com/canonical/microovn/microovn/client"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/mdns"
	"github.com/canonical/microcloud/microcloud/service"
)

// DefaultAutoLookupTimeout is the default time limit for automatically finding systems over mDNS.
const DefaultAutoLookupTimeout time.Duration = 5 * time.Second

// DefaultLookupTimeout is the default time limit for finding systems interactively.
const DefaultLookupTimeout time.Duration = time.Minute

// InitSystem represents the configuration passed to individual systems that join via the Handler.
type InitSystem struct {
	// ServerInfo contains the data reported by mDNS about this system.
	ServerInfo mdns.ServerInfo
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

// initConfig holds the configuration for cluster formation based on the initial flags and answers provided to MicroCloud.
type initConfig struct {
	// common holds information common to the CLI.
	common *CmdControl

	// asker is the CLI user input helper.
	asker *cli.Asker

	// address is the cluster address of the local system.
	address string

	// name is the cluster name for the local system.
	name string

	// bootstrap indicates whether we are setting up a new system from scratch.
	bootstrap bool

	// autoSetup indicates whether questions should automatically choose defaults.
	autoSetup bool

	// lookupTimeout is the duration to wait for mDNS records to appear during system lookup.
	lookupTimeout time.Duration

	// wipeAllDisks indicates whether all disks should be wiped, or if the user should be prompted.
	wipeAllDisks bool

	// encryptAllDisks indicates whether all disks should be encrypted, or if the user should be prompted.
	encryptAllDisks bool

	// lookupIface is the interface used for mDNS lookup.
	lookupIface *net.Interface

	// lookupSubnet is the subnet to limit mDNS lookup over.
	lookupSubnet *net.IPNet

	// systems is a map of system configuration to supply for cluster creation.
	systems map[string]InitSystem

	// state is the current state information for each system.
	state map[string]service.SystemInformation
}

type cmdInit struct {
	common *CmdControl

	flagAutoSetup       bool
	flagLookupTimeout   int64
	flagWipeAllDisks    bool
	flagEncryptAllDisks bool
	flagAddress         string
	flagPreseed         bool
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
	cmd.Flags().BoolVar(&c.flagEncryptAllDisks, "encrypt", false, "Encrypt disks to add to MicroCeph")
	cmd.Flags().StringVar(&c.flagAddress, "address", "", "Address to use for MicroCloud")
	cmd.Flags().BoolVar(&c.flagPreseed, "preseed", false, "Expect Preseed YAML for configuring MicroCloud in stdin")
	cmd.Flags().Int64Var(&c.flagLookupTimeout, "lookup-timeout", 0, "Amount of seconds to wait for systems to show up. Defaults: 60s for interactive, 5s for automatic and preseed")

	return cmd
}

func (c *cmdInit) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
	}

	cfg := initConfig{
		bootstrap:       true,
		address:         c.flagAddress,
		autoSetup:       c.flagAutoSetup,
		wipeAllDisks:    c.flagWipeAllDisks,
		encryptAllDisks: c.flagEncryptAllDisks,
		common:          c.common,
		asker:           &c.common.asker,
		systems:         map[string]InitSystem{},
		state:           map[string]service.SystemInformation{},
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

	return cfg.RunInteractive(cmd, args)
}

func (c *initConfig) RunInteractive(cmd *cobra.Command, args []string) error {
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

	err = c.askAddress()
	if err != nil {
		return err
	}

	c.name, err = os.Hostname()
	if err != nil {
		return fmt.Errorf("Failed to retrieve system hostname: %w", err)
	}

	c.systems[c.name] = InitSystem{
		ServerInfo: mdns.ServerInfo{
			Name:    c.name,
			Address: c.address,
		},
	}

	services := []types.ServiceType{types.MicroCloud, types.LXD}
	optionalServices := map[types.ServiceType]string{
		types.MicroCeph: api.MicroCephDir,
		types.MicroOVN:  api.MicroOVNDir,
	}

	services, err = c.askMissingServices(services, optionalServices)
	if err != nil {
		return err
	}

	s, err := service.NewHandler(c.name, c.address, c.common.FlagMicroCloudDir, c.common.FlagLogDebug, c.common.FlagLogVerbose, c.common.FlagSocketGroup, services...)
	if err != nil {
		return err
	}

	err = c.lookupPeers(s, nil)
	if err != nil {
		return err
	}

	state, err := s.CollectSystemInformation(context.Background(), mdns.ServerInfo{Name: c.name, Address: c.address, Services: services})
	if err != nil {
		return err
	}

	c.state[c.name] = *state
	fmt.Println("Gathering system information...")
	for _, system := range c.systems {
		if system.ServerInfo.Name == "" || system.ServerInfo.Name == c.name {
			continue
		}

		state, err := s.CollectSystemInformation(context.Background(), system.ServerInfo)
		if err != nil {
			return err
		}

		c.state[system.ServerInfo.Name] = *state
	}

	// Ensure LXD is not already clustered if we are running `microcloud init`.
	for _, info := range c.state {
		if info.ServiceClustered(types.LXD) {
			return fmt.Errorf("%s is already clustered on %q, aborting setup", types.LXD, info.ClusterName)
		}
	}

	// Ensure there are no existing cluster conflicts.
	conflict, serviceType := service.ClustersConflict(c.state, services)
	if conflict {
		return fmt.Errorf("Some systems are already part of different %s clusters. Aborting initialization", serviceType)
	}

	// Ask to reuse existing clusters.
	err = c.askClustered(s, services)
	if err != nil {
		return err
	}

	err = c.askDisks(s)
	if err != nil {
		return err
	}

	err = c.askNetwork(s)
	if err != nil {
		return err
	}

	err = c.validateSystems(s)
	if err != nil {
		return err
	}

	err = c.setupCluster(s)
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
func (c *initConfig) lookupPeers(s *service.Handler, expectedSystems []string) error {
	header := []string{"NAME", "IFACE", "ADDR"}
	var table *SelectableTable
	var answers []string

	autoSetup := c.autoSetup
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

	ctx, cancel := context.WithTimeout(context.Background(), c.lookupTimeout)
	defer cancel()

	expectedSystemsMap := make(map[string]bool, len(expectedSystems))
	for _, system := range expectedSystems {
		expectedSystemsMap[system] = true
	}

	fmt.Println("Scanning for eligible servers ...")
	totalPeers := map[string]mdns.ServerInfo{}
	done := false
	for !done {
		select {
		case <-ctx.Done():
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

			peers, err := mdns.LookupPeers(ctx, c.lookupIface, mdns.Version, s.Name)
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
					if c.lookupSubnet != nil && !c.lookupSubnet.Contains(net.ParseIP(info.Address)) {
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
				c.systems[peer] = InitSystem{
					ServerInfo: info,
				}
			}
		}
	}

	if autoSetup {
		for _, info := range totalPeers {
			c.systems[info.Name] = InitSystem{
				ServerInfo: info,
			}
		}

		if len(expectedSystems) > 0 {
			return nil
		}

		// Add a space between the CLI and the response.
		fmt.Println("")
	}

	for _, info := range c.systems {
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

func (c *initConfig) addPeers(sh *service.Handler) error {
	// Grab the systems that are clustered from the InitSystem map.
	initializedServices := map[types.ServiceType]string{}
	existingSystems := map[types.ServiceType]map[string]string{}
	for serviceType := range sh.Services {
		for peer := range c.systems {
			if c.state[peer].ExistingServices != nil && c.state[peer].ExistingServices[serviceType] != nil {
				initializedServices[serviceType] = peer
				existingSystems[serviceType] = c.state[peer].ExistingServices[serviceType]
				break
			}
		}
	}

	// Prepare a JoinConfig to send to each joiner.
	joinConfig := make(map[string]types.ServicesPut, len(c.systems))
	for peer, info := range c.systems {
		joinConfig[peer] = types.ServicesPut{
			Tokens:     []types.ServiceToken{},
			Address:    info.ServerInfo.Address,
			LXDConfig:  info.JoinConfig,
			CephConfig: info.MicroCephDisks,
		}
	}

	clusterSize := map[types.ServiceType]int{}
	if c.bootstrap {
		for serviceType, clusterMembers := range existingSystems {
			clusterSize[serviceType] = len(clusterMembers)
		}
	}

	// Concurrently issue a token for each joiner.
	for peer := range c.systems {
		mut := sync.Mutex{}
		err := sh.RunConcurrent("", "", func(s service.Service) error {
			// Only issue a token if the system isn't already part of that cluster.
			if existingSystems[s.Type()][peer] == "" {
				clusteredSystem := c.systems[initializedServices[s.Type()]]

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
				if !c.bootstrap {
					currentCluster, err = s.ClusterMembers(context.Background())
					if err != nil {
						return fmt.Errorf("Failed to check for existing %s cluster size: %w", s.Type(), err)
					}
				}

				mut.Lock()

				if !c.bootstrap {
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
		err := waitForJoin(sh, clusterSize, c.systems[peer].ServerInfo.AuthSecret, peer, cfg)
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

func (c *initConfig) validateSystems(s *service.Handler) (err error) {
	if !c.bootstrap {
		return nil
	}

	// Assume that the UPLINK network on each system is the same, so grab just
	// the gateways from the current node's UPLINK to verify against the other
	// systems' management addrs
	var ip4OVNRanges, ip6OVNRanges []*shared.IPRange

	for _, network := range c.systems[s.Name].Networks {
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
	for systemName, system := range c.systems {
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

// setupCluster Bootstraps the cluster if necessary, adds all peers to the cluster, and completes any post cluster
// configuration.
func (c *initConfig) setupCluster(s *service.Handler) error {
	initializedServices := map[types.ServiceType]string{}
	bootstrapSystem := c.systems[s.Name]
	if c.bootstrap {
		for serviceType := range s.Services {
			for peer := range c.systems {
				if c.state[peer].ExistingServices[serviceType] != nil {
					initializedServices[serviceType] = peer
					break
				}
			}
		}

		fmt.Println("Initializing a new cluster")
		mu := sync.Mutex{}
		err := s.RunConcurrent(types.MicroCloud, "", func(s service.Service) error {
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

			// Since the system is now clustered, update its existing services map.
			mu.Lock()
			clustered := c.state[s.Name()]
			if clustered.ExistingServices == nil {
				clustered.ExistingServices = map[types.ServiceType]map[string]string{}
			}

			clustered.ExistingServices[s.Type()] = map[string]string{s.Name(): s.Address()}
			c.state[s.Name()] = clustered
			mu.Unlock()

			fmt.Printf(" Local %s is ready\n", s.Type())

			return nil
		})
		if err != nil {
			return err
		}
	}

	err := c.addPeers(s)
	if err != nil {
		return err
	}

	if c.bootstrap {
		// Joiners will add their disks as part of the join process, so only add disks here for the system we bootstrapped, or already existed.
		peer := s.Name
		microCeph := initializedServices[types.MicroCeph]
		if microCeph != "" {
			peer = microCeph
		}

		for name := range c.state[peer].ExistingServices[types.MicroCeph] {
			// There may be existing cluster members that are not a part of MicroCloud, so ignore those.
			if c.systems[name].ServerInfo.Name == "" {
				continue
			}

			var legacyClient *clientv1.Client
			for _, disk := range c.systems[name].MicroCephDisks {
				if legacyClient == nil {
					legacyClient, err = s.Services[types.MicroCeph].(*service.CephService).LegacyClient(name, c.systems[name].ServerInfo.AuthSecret)
					if err != nil {
						return err
					}
				}

				logger.Debug("Adding disk to MicroCeph", logger.Ctx{"name": name, "disk": disk.Path})
				_, err = cephClient.AddDisk(context.Background(), legacyClient, &disk)
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
		legacyClient, err := ovn.LegacyClient()
		if err != nil {
			return err
		}

		services, err := ovnClient.GetServices(context.Background(), legacyClient)
		if err != nil {
			return err
		}

		clusterMap := map[string]string{}
		if c.bootstrap {
			for peer, system := range c.systems {
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
	for name, system := range c.systems {
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
	system := c.systems[s.Name]
	if c.bootstrap {
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
			if err != nil {
				return err
			}
		} else {
			// Ensure any pre-existing devices and config are carried over to the new profile, unless we are managing them.
			existingProfile, _, err := lxdClient.GetProfile("default")
			if err != nil {
				return err
			}

			askConflictingConfig := []string{}
			askConflictingDevices := []string{}
			for k, v := range profile.Config {
				_, ok := existingProfile.Config[k]
				if !ok {
					existingProfile.Config[k] = v
				} else {
					askConflictingConfig = append(askConflictingConfig, k)
				}
			}

			for k, v := range profile.Devices {
				_, ok := existingProfile.Devices[k]
				if !ok {
					existingProfile.Devices[k] = v
				} else {
					askConflictingDevices = append(askConflictingDevices, k)
				}
			}

			if !c.autoSetup && len(askConflictingConfig) > 0 || len(askConflictingDevices) > 0 {
				replace, err := c.asker.AskBool("Replace existing default profile configuration? (yes/no) [default=no]: ", "no")
				if err != nil {
					return err
				}

				if replace {
					for _, key := range askConflictingConfig {
						existingProfile.Config[key] = profile.Config[key]
					}

					for _, key := range askConflictingDevices {
						existingProfile.Devices[key] = profile.Devices[key]
					}
				}
			}

			err = lxdClient.UpdateProfile(profile.Name, existingProfile.Writable(), "")
			if err != nil {
				return err
			}
		}
	}

	// With storage pools set up, add some volumes for images & backups.
	for name, system := range c.systems {
		lxdClient, err := lxd.Client(context.Background(), system.ServerInfo.AuthSecret)
		if err != nil {
			return err
		}

		poolNames := []string{}
		if c.bootstrap {
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
