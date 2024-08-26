package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/canonical/lxd/shared"
	lxdAPI "github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/filter"
	"github.com/canonical/lxd/shared/units"
	"github.com/canonical/lxd/shared/validate"
	cephTypes "github.com/canonical/microceph/microceph/api/types"
	"github.com/canonical/microcluster/v2/microcluster"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/api/types"
	cloudClient "github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcloud/microcloud/mdns"
	"github.com/canonical/microcloud/microcloud/service"
)

// Preseed represents the structure of the supported preseed yaml.
type Preseed struct {
	LookupSubnet          string        `yaml:"lookup_subnet"`
	LookupTimeout         int64         `yaml:"lookup_timeout"`
	SessionPassphrase     string        `yaml:"session_passphrase"`
	SessionTimeout        int64         `yaml:"session_timeout"`
	ReuseExistingClusters bool          `yaml:"reuse_existing_clusters"`
	Initiator             string        `yaml:"initiator"`
	InitiatorAddress      string        `yaml:"initiator_address"`
	Systems               []System      `yaml:"systems"`
	OVN                   InitNetwork   `yaml:"ovn"`
	Ceph                  CephOptions   `yaml:"ceph"`
	Storage               StorageFilter `yaml:"storage"`
}

// System represents the structure of the systems we expect to find in the preseed yaml.
type System struct {
	Name            string      `yaml:"name"`
	Address         string      `yaml:"address"`
	UplinkInterface string      `yaml:"ovn_uplink_interface"`
	UnderlayIP      string      `yaml:"underlay_ip"`
	Storage         InitStorage `yaml:"storage"`
}

// InitStorage separates the direct paths used for local and ceph disks.
type InitStorage struct {
	Local DirectStorage   `yaml:"local"`
	Ceph  []DirectStorage `yaml:"ceph"`
}

// DirectStorage is a direct path to a disk, to be used to override DiskFilter.
type DirectStorage struct {
	Path    string `yaml:"path"`
	Wipe    bool   `yaml:"wipe"`
	Encrypt bool   `yaml:"encrypt"`
}

// InitNetwork represents the structure of the network config in the preseed yaml.
type InitNetwork struct {
	IPv4Gateway string `yaml:"ipv4_gateway"`
	IPv4Range   string `yaml:"ipv4_range"`
	IPv6Gateway string `yaml:"ipv6_gateway"`
	DNSServers  string `yaml:"dns_servers"`
}

// CephOptions represents the structure of the ceph options in the preseed yaml.
type CephOptions struct {
	InternalNetwork string `yaml:"internal_network"`
}

// StorageFilter separates the filters used for local and ceph disks.
type StorageFilter struct {
	CephFS bool         `yaml:"cephfs"`
	Local  []DiskFilter `yaml:"local"`
	Ceph   []DiskFilter `yaml:"ceph"`
}

// DiskFilter is the optional filter for finding disks according to their fields in api.ResourcesStorageDisk in LXD.
type DiskFilter struct {
	Find    string `yaml:"find"`
	FindMin int    `yaml:"find_min"`
	FindMax int    `yaml:"find_max"`
	Wipe    bool   `yaml:"wipe"`
	Encrypt bool   `yaml:"encrypt"`
}

// DiskOperatorSet is the set of operators supported for filtering disks.
func DiskOperatorSet() filter.OperatorSet {
	return filter.OperatorSet{
		And:       "&&",
		Or:        "||",
		Equals:    "==",
		NotEquals: "!=",
		Negate:    "!",
		Quote:     []string{"\"", "'"},

		GreaterThan:  ">",
		LessThan:     "<",
		GreaterEqual: ">=",
		LessEqual:    "<=",
	}
}

type cmdPreseed struct {
	common *CmdControl
}

func (c *cmdPreseed) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preseed",
		Short: "Initialize and extend a MicroCloud cluster unattended",
		RunE:  c.Run,
	}

	return cmd
}

func (c *cmdPreseed) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
	}

	cfg := initConfig{
		common:  c.common,
		systems: map[string]InitSystem{},
		state:   map[string]service.SystemInformation{},
	}

	return cfg.RunPreseed(cmd)
}

