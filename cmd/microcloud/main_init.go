package main

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared"
	lxdAPI "github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/lxd/shared/revert"
	"github.com/canonical/lxd/shared/validate"
	cephTypes "github.com/canonical/microceph/microceph/api/types"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/api/types"
	cloudClient "github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcloud/microcloud/cmd/tui"
	"github.com/canonical/microcloud/microcloud/multicast"
	"github.com/canonical/microcloud/microcloud/service"
)

// DefaultAutoLookupTimeout is the default time limit for automatically finding systems over multicast.
const DefaultAutoLookupTimeout time.Duration = 5 * time.Second

// DefaultLookupTimeout is the default time limit for finding systems interactively.
const DefaultLookupTimeout time.Duration = time.Minute

// RecommendedOSDHosts is the minimum number of OSD hosts recommended for a new cluster for fault-tolerance.
const RecommendedOSDHosts = 3

// DefaultAutoSessionTimeout is the default time limit for an automatic trust establishment session.
const DefaultAutoSessionTimeout time.Duration = 10 * time.Minute

// DefaultSessionTimeout is the default time limit for the trust establishment session.
const DefaultSessionTimeout time.Duration = 60 * time.Minute

// LXDInitializationTimeout is the time limit for LXD initialization for microcloud.
const LXDInitializationTimeout time.Duration = 1 * time.Minute

// InitSystem represents the configuration passed to individual systems that join via the Handler.
type InitSystem struct {
	// ServerInfo contains the data reported about this system.
	ServerInfo multicast.ServerInfo
	// AvailableDisks contains the disks as reported by LXD.
	AvailableDisks []lxdAPI.ResourcesStorageDisk
	// MicroCephDisks contains the disks intended to be passed to MicroCeph.
	MicroCephDisks []cephTypes.DisksPost
	// MicroCephPublicNetwork specifies the optional public network configuration for Ceph.
	// Includes the subnet (IPv4/IPv6 CIDR), network interface name, and IP address within the subnet.
	MicroCephPublicNetwork *NetworkInterfaceInfo
	// MicroCephInternalNetwork specifies the optional cluster network configuration for Ceph.
	// Includes the subnet (IPv4/IPv6 CIDR), network interface name, and IP address within the subnet.
	MicroCephInternalNetwork *NetworkInterfaceInfo
	// MicroCloudInternalNetwork contains the network configuration for the MicroCloud internal network.
	// Includes the subnet (IPv4/IPv6 CIDR), network interface name, and IP address within the subnet.
	MicroCloudInternalNetwork *NetworkInterfaceInfo
	// TargetNetworks contains the network configuration for the target system.
	TargetNetworks []lxdAPI.NetworksPost
	// TargetStoragePools contains the storage pool configuration for the target system.
	TargetStoragePools []lxdAPI.StoragePoolsPost
	// Networks is the cluster-wide network configuration.
	Networks []lxdAPI.NetworksPost
	// OVNGeneveNetwork specifies the configuration for the OVN Geneve tunnel.
	// Includes the IP address, network interface name, and subnet to use for Geneve traffic.
	// If left empty, Geneve traffic will be routed through the management network.
	OVNGeneveNetwork *NetworkInterfaceInfo
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
	asker *tui.InputHandler

	// address is the cluster address of the local system.
	address string

	// name is the cluster name for the local system.
	name string

	// bootstrap indicates whether we are setting up a new system from scratch.
	bootstrap bool

	// autoSetup indicates whether questions should automatically choose defaults.
	autoSetup bool

	// setupMany indicates whether we are setting up remote nodes concurrently, or just a single cluster member.
	setupMany bool

	// lookupTimeout is the duration to wait for peers to appear during multicast system lookup.
	lookupTimeout time.Duration

	// sessionTimeout is the duration to wait for the trust establishment session to complete.
	sessionTimeout time.Duration

	// lookupIface is the interface used for multicast lookup.
	lookupIface *net.Interface

	// lookupSubnet is the subnet in which other peers are being expected.
	// It represents the internal network used for MicroCloud.
	lookupSubnet *net.IPNet

	// systems is a map of system configuration to supply for cluster creation.
	systems map[string]InitSystem

	// state is the current state information for each system.
	state map[string]service.SystemInformation
}