// RunPreseed initializes MicroCloud from a preseed yaml filepath input.
func (c *initConfig) RunPreseed(cmd *cobra.Command) error {
	c.autoSetup = true

	bytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("Failed to read from stdin: %w", err)
	}

	config := Preseed{}
	err = yaml.Unmarshal(bytes, &config)
	if err != nil {
		return fmt.Errorf("Failed to parse the preseed yaml: %w", err)
	}

	c.bootstrap = config.isBootstrap()

	c.lookupTimeout = DefaultLookupTimeout
	if config.LookupTimeout > 0 {
		c.lookupTimeout = time.Duration(config.LookupTimeout) * time.Second
	}

	c.sessionTimeout = DefaultSessionTimeout
	if config.SessionTimeout > 0 {
		c.sessionTimeout = time.Duration(config.SessionTimeout) * time.Second
	}

	cloudApp, err := microcluster.App(microcluster.Args{StateDir: c.common.FlagMicroCloudDir})
	if err != nil {
		return err
	}

	status, err := cloudApp.Status(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to get MicroCloud status: %w", err)
	}

	if status.Ready && c.bootstrap {
		return fmt.Errorf("MicroCloud is already initialized")
	}

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	err = config.validate(hostname, c.bootstrap)
	if err != nil {
		return err
	}

	var listenAddr string
	if status.Ready {
		// If the cluster is already bootstrapped use its address.
		listenAddr = status.Address.Addr().String()
	} else {
		// In case of bootstrap use the address from the preseed file.
		listenAddr, err = config.address(hostname)
		if err != nil {
			return err
		}
	}

	listenIP := net.ParseIP(listenAddr)
	if listenIP == nil {
		return fmt.Errorf("Invalid MicroCloud listen address %q", listenAddr)
	}

	// Build the service handler.
	services := []types.ServiceType{types.MicroCloud, types.LXD}
	optionalServices := map[types.ServiceType]string{
		types.MicroCeph: api.MicroCephDir,
		types.MicroOVN:  api.MicroOVNDir,
	}

	services, err = c.askMissingServices(services, optionalServices)
	if err != nil {
		return err
	}

	c.name = hostname
	c.address = listenIP.String()
	s, err := service.NewHandler(c.name, c.address, c.common.FlagMicroCloudDir, services...)
	if err != nil {
		return err
	}

	initiator := config.isInitiator(c.name, c.address)

	if !status.Ready && !c.bootstrap && initiator {
		return fmt.Errorf("MicroCloud isn't yet initialized and cannot be the initiator")
	}

	if status.Ready && !initiator {
		return fmt.Errorf("MicroCloud is already initialized and can only be the initiator")
	}

	systems, err := config.Parse(s, c)
	if err != nil {
		return err
	}

	// Exit in case of join.
	// Only the initiator has to continue.
	if systems == nil {
		return nil
	}

	if !c.bootstrap {
		peers, err := s.Services[types.MicroCloud].ClusterMembers(context.Background())
		if err != nil {
			return err
		}

		for name, system := range systems {
			if peers[name] != "" {
				return fmt.Errorf("System with name %q is already clustered", name)
			}

			for _, addr := range peers {
				if system.ServerInfo.Address == addr {
					return fmt.Errorf("System with address %q is already clustered", addr)
				}
			}
		}
	}

	err = c.validateSystems(s)
	if err != nil {
		return err
	}

	if !c.bootstrap {
		existingClusters, err := s.GetExistingClusters(context.Background(), mdns.ServerInfo{Name: c.name, Address: c.address})
		if err != nil {
			return err
		}

		for name, address := range existingClusters[types.MicroCloud] {
			_, ok := c.systems[name]
			if !ok {
				c.systems[name] = InitSystem{
					ServerInfo: mdns.ServerInfo{
						Name:     name,
						Address:  address,
						Services: services,
					},
				}
			}

			state, ok := c.state[name]
			if !ok {
				state.ExistingServices = existingClusters
				c.state[name] = state
			}
		}
	}

	return c.setupCluster(s)
}