type cmdInit struct {
	common *CmdControl

	flagSessionTimeout int64
}

// command returns the subcommand for initializing a MicroCloud.
func (c *cmdInit) command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "init",
		Aliases: []string{"bootstrap"},
		Short:   "Initialize MicroCloud and create a new cluster",
		RunE:    c.run,
	}

	cmd.Flags().Int64Var(&c.flagSessionTimeout, "session-timeout", 0, "Amount of seconds to wait for the trust establishment session. Defaults: 60m")

	return cmd
}

// run runs the subcommand for initializing a MicroCloud.
func (c *cmdInit) run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
	}

	cfg := initConfig{
		bootstrap: true,
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

	return cfg.runInteractive(cmd, args)
}

// runInteractive runs the interactive subcommand for initializing a MicroCloud.
func (c *initConfig) runInteractive(cmd *cobra.Command, args []string) error {
	fmt.Println("Waiting for services to start ...")
	err := checkInitialized(c.common.FlagMicroCloudDir, false, false)
	if err != nil {
		return err
	}

	c.setupMany, err = c.common.asker.AskBool("Do you want to set up more than one cluster member?", true)
	if err != nil {
		return err
	}

	c.name, err = os.Hostname()
	if err != nil {
		return fmt.Errorf("Failed to retrieve system hostname: %w", err)
	}

	c.systems[c.name] = InitSystem{
		ServerInfo: multicast.ServerInfo{
			Name:    c.name,
			Address: c.address,
		},
	}

	err = c.askAddress("")
	if err != nil {
		return err
	}

	installedServices := []types.ServiceType{types.MicroCloud, types.LXD}
	optionalServices := map[types.ServiceType]string{
		types.MicroCeph: api.MicroCephDir,
		types.MicroOVN:  api.MicroOVNDir,
	}

	installedServices, err = c.askMissingServices(installedServices, optionalServices)
	if err != nil {
		return err
	}

	// check the API for service versions.
	s, err := service.NewHandler(c.name, c.address, c.common.FlagMicroCloudDir, installedServices...)
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

	var reverter *revert.Reverter
	if c.setupMany {
		err = c.runSession(context.Background(), s, types.SessionInitiating, c.sessionTimeout, func(gw *cloudClient.WebsocketGateway) error {
			return c.initiatingSession(gw, s, services, "", nil)
		})
		if err != nil {
			return err
		}

		reverter = revert.New()
		defer reverter.Fail()

		reverter.Add(func() {
			// Stop each joiner member session.
			cloud := s.Services[types.MicroCloud].(*service.CloudService)
			for peer, system := range c.systems {
				if system.ServerInfo.Name == "" || system.ServerInfo.Name == c.name {
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
	}

	state, err := s.CollectSystemInformation(context.Background(), multicast.ServerInfo{Name: c.name, Address: c.address, Services: services})
	if err != nil {
		return err
	}

	c.state[c.name] = *state
	fmt.Println("Gathering system information ...")
	for peer, system := range c.systems {
		if system.ServerInfo.Name == "" || system.ServerInfo.Name == c.name {
			continue
		}

		state, err := s.CollectSystemInformation(context.Background(), system.ServerInfo)
		if err != nil {
			return err
		}

		c.state[system.ServerInfo.Name] = *state

		// Initialize MicroCloud network for other peers.
		err = populateMicroCloudNetworkFromState(state, peer, &system, c.lookupSubnet)
		if err != nil {
			return err
		}
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

	if c.setupMany {
		reverter.Success()
	}

	return nil
}

func populateMicroCloudNetworkFromState(state *service.SystemInformation, peer string, system *InitSystem, lookupSubnet *net.IPNet) error {
	isLookupSubnet := lookupSubnet.IP.To4() != nil
microCloudPeerIfaceFound:
	for iface, network := range state.AvailableMicroCloudInterfaces {
		for _, addr := range network.Addresses {
			ip, _, err := net.ParseCIDR(addr)
			if err != nil {
				return fmt.Errorf("Failed to parse available network interface CIDR address: %q: %w", addr, err)
			}

			isAddr := ip.To4() != nil
			// Lookup subnet address and IP address should be of the same type (IPv4 or IPv6).
			if isLookupSubnet == isAddr && lookupSubnet.Contains(ip) {
				system.MicroCloudInternalNetwork = &NetworkInterfaceInfo{
					Interface: net.Interface{Name: iface},
					Subnet:    lookupSubnet,
					IP:        ip,
				}

				break microCloudPeerIfaceFound
			}
		}
	}

	if system.MicroCloudInternalNetwork == nil {
		return fmt.Errorf("Failed to initialize a suitable network interface for MicroCloud on %q", peer)
	}

	return nil
}

// waitForJoin requests a system to join each service's respective cluster,
// and then waits for the request to either complete or time out.
// If the request was successful, it additionally waits until the cluster appears in the database.
func waitForJoin(sh *service.Handler, clusterSizes map[types.ServiceType]int, peer string, cert *x509.Certificate, cfg types.ServicesPut) error {
	cloud := sh.Services[types.MicroCloud].(*service.CloudService)
	err := cloud.RequestJoin(context.Background(), peer, cert, cfg)
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

	return nil
}

func (c *initConfig) addPeers(sh *service.Handler) (revert.Hook, error) {
	reverter := revert.New()
	defer reverter.Fail()

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

		if info.OVNGeneveNetwork != nil {
			p := joinConfig[peer]
			p.OVNConfig = map[string]string{"ovn-encap-ip": info.OVNGeneveNetwork.IP.String()}
			joinConfig[peer] = p
		}
	}

	clusterSize := map[types.ServiceType]int{}
	for serviceType, clusterMembers := range existingSystems {
		clusterSize[serviceType] = len(clusterMembers)
	}

	// First let each joiner join the MicroCloud cluster.
	// This ensures that the tokens for existing services can be issued on the remote system already using mTLS.
	for peer := range c.systems {
		// Only join other peers which aren't yet part of MicroCloud.
		if peer != sh.Name && existingSystems[types.MicroCloud][peer] == "" {
			token, err := sh.Services[types.MicroCloud].IssueToken(context.Background(), peer)
			if err != nil {
				return nil, fmt.Errorf("Failed to issue MicroCloud token for peer %q: %w", peer, err)
			}

			cfg := joinConfig[peer]
			cfg.Tokens = append(cfg.Tokens, types.ServiceToken{Service: types.MicroCloud, JoinToken: token})

			cert := c.systems[peer].ServerInfo.Certificate
			err = waitForJoin(sh, clusterSize, peer, cert, cfg)
			if err != nil {
				return nil, err
			}
		}
	}

	// Concurrently issue a token for each joiner.
	for peer := range c.systems {
		mut := sync.Mutex{}
		err := sh.RunConcurrent("", "", func(s service.Service) error {
			// Skip MicroCloud as the cluster is already formed.
			if s.Type() == types.MicroCloud {
				return nil
			}

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
					token, err = cloud.RemoteIssueToken(context.Background(), clusteredSystem.ServerInfo.Address, peer, s.Type())
					if err != nil {
						return err
					}
				}

				mut.Lock()
				reverter.Add(func() {
					err = s.DeleteToken(context.Background(), peer, clusteredSystem.ServerInfo.Address)
					if err != nil {
						logger.Error("Failed to clean up join token", logger.Ctx{"service": s.Type(), "error": err})
					}
				})

				cfg := joinConfig[peer]
				cfg.Tokens = append(cfg.Tokens, types.ServiceToken{Service: s.Type(), JoinToken: token})
				joinConfig[peer] = cfg
				mut.Unlock()
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	fmt.Println("Awaiting cluster formation ...")

	// If the local node needs to join an existing cluster, do it first so we can proceed as normal.
	if len(joinConfig[sh.Name].Tokens) > 0 {
		cfg := joinConfig[sh.Name]
		err := waitForJoin(sh, clusterSize, sh.Name, nil, cfg)
		if err != nil {
			return nil, err
		}

		fmt.Println(tui.SummarizeResult("Peer %s has joined the cluster", sh.Name))
	}

	for peer, cfg := range joinConfig {
		if len(cfg.Tokens) == 0 || peer == sh.Name {
			continue
		}

		logger.Debug("Initiating sequential request for cluster join", logger.Ctx{"peer": peer})
		err := waitForJoin(sh, clusterSize, peer, nil, cfg)
		if err != nil {
			return nil, err
		}

		fmt.Println(tui.SummarizeResult("Peer %s has joined the cluster", peer))
	}

	cleanup := reverter.Clone().Fail

	reverter.Success()

	return cleanup, nil
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
		// If the system is ourselves, we don't have a multicast discovery payload so grab the address locally.
		addr := system.ServerInfo.Address
		if systemName == s.Name {
			addr = s.Address()
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
	reverter := revert.New()
	defer reverter.Fail()

	lxd := s.Services[types.LXD].(*service.LXDService)
	lxdClient, err := lxd.Client(context.Background())
	if err != nil {
		return err
	}

	// If bootstrapping, finalize setup of storage pools & networks, and update the default profile accordingly.
	system := c.systems[s.Name]
	profile := lxdAPI.ProfilesPost{ProfilePut: lxdAPI.ProfilePut{Devices: map[string]map[string]string{}}, Name: "default"}
	profiles, err := lxdClient.GetProfileNames()
	if err != nil {
		return err
	}

	for _, network := range system.Networks {
		if network.Name == service.DefaultOVNNetwork || profile.Devices["eth0"] == nil {
			profile.Devices["eth0"] = map[string]string{"name": "eth0", "network": network.Name, "type": "nic"}
		}
	}

	for _, pool := range system.StoragePools {
		if pool.Driver == "ceph" || profile.Devices["root"] == nil {
			profile.Devices["root"] = map[string]string{"path": "/", "pool": pool.Name, "type": "disk"}
		}
	}

	newProfile, err := c.askUpdateProfile(profile, profiles, lxdClient)
	if err != nil {
		return err
	}

	profile.ProfilePut = *newProfile

	initializedServices := map[types.ServiceType]string{}
	bootstrapSystem := c.systems[s.Name]
	for serviceType := range s.Services {
		for peer := range c.systems {
			if c.state[peer].ExistingServices[serviceType] != nil {
				initializedServices[serviceType] = peer
				break
			}
		}
	}

	fmt.Println("Initializing new services ...")
	mu := sync.Mutex{}
	err = s.RunConcurrent(types.MicroCloud, types.LXD, func(s service.Service) error {
		// If there's already an initialized system for this service, we don't need to bootstrap it.
		if initializedServices[s.Type()] != "" {
			return nil
		}

		if s.Type() == types.MicroCeph {
			microCephBootstrapConf := make(map[string]string)
			if bootstrapSystem.MicroCephInternalNetwork != nil {
				microCephBootstrapConf["ClusterNet"] = bootstrapSystem.MicroCephInternalNetwork.Subnet.String()
			}

			if bootstrapSystem.MicroCephPublicNetwork != nil {
				microCephBootstrapConf["PublicNet"] = bootstrapSystem.MicroCephPublicNetwork.Subnet.String()
			}

			if len(microCephBootstrapConf) > 0 {
				s.SetConfig(microCephBootstrapConf)
			}
		}

		if s.Type() == types.MicroOVN {
			microOvnBootstrapConf := make(map[string]string)
			if bootstrapSystem.OVNGeneveNetwork != nil {
				microOvnBootstrapConf["ovn-encap-ip"] = bootstrapSystem.OVNGeneveNetwork.IP.String()
			}

			if len(microOvnBootstrapConf) > 0 {
				s.SetConfig(microOvnBootstrapConf)
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

		fmt.Println(tui.SummarizeResult("Local %s is ready", s.Type()))

		return nil
	})
	if err != nil {
		return err
	}

	cleanup, err := c.addPeers(s)
	if err != nil {
		return err
	}

	reverter.Add(cleanup)

	peer := s.Name
	microCeph := initializedServices[types.MicroCeph]
	if microCeph != "" {
		peer = microCeph
	}

	if s.Services[types.MicroCeph] != nil {
		for name := range c.state[peer].ExistingServices[types.MicroCeph] {
			// There may be existing cluster members that are not a part of MicroCloud, so ignore those.
			if c.systems[name].ServerInfo.Name == "" {
				continue
			}

			for _, disk := range c.systems[name].MicroCephDisks {
				logger.Debug("Adding disk to MicroCeph", logger.Ctx{"name": name, "disk": disk.Path})
				resp, err := s.Services[types.MicroCeph].(*service.CephService).AddDisk(context.Background(), disk, name)
				if err != nil {
					return err
				}

				var diskErr string
				for _, report := range resp.Reports {
					if report.Error != "" {
						if diskErr == "" {
							diskErr = report.Error
						} else {
							// Populate errors backwards, as the latest error is at the end of the list.
							diskErr = fmt.Sprintf("%s: %s", report.Error, diskErr)
						}
					}
				}

				if diskErr != "" {
					return fmt.Errorf("Failed to add disk to MicroCeph: %s", diskErr)
				}
			}
		}

		cephService := s.Services[types.MicroCeph].(*service.CephService)

		allDisks, err := cephService.GetDisks(context.Background(), "", nil)
		if err != nil {
			return err
		}

		if len(allDisks) > 0 {
			defaultPoolSize := len(allDisks)
			if defaultPoolSize > RecommendedOSDHosts {
				defaultPoolSize = RecommendedOSDHosts
			}

			pools, err := cephService.GetPools(context.Background(), s.Name)
			if err != nil {
				return err
			}

			defaultOSDPools := map[string]bool{
				service.DefaultMgrOSDPool:        true,
				service.DefaultCephFSDataOSDPool: true,
				service.DefaultCephFSMetaOSDPool: true,
				service.DefaultCephFSOSDPool:     true,
				service.DefaultCephOSDPool:       true,
			}

			poolsToUpdate := []string{}
			for _, pool := range pools {
				if defaultOSDPools[pool.Pool] && pool.Size < int64(defaultPoolSize) {
					poolsToUpdate = append(poolsToUpdate, pool.Pool)
				}
			}

			// If there are no OSD pools, MicroCeph requires us to pass an empty string to set the default OSD pool size.
			if len(poolsToUpdate) == 0 {
				poolsToUpdate = append(poolsToUpdate, "")
			}

			err = cephService.PoolSetReplicationFactor(context.Background(), cephTypes.PoolPut{Pools: poolsToUpdate, Size: int64(defaultPoolSize)}, s.Name)
			if err != nil {
				return err
			}
		}
	}

	fmt.Println("Configuring cluster-wide devices ...")

	var ovnConfig string
	if s.Services[types.MicroOVN] != nil {
		serviceOVN := s.Services[types.MicroOVN].(*service.OVNService)

		services, err := serviceOVN.GetServices(context.Background())
		if err != nil {
			return err
		}

		clusterMap := map[string]string{}
		for peer, system := range c.systems {
			clusterMap[peer] = system.ServerInfo.Address
		}

		conns := []string{}
		for _, service := range services {
			if service.Service == "central" {
				addr := s.Address()
				if service.Location != s.Name {
					addr = clusterMap[service.Location]
				}

				conns = append(conns, "ssl:"+util.CanonicalNetworkAddress(addr, 6641))
			}
		}

		ovnConfig = strings.Join(conns, ",")
	}

	config := map[string]string{"network.ovn.northbound_connection": ovnConfig}
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

	reverter.Add(func() {
		if !c.bootstrap {
			return
		}

		system := c.systems[s.Name]
		lxdClient, err := lxd.Client(context.Background())
		if err != nil {
			logger.Error("Failed to get LXD client for cleanup", logger.Ctx{"error": err})

			return
		}

		for _, network := range system.Networks {
			_ = lxdClient.DeleteNetwork(network.Name)
		}

		for _, pool := range system.StoragePools {
			_ = lxdClient.DeleteStoragePool(pool.Name)
		}
	})

	// Create preliminary networks & storage pools on each target.
	for name, system := range c.systems {
		lxdClient, err := lxd.Client(context.Background())
		if err != nil {
			return err
		}

		targetClient := lxdClient.UseTarget(name)

		for _, pool := range system.TargetStoragePools {
			err = targetClient.CreateStoragePool(pool)
			if err != nil {
				return err
			}
		}

		for _, network := range system.TargetNetworks {
			err = targetClient.CreateNetwork(network)
			if err != nil {
				return err
			}
		}
	}

	cephFSPool := lxdAPI.StoragePoolsPost{}
	for _, pool := range system.StoragePools {
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

	for _, network := range system.Networks {
		err = lxdClient.CreateNetwork(network)
		if err != nil {
			return err
		}
	}

	if !slices.Contains(profiles, profile.Name) {
		err = lxdClient.CreateProfile(profile)
		if err != nil {
			return err
		}
	} else {
		op, err := lxdClient.UpdateProfile(profile.Name, profile.ProfilePut, "")
		if err != nil {
			return fmt.Errorf("Failed to update profile %q: %w", profile.Name, err)
		}

		err = op.Wait()
		if err != nil {
			return fmt.Errorf("Failed to wait for profile %q to update: %w", profile.Name, err)
		}
	}

	// With storage pools set up, add some volumes for images & backups.
	for name, system := range c.systems {
		lxdClient, err := lxd.Client(context.Background())
		if err != nil {
			return err
		}

		poolNames := []string{}

		// In case any storage pools are marked for initial setup,
		// add them to the list of available storage pool names.
		for _, pool := range system.TargetStoragePools {
			poolNames = append(poolNames, pool.Name)
		}

		// When joining the selected system, it can grow either the local or remote storage pool.
		// In this case add the pool's name to the list of available storage pools.
		for _, cfg := range system.JoinConfig {
			if cfg.Name == "local" || cfg.Name == "remote" {
				if cfg.Entity == "storage-pool" && cfg.Key == "source" {
					poolNames = append(poolNames, cfg.Name)
				}
			}
		}

		targetClient := lxdClient.UseTarget(name)
		for _, pool := range poolNames {
			if pool == "local" {
				server, _, err := targetClient.GetServer()
				if err != nil {
					return err
				}

				reverter.Add(func() {
					_ = targetClient.UpdateServer(server.Writable(), "")
				})

				op, err := targetClient.CreateStoragePoolVolume("local", lxdAPI.StorageVolumesPost{Name: "images", Type: "custom"})
				if err != nil {
					return fmt.Errorf("Failed to create volume %q on pool %q: %w", "images", "local", err)
				}

				err = op.Wait()
				if err != nil {
					return fmt.Errorf("Failed to wait for volume %q on pool %q: %w", "images", "local", err)
				}

				reverter.Add(func() {
					op, err := targetClient.DeleteStoragePoolVolume("local", "custom", "images")
					if err == nil {
						_ = op.Wait()
					}
				})

				op, err = targetClient.CreateStoragePoolVolume("local", lxdAPI.StorageVolumesPost{Name: "backups", Type: "custom"})
				if err != nil {
					return fmt.Errorf("Failed to create volume %q on pool %q: %w", "backups", "local", err)
				}

				err = op.Wait()
				if err != nil {
					return fmt.Errorf("Failed to wait for volume %q on pool %q: %w", "backups", "local", err)
				}

				reverter.Add(func() {
					op, err = targetClient.DeleteStoragePoolVolume("local", "custom", "backups")
					if err == nil {
						_ = op.Wait()
					}
				})

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

	reverter.Success()

	fmt.Println(tui.SuccessColor("MicroCloud is ready", true))

	return nil
}