// validate validates the unmarshaled preseed input.
func (p *Preseed) validate(name string, bootstrap bool) error {
	uplinkCount := 0
	underlayCount := 0
	directCephCount := 0
	directLocalCount := 0
	localInit := false

	if len(p.Systems) < 1 {
		return fmt.Errorf("No systems given")
	}

	if p.Initiator == "" && p.InitiatorAddress == "" {
		return fmt.Errorf("Missing initiator's name or address")
	}

	if p.Initiator != "" && p.InitiatorAddress != "" {
		return fmt.Errorf("Cannot provide both the initiator's name and address")
	}

	if p.InitiatorAddress != "" && p.LookupSubnet != "" {
		return fmt.Errorf("Cannot provide both the initiator's address and lookup subnet")
	}

	if len(p.Systems) > 1 && p.SessionPassphrase == "" {
		return fmt.Errorf("Missing session passphrase")
	}

	systemNames := make([]string, 0, len(p.Systems))
	for _, system := range p.Systems {
		if system.Name == "" {
			return fmt.Errorf("Missing system name")
		}

		if system.Address != "" && p.LookupSubnet != "" {
			return fmt.Errorf("Cannot provide both the address for system %q and the lookup subnet", system.Name)
		}

		if system.Address == "" && p.InitiatorAddress != "" {
			return fmt.Errorf("Missing address for system %q when the initiator's address is set", system.Name)
		}

		if system.Address != "" && p.InitiatorAddress == "" {
			return fmt.Errorf("Missing the initiator's address as system %q has an address", system.Name)
		}

		if system.Name == name {
			localInit = true
		}

		if system.UplinkInterface != "" {
			uplinkCount++
		}

		if system.UnderlayIP != "" {
			_, _, err := net.ParseCIDR(system.UnderlayIP)
			if err != nil {
				return fmt.Errorf("Invalid underlay IP: %w", err)
			}

			underlayCount++
		}

		if len(system.Storage.Ceph) > 0 {
			directCephCount++
		}

		if system.Storage.Local.Path != "" {
			directLocalCount++
		}

		if shared.ValueInSlice(system.Name, systemNames) {
			return fmt.Errorf("Duplicate system name %q", system.Name)
		}

		systemNames = append(systemNames, system.Name)
	}

	if !bootstrap && p.ReuseExistingClusters {
		return fmt.Errorf("Additional cluster members cannot be part of a pre-existing cluster")
	}

	if bootstrap && !localInit {
		return fmt.Errorf("Local MicroCloud must be included in the list of systems when initializing")
	}

	containsUplinks := false
	containsLocalStorage := false
	containsCephStorage := false
	containsUplinks = uplinkCount > 0
	if containsUplinks && uplinkCount < len(p.Systems) {
		return fmt.Errorf("Some systems are missing an uplink interface")
	}

	containsUnderlay := underlayCount > 0
	if containsUnderlay && underlayCount < len(p.Systems) {
		return fmt.Errorf("Some systems are missing an underlay interface")
	}

	containsLocalStorage = directLocalCount > 0
	if containsLocalStorage && directLocalCount < len(p.Systems) && len(p.Storage.Local) == 0 {
		return fmt.Errorf("Some systems are missing local storage disks")
	}

	containsCephStorage = directCephCount > 0 || len(p.Storage.Ceph) > 0
	usingCephInternalNetwork := p.Ceph.InternalNetwork != ""
	if !containsCephStorage && usingCephInternalNetwork {
		return fmt.Errorf("Cannot specify a Ceph internal network without Ceph storage disks")
	}

	if usingCephInternalNetwork {
		err := validate.IsNetwork(p.Ceph.InternalNetwork)
		if err != nil {
			return fmt.Errorf("Invalid Ceph internal network subnet: %v", err)
		}
	}

	if p.OVN.IPv4Gateway == "" && p.OVN.IPv4Range != "" {
		return fmt.Errorf("Cannot specify IPv4 range without IPv4 gateway")
	}

	if p.OVN.IPv4Gateway != "" {
		_, _, err := net.ParseCIDR(p.OVN.IPv4Gateway)
		if err != nil {
			return err
		}

		if p.OVN.IPv4Range == "" {
			return fmt.Errorf("Cannot specify IPv4 range without IPv4 gateway")
		}

		start, end, ok := strings.Cut(p.OVN.IPv4Range, "-")
		startIP := net.ParseIP(start)
		endIP := net.ParseIP(end)
		if !ok || startIP == nil || endIP == nil {
			return fmt.Errorf("Invalid IPv4 range (must be of the form <ip>-<ip>)")
		}
	}

	if p.OVN.IPv6Gateway != "" {
		_, _, err := net.ParseCIDR(p.OVN.IPv4Gateway)
		if err != nil {
			return err
		}
	}

	for _, filter := range p.Storage.Ceph {
		if filter.Find == "" {
			return fmt.Errorf("Received empty remote disk filter")
		}

		if filter.FindMax > 0 {
			if filter.FindMax < filter.FindMin {
				return fmt.Errorf("Invalid remote storage filter constraints find_max (%d) must be larger than find_min (%d)", filter.FindMax, filter.FindMin)
			}
		}

		// For distributed storage, the minimum match count must be defined so that we don't have a default configuration that can be non-HA.
		if filter.FindMin < 1 {
			return fmt.Errorf("Remote storage filter cannot be defined with find_min less than 1")
		}
	}

	for i, filter := range p.Storage.Local {
		if filter.Find == "" {
			return fmt.Errorf("Received empty local disk filter")
		}

		if filter.FindMax > 0 {
			if filter.FindMax < filter.FindMin {
				return fmt.Errorf("Invalid local storage filter constraints find_max (%d) larger than find_min (%d)", filter.FindMax, filter.FindMin)
			}
		}

		// For local storage, we can set a default minimum match count because we require at least 1 disk per system.
		if filter.FindMin == 0 {
			filter.FindMin = 1
			p.Storage.Local[i] = filter
		}
	}

	return nil
}

// isInitiator returns true if the current host is marked as being the initiator.
func (p *Preseed) isInitiator(name string, address string) bool {
	if name == p.Initiator && p.Initiator != "" {
		return true
	}

	if address == p.InitiatorAddress && p.InitiatorAddress != "" {
		return true
	}

	return false
}

// isBootstrap returns true if MicroCloud is in bootstrap mode.
// This is the case if either no initiator address is set
// or the initiator address is set to an address of a system
// in the current list of systems in the preseed file.
func (p *Preseed) isBootstrap() bool {
	for _, system := range p.Systems {
		if system.Name == p.Initiator {
			return true
		}

		if system.Address != "" && system.Address == p.InitiatorAddress {
			return true
		}
	}

	return false
}

// address either returns the address specified for the respective system
// or the first address found on the system within the provided lookup subnet.
func (p *Preseed) address(name string) (string, error) {
	for _, system := range p.Systems {
		if system.Name == name && system.Address != "" {
			return system.Address, nil
		}
	}

	_, lookupSubnet, err := net.ParseCIDR(p.LookupSubnet)
	if err != nil {
		return "", err
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}

		ip := addrInSubnet(addrs, *lookupSubnet)
		if ip != nil {
			return ip.String(), nil
		}
	}

	return "", fmt.Errorf("Failed to determine MicroCloud address within subnet %q", p.LookupSubnet)
}

// Match matches the devices to the given filter, and returns the result.
func (d *DiskFilter) Match(disks []lxdAPI.ResourcesStorageDisk) ([]lxdAPI.ResourcesStorageDisk, error) {
	if d.Find == "" {
		return nil, fmt.Errorf("Received empty filter")
	}

	clauses, err := filter.Parse(d.Find, DiskOperatorSet())
	if err != nil {
		return nil, err
	}

	clauses.ParseUint = func(c filter.Clause) (uint64, error) {
		if c.Field == "size" {
			bytes, err := units.ParseByteSizeString(c.Value)
			if err != nil {
				return 0, err
			}

			return uint64(bytes), nil
		}

		return strconv.ParseUint(c.Value, 10, 0)
	}

	matches := []lxdAPI.ResourcesStorageDisk{}
	for _, disk := range disks {
		match, err := filter.Match(disk, *clauses)
		if err != nil {
			return nil, err
		}

		if match {
			matches = append(matches, disk)
		}
	}

	return matches, nil
}

// Parse converts the preseed data into the appropriate set of InitSystem to use when setting up MicroCloud.
func (p *Preseed) Parse(s *service.Handler, c *initConfig) (map[string]InitSystem, error) {
	c.systems = make(map[string]InitSystem, len(p.Systems))
	if c.bootstrap {
		c.systems[s.Name] = InitSystem{ServerInfo: mdns.ServerInfo{Name: s.Name}}
	}

	expectedSystems := make([]string, 0, len(p.Systems))
	for _, system := range p.Systems {
		if system.Name == s.Name {
			continue
		}

		expectedSystems = append(expectedSystems, system.Name)
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("Failed to get network interfaces: %w", err)
	}

	for _, iface := range ifaces {
		addresses, err := iface.Addrs()
		if err != nil {
			return nil, fmt.Errorf("Failed to get addresses of interface %q: %w", iface.Name, err)
		}

		addressStrings := make([]string, 0, len(addresses))
		for _, address := range addresses {
			ipNet, ok := address.(*net.IPNet)
			if !ok {
				continue
			}

			addressStrings = append(addressStrings, ipNet.IP.String())
		}

		if shared.ValueInSlice(c.address, addressStrings) {
			c.lookupIface = &iface
			break
		}
	}

	if c.lookupIface == nil {
		return nil, fmt.Errorf("Failed to find lookup interface for address %q", c.address)
	}

	initiator := p.isInitiator(c.name, c.address)

	expectedServices := make([]types.ServiceType, len(s.Services))
	for _, v := range s.Services {
		expectedServices = append(expectedServices, v.Type())
	}

	if !initiator {
		err = c.runSession(context.Background(), s, types.SessionJoining, c.sessionTimeout, func(gw *cloudClient.WebsocketGateway) error {
			return c.joiningSession(gw, s, expectedServices, p.InitiatorAddress, p.SessionPassphrase)
		})
		return nil, err
	}

	if len(expectedSystems) > 0 {
		err = c.runSession(context.Background(), s, types.SessionInitiating, c.sessionTimeout, func(gw *cloudClient.WebsocketGateway) error {
			return c.initiatingSession(gw, s, expectedServices, p.SessionPassphrase, expectedSystems)
		})
		if err != nil {
			return nil, err
		}
	}

	for peer, system := range c.systems {
		existingClusters, err := s.GetExistingClusters(context.Background(), system.ServerInfo)
		if err != nil {
			return nil, err
		}

		for serviceType, cluster := range existingClusters {
			if len(cluster) > 0 {
				fmt.Printf("Existing %s cluster is incompatible with MicroCloud, skipping %s setup\n", serviceType, serviceType)

				delete(s.Services, serviceType)
			}
		}

		state := c.state[peer]
		state.ExistingServices = existingClusters
		c.state[peer] = state
	}

	for name, system := range c.systems {
		system.MicroCephDisks = []cephTypes.DisksPost{}
		system.TargetStoragePools = []lxdAPI.StoragePoolsPost{}
		system.StoragePools = []lxdAPI.StoragePoolsPost{}
		system.JoinConfig = []lxdAPI.ClusterMemberConfigKey{}

		c.systems[name] = system
	}

	lxd := s.Services[types.LXD].(*service.LXDService)
	ifaceByPeer := map[string]string{}
	ovnUnderlayNeeded := false
	for _, cfg := range p.Systems {
		if cfg.UplinkInterface != "" {
			ifaceByPeer[cfg.Name] = cfg.UplinkInterface
		}

		if cfg.UnderlayIP != "" {
			ovnUnderlayNeeded = true
		}
	}

	localInfo, err := s.CollectSystemInformation(context.Background(), mdns.ServerInfo{Name: c.name, Address: c.address})
	if err != nil {
		return nil, err
	}

	// If an uplink interface was explicitly chosen, we will try to set up an OVN network.
	explicitOVN := len(ifaceByPeer) > 0

	cephInterfaces := map[string]map[string]service.DedicatedInterface{}
	for _, system := range c.systems {
		cert := system.ServerInfo.Certificate
		uplinkIfaces, cephIfaces, _, err := lxd.GetNetworkInterfaces(context.Background(), system.ServerInfo.Name, system.ServerInfo.Address, cert)
		if err != nil {
			return nil, err
		}

		// Take the first alphabetical interface for each system's uplink network.
		if !explicitOVN {
			for k := range uplinkIfaces {
				currentIface := ifaceByPeer[system.ServerInfo.Name]
				if k < currentIface || currentIface == "" {
					ifaceByPeer[system.ServerInfo.Name] = k
				}
			}
		}

		for ifaceName, iface := range cephIfaces {
			if cephInterfaces[system.ServerInfo.Name] == nil {
				cephInterfaces[system.ServerInfo.Name] = map[string]service.DedicatedInterface{}
			}

			cephInterfaces[system.ServerInfo.Name][ifaceName] = iface
		}
	}

	// If we have specified any part of OVN config, implicitly assume we want to set it up.
	hasOVN, _ := localInfo.SupportsOVNNetwork()
	usingOVN := p.OVN.IPv4Gateway != "" || p.OVN.IPv6Gateway != "" || explicitOVN || hasOVN
	if usingOVN {
		for peer, iface := range ifaceByPeer {
			system := c.systems[peer]
			if c.bootstrap {
				system.TargetNetworks = append(system.TargetNetworks, lxd.DefaultPendingOVNNetwork(iface))
				if s.Name == peer {
					uplink, ovn := lxd.DefaultOVNNetwork(p.OVN.IPv4Gateway, p.OVN.IPv4Range, p.OVN.IPv6Gateway, p.OVN.DNSServers)
					system.Networks = append(system.Networks, uplink, ovn)
				}
			} else {
				system.JoinConfig = append(system.JoinConfig, lxd.DefaultOVNNetworkJoinConfig(iface))
			}

			c.systems[peer] = system
		}

		// Check the preseed underlay network configuration against the available ips.
		if ovnUnderlayNeeded {
			canOVNUnderlay := true
			for peer, system := range c.state {
				if len(system.AvailableOVNInterfaces) == 0 {
					fmt.Printf("Not enough interfaces available on %s to create an underlay network, skipping\n", peer)
					canOVNUnderlay = false
					break
				}
			}

			if canOVNUnderlay {
				// TODO: call `s.Services[types.MicroOVN].(*service.OVNService).SupportsFeature(context.Background(), "custom_encapsulation_ip")`
				// when MicroCloud will be updated with microcluster/v2
				underlays := make(map[string]string, len(p.Systems))
				for _, sys := range p.Systems {
					underlays[sys.Name] = sys.UnderlayIP
				}

				underlayCount := 0
				for _, sys := range p.Systems {
					for _, net := range c.state[sys.Name].AvailableOVNInterfaces {
						if len(net.Addresses) != 0 {
							for _, cidrAddr := range net.Addresses {
								if underlays[sys.Name] == cidrAddr {
									underlayCount = underlayCount + 1
									goto out
								}
							}
						}
					}

				out:
				}

				if underlayCount != len(p.Systems) {
					return nil, fmt.Errorf("Failed to find all underlay IPs on the network")
				}

				// Apply the underlay IPs to the systems.
				for peer, system := range c.systems {
					ip, _, err := net.ParseCIDR(underlays[peer])
					if err != nil {
						return nil, fmt.Errorf("Failed to parse underlay IP: %w", err)
					}

					system.OVNGeneveAddr = ip.String()
					c.systems[peer] = system
				}
			}
		}
	} else {
		// Check if FAN networking is usable.
		fanUsable, _, err := service.FanNetworkUsable()
		if err != nil {
			return nil, err
		}

		for peer, system := range c.systems {
			if c.bootstrap && fanUsable {
				system.TargetNetworks = append(system.TargetNetworks, lxd.DefaultPendingFanNetwork())
				if s.Name == peer {
					final, err := lxd.DefaultFanNetwork()
					if err != nil {
						return nil, err
					}

					system.Networks = append(system.Networks, final)
				}
			}

			c.systems[peer] = system
		}
	}

	directCephMatches := map[string]int{}
	directZFSMatches := map[string]int{}
	for peer, system := range c.systems {
		directLocal := DirectStorage{}
		directCeph := []DirectStorage{}
		for _, sys := range p.Systems {
			if sys.Name == peer {
				directLocal = sys.Storage.Local
				directCeph = sys.Storage.Ceph
			}

			for _, disk := range directCeph {
				_, err := os.Stat(disk.Path)
				if err != nil {
					return nil, fmt.Errorf("Failed to find specified disk path: %w", err)
				}
			}
		}

		// Setup directly specified disks for ZFS pool.
		if directLocal.Path != "" {
			if c.bootstrap {
				system.TargetStoragePools = append(system.TargetStoragePools, lxd.DefaultPendingZFSStoragePool(directLocal.Wipe, directLocal.Path))
				if s.Name == peer {
					system.StoragePools = append(system.StoragePools, lxd.DefaultZFSStoragePool())
				}
			} else {
				system.JoinConfig = append(system.JoinConfig, lxd.DefaultZFSStoragePoolJoinConfig(directLocal.Wipe, directLocal.Path)...)
			}

			directZFSMatches[peer] = directZFSMatches[peer] + 1
		}

		for _, disk := range directCeph {
			system.MicroCephDisks = append(
				system.MicroCephDisks,
				cephTypes.DisksPost{
					Path:    []string{disk.Path},
					Wipe:    disk.Wipe,
					Encrypt: disk.Encrypt,
				},
			)
		}

		// Setup ceph pool for disks specified to MicroCeph.
		if len(system.MicroCephDisks) > 0 {
			if c.bootstrap {
				system.TargetStoragePools = append(system.TargetStoragePools, lxd.DefaultPendingCephStoragePool())

				if s.Name == peer {
					system.StoragePools = append(system.StoragePools, lxd.DefaultCephStoragePool())
				}
			} else {
				system.JoinConfig = append(system.JoinConfig, lxd.DefaultCephStoragePoolJoinConfig())
			}

			directCephMatches[peer] = directCephMatches[peer] + 1
		}

		c.systems[peer] = system
	}

	allResources := map[string]*lxdAPI.Resources{}
	for peer, system := range c.systems {
		// Skip any systems that had direct configuration.
		if len(system.MicroCephDisks) > 0 || len(system.TargetStoragePools) > 0 || len(system.StoragePools) > 0 {
			continue
		}

		setupStorage := false
		for _, cfg := range system.JoinConfig {
			if cfg.Entity == "storage-pool" {
				setupStorage = true
				break
			}
		}

		if setupStorage {
			continue
		}

		cert := system.ServerInfo.Certificate

		// Fetch system resources from LXD to find disks if we haven't directly set up disks.
		allResources[peer], err = s.Services[types.LXD].(*service.LXDService).GetResources(context.Background(), peer, system.ServerInfo.Address, cert)
		if err != nil {
			return nil, fmt.Errorf("Failed to get system resources of peer %q: %w", peer, err)
		}
	}

	cephMatches := map[string]int{}
	zfsMatches := map[string]int{}
	cephMachines := map[string]bool{}
	zfsMachines := map[string]bool{}
	for peer, r := range allResources {
		system := c.systems[peer]

		disks := make([]lxdAPI.ResourcesStorageDisk, 0, len(r.Storage.Disks))
		for _, disk := range r.Storage.Disks {
			if len(disk.Partitions) == 0 {
				disks = append(disks, disk)
			}
		}

		addedCephPool := false
		for _, filter := range p.Storage.Ceph {
			matched, err := filter.Match(disks)
			if err != nil {
				return nil, fmt.Errorf("Failed to apply filter for ceph disks: %w", err)
			}

			for _, disk := range matched {
				system.MicroCephDisks = append(
					system.MicroCephDisks,
					cephTypes.DisksPost{
						Path:    []string{parseDiskPath(disk)},
						Wipe:    filter.Wipe,
						Encrypt: filter.Encrypt,
					},
				)
				// There should only be one ceph pool per system.
				if !addedCephPool {
					if c.bootstrap {
						system.TargetStoragePools = append(system.TargetStoragePools, lxd.DefaultPendingCephStoragePool())

						if s.Name == peer {
							system.StoragePools = append(system.StoragePools, lxd.DefaultCephStoragePool())
						}
					} else {
						system.JoinConfig = append(system.JoinConfig, lxd.DefaultCephStoragePoolJoinConfig())
					}

					addedCephPool = true
				}

				cephMatches[filter.Find] = cephMatches[filter.Find] + 1
			}

			// Remove any selected disks from the remaining available set.
			if len(matched) > 0 {
				cephMachines[peer] = true
				newDisks := []lxdAPI.ResourcesStorageDisk{}
				for _, disk := range disks {
					isMatch := false
					for _, match := range matched {
						if disk.ID == match.ID {
							isMatch = true
							break
						}
					}

					if !isMatch {
						newDisks = append(newDisks, disk)
					}
				}

				disks = newDisks
			}
		}

		if c.bootstrap {
			osdHosts := 0
			for _, system := range c.systems {
				if len(system.MicroCephDisks) > 0 {
					osdHosts++
				}
			}

			if osdHosts < RecommendedOSDHosts {
				fmt.Printf("Warning: OSD host count is less than %d. Distributed storage is not fault-tolerant\n", RecommendedOSDHosts)
			}
		}

		for _, filter := range p.Storage.Local {
			// No need to check filters anymore if each machine has a disk.
			if len(zfsMachines) == len(c.systems) {
				break
			}

			matched, err := filter.Match(disks)
			if err != nil {
				return nil, fmt.Errorf("Failed to apply filter for local disks: %w", err)
			}

			if len(matched) > 0 {
				zfsMachines[peer] = true
				if c.bootstrap {
					system.TargetStoragePools = append(system.TargetStoragePools, lxd.DefaultPendingZFSStoragePool(filter.Wipe, parseDiskPath(matched[0])))
					if s.Name == peer {
						system.StoragePools = append(system.StoragePools, lxd.DefaultZFSStoragePool())
					}
				} else {
					system.JoinConfig = append(system.JoinConfig, lxd.DefaultZFSStoragePoolJoinConfig(filter.Wipe, parseDiskPath(matched[0]))...)
				}

				zfsMatches[filter.Find] = zfsMatches[filter.Find] + 1
			}
		}

		c.systems[peer] = system
	}

	// Initialize Ceph network if specified.
	if c.bootstrap {
		var initializedMicroCephSystem *InitSystem
		for peer, system := range c.systems {
			if c.state[peer].ExistingServices[types.MicroCeph][peer] != "" {
				initializedMicroCephSystem = &system
				break
			}
		}

		var customTargetCephInternalNetwork string
		if initializedMicroCephSystem != nil {
			// If there is at least one initialized system with MicroCeph (we consider that more than one initialized MicroCeph systems are part of the same cluster),
			// we need to fetch its Ceph configuration to validate against this to-be-bootstrapped cluster.
			targetInternalCephNetwork, err := getTargetCephNetworks(s, initializedMicroCephSystem)
			if err != nil {
				return nil, err
			}

			if targetInternalCephNetwork.String() != c.lookupSubnet.String() {
				customTargetCephInternalNetwork = targetInternalCephNetwork.String()
			}
		}

		var internalCephNetwork string
		if customTargetCephInternalNetwork == "" {
			internalCephNetwork = p.Ceph.InternalNetwork
		} else {
			internalCephNetwork = customTargetCephInternalNetwork
		}

		if internalCephNetwork != "" {
			err = validateCephInterfacesForSubnet(lxd, c.systems, cephInterfaces, internalCephNetwork)
			if err != nil {
				return nil, err
			}

			bootstrapSystem := c.systems[s.Name]
			bootstrapSystem.MicroCephInternalNetworkSubnet = internalCephNetwork
			c.systems[s.Name] = bootstrapSystem
		}
	} else {
		localInternalCephNetwork, err := getTargetCephNetworks(s, nil)
		if err != nil {
			return nil, err
		}

		if localInternalCephNetwork.String() != "" && localInternalCephNetwork.String() != c.lookupSubnet.String() {
			err = validateCephInterfacesForSubnet(lxd, c.systems, cephInterfaces, localInternalCephNetwork.String())
			if err != nil {
				return nil, err
			}
		}
	}

	// Check that the filters matched the correct amount of disks.
	for _, filter := range p.Storage.Ceph {
		if cephMatches[filter.Find] < filter.FindMin {
			return nil, fmt.Errorf("Failed to find at least %d disks for filter %q", filter.FindMin, filter.Find)
		}

		if cephMatches[filter.Find] > filter.FindMax && filter.FindMax > 0 {
			return nil, fmt.Errorf("Found more than %d disks for filter %q", filter.FindMax, filter.Find)
		}
	}

	for _, filter := range p.Storage.Local {
		if zfsMatches[filter.Find] < filter.FindMin {
			return nil, fmt.Errorf("Failed to find at least %d disks for filter %q", filter.FindMin, filter.Find)
		}

		if zfsMatches[filter.Find] > filter.FindMax && filter.FindMax > 0 {
			return nil, fmt.Errorf("Found more than %d disks for filter %q", filter.FindMax, filter.Find)
		}
	}

	if c.bootstrap && len(zfsMachines)+len(directZFSMatches) > 0 && len(zfsMachines)+len(directZFSMatches) < len(c.systems) {
		return nil, fmt.Errorf("Failed to find at least 1 disk on each machine for local storage pool configuration")
	}

	hasCephFS, _ := localInfo.SupportsRemoteFSPool()
	if (len(cephMatches)+len(directCephMatches) > 0 && p.Storage.CephFS) || hasCephFS {
		for name, system := range c.systems {
			if c.bootstrap {
				system.TargetStoragePools = append(system.TargetStoragePools, lxd.DefaultPendingCephFSStoragePool())
				if s.Name == name {
					system.StoragePools = append(system.StoragePools, lxd.DefaultCephFSStoragePool())
				}
			} else {
				system.JoinConfig = append(system.JoinConfig, lxd.DefaultCephFSStoragePoolJoinConfig())
			}

			c.systems[name] = system
		}
	}

	return c.systems, nil
}

// Returns the first IP address assigned to iface that falls within lookupSubnet.
func addrInSubnet(addrs []net.Addr, lookupSubnet net.IPNet) net.IP {
	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			continue
		}

		if lookupSubnet.Contains(ip) {
			return ip
		}
	}

	return nil
}
