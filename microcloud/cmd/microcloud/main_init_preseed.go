package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	lxdAPI "github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/filter"
	"github.com/canonical/lxd/shared/units"
	cephTypes "github.com/canonical/microceph/microceph/api/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/mdns"
	"github.com/canonical/microcloud/microcloud/service"
)

// Preseed represents the structure of the supported preseed yaml.
type Preseed struct {
	LookupSubnet string        `yaml:"lookup_subnet"`
	Systems      []System      `yaml:"systems"`
	OVN          InitNetwork   `yaml:"ovn"`
	Storage      StorageFilter `yaml:"storage"`
}

// System represents the structure of the systems we expect to find in the preseed yaml.
type System struct {
	Name            string      `yaml:"name"`
	UplinkInterface string      `yaml:"ovn_uplink_interface"`
	Storage         InitStorage `yaml:"storage"`
}

// InitStorage separates the direct paths used for local and ceph disks.
type InitStorage struct {
	Local DirectStorage   `yaml:"local"`
	Ceph  []DirectStorage `yaml:"ceph"`
}

// DirectStorage is a direct path to a disk, to be used to override DiskFilter.
type DirectStorage struct {
	Path string `yaml:"path"`
	Wipe bool   `yaml:"wipe"`
}

// InitNetwork represents the structure of the network config in the preseed yaml.
type InitNetwork struct {
	IPv4Gateway string `yaml:"ipv4_gateway"`
	IPv4Range   string `yaml:"ipv4_range"`
	IPv6Gateway string `yaml:"ipv6_gateway"`
}

// StorageFilter separates the filters used for local and ceph disks.
type StorageFilter struct {
	Local []DiskFilter `yaml:"local"`
	Ceph  []DiskFilter `yaml:"ceph"`
}

// DiskFilter is the optional filter for finding disks according to their fields in api.ResourcesStorageDisk in LXD.
type DiskFilter struct {
	Find    string `yaml:"find"`
	FindMin int    `yaml:"find_min"`
	FindMax int    `yaml:"find_max"`
	Wipe    bool   `yaml:"wipe"`
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

// RunPreseed initializes MicroCloud from a preseed yaml filepath input.
func (c *CmdControl) RunPreseed(cmd *cobra.Command, preseed string, init bool) error {
	fileBytes, err := os.ReadFile(preseed)
	if err != nil {
		return fmt.Errorf("Failed to read preseed file %q: %w", preseed, err)
	}

	config := Preseed{}
	err = yaml.Unmarshal(fileBytes, &config)
	if err != nil {
		return fmt.Errorf("Failed to parse the preseed yaml: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	err = config.validate(hostname, init)
	if err != nil {
		return err
	}

	ip, _, err := net.ParseCIDR(config.LookupSubnet)
	if err != nil {
		return err
	}

	// Build the service handler.
	services := []types.ServiceType{types.MicroCloud, types.LXD}
	optionalServices := map[types.ServiceType]string{
		types.MicroCeph: api.MicroCephDir,
		types.MicroOVN:  api.MicroOVNDir,
	}

	services, err = c.askMissingServices(services, optionalServices, true)
	if err != nil {
		return err
	}

	s, err := service.NewHandler(hostname, ip.String(), c.FlagMicroCloudDir, c.FlagLogDebug, c.FlagLogVerbose, services...)
	if err != nil {
		return err
	}

	systems, err := config.Parse(s, init)
	if err != nil {
		return err
	}

	if !init {
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

	return setupCluster(s, systems)
}

// validate validates the unmarshaled preseed input.
func (p *Preseed) validate(name string, bootstrap bool) error {
	uplinkCount := 0
	directCephCount := 0
	directLocalCount := 0
	localInit := false

	if len(p.Systems) < 1 {
		return fmt.Errorf("No systems given")
	}

	if bootstrap && len(p.Systems) < 2 {
		return fmt.Errorf("At least 2 systems are required to set up MicroCloud")
	}

	for _, system := range p.Systems {
		if system.Name == "" {
			return fmt.Errorf("Missing system name")
		}

		if system.Name == name {
			localInit = true
		}

		if system.UplinkInterface != "" {
			uplinkCount++
		}

		if len(system.Storage.Ceph) > 0 {
			directCephCount++
		}

		if system.Storage.Local.Path != "" {
			directLocalCount++
		}
	}

	if bootstrap && !localInit {
		return fmt.Errorf("Local MicroCloud must be included in the list of systems when initializing")
	}

	if !bootstrap && localInit {
		return fmt.Errorf("Local MicroCloud must not be included in the list of systems when adding new members")
	}

	containsUplinks := false
	containsLocalStorage := false
	containsCephStorage := false
	containsUplinks = uplinkCount > 0
	if containsUplinks && uplinkCount < len(p.Systems) {
		return fmt.Errorf("Some systems are missing an uplink interface")
	}

	containsCephStorage = directCephCount > 0
	if containsCephStorage && directCephCount < len(p.Systems) && len(p.Storage.Ceph) == 0 {
		return fmt.Errorf("Some systems are missing ceph storage disks")
	}

	containsLocalStorage = directLocalCount > 0
	if containsLocalStorage && directLocalCount < len(p.Systems) && len(p.Storage.Local) == 0 {
		return fmt.Errorf("Some systems are missing local storage disks")
	}

	if containsCephStorage || len(p.Storage.Ceph) > 0 {
		if bootstrap && (len(p.Systems)) < 3 {
			return fmt.Errorf("At least 3 systems are required to configure distributed storage")
		}
	}

	_, _, err := net.ParseCIDR(p.LookupSubnet)
	if err != nil {
		return err
	}

	if (InitNetwork{} != p.OVN) && !containsUplinks {
		return fmt.Errorf("OVN uplink configuration found, but no uplink interfaces selected")
	}

	usingOVN := p.OVN.IPv4Gateway != "" || p.OVN.IPv6Gateway != "" || containsUplinks
	if bootstrap && usingOVN && len(p.Systems) < 3 {
		return fmt.Errorf("At least 3 systems are required to configure distributed networking")
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
			// If we have selected disks directly, we shouldn't need to validate that the filter matches 3 systems.
			numDirect := 0
			for _, s := range p.Systems {
				if len(s.Storage.Ceph) > 0 {
					numDirect++
				}
			}
			if filter.FindMax < filter.FindMin || (bootstrap && filter.FindMax+numDirect < 3) {
				return fmt.Errorf("Invalid remote storage filter constraints find_max (%d) must be at least 3 and larger than find_min (%d)", filter.FindMax, filter.FindMin)
			}
		}
	}

	for _, filter := range p.Storage.Local {
		if filter.Find == "" {
			return fmt.Errorf("Received empty local disk filter")
		}

		if filter.FindMax > 0 {
			if filter.FindMax < filter.FindMin {
				return fmt.Errorf("Invalid local storage filter constraints find_max (%d) larger than find_min (%d)", filter.FindMax, filter.FindMin)
			}
		}
	}

	return nil
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
func (p *Preseed) Parse(s *service.Handler, bootstrap bool) (map[string]InitSystem, error) {
	systems := make(map[string]InitSystem, len(p.Systems))
	if bootstrap {
		systems[s.Name] = InitSystem{ServerInfo: mdns.ServerInfo{Name: s.Name}}
	}

	expectedSystems := make([]string, 0, len(p.Systems))
	for _, system := range p.Systems {
		if system.Name == s.Name {
			continue
		}

		expectedSystems = append(expectedSystems, system.Name)
	}

	// Lookup peers until expected systems are found.
	_, lookupSubnet, err := net.ParseCIDR(p.LookupSubnet)
	if err != nil {
		return nil, err
	}

	err = lookupPeers(s, true, lookupSubnet, expectedSystems, systems)
	if err != nil {
		return nil, err
	}

	for name, system := range systems {
		system.MicroCephDisks = []cephTypes.DisksPost{}
		system.TargetStoragePools = []lxdAPI.StoragePoolsPost{}
		system.StoragePools = []lxdAPI.StoragePoolsPost{}
		system.JoinConfig = []lxdAPI.ClusterMemberConfigKey{}

		systems[name] = system
	}

	lxd := s.Services[types.LXD].(*service.LXDService)
	ifaceByPeer := map[string]string{}
	for _, cfg := range p.Systems {
		if cfg.UplinkInterface != "" {
			ifaceByPeer[cfg.Name] = cfg.UplinkInterface
		}
	}

	// If we gave OVN config, implicitly assume we want interfaces and pick the first one.
	usingOVN := p.OVN.IPv4Gateway != "" || p.OVN.IPv6Gateway != ""
	if usingOVN {
		// Only select systems not explicitly picked above.
		infos := make([]mdns.ServerInfo, 0, len(systems))
		for peer, system := range systems {
			if ifaceByPeer[peer] == "" {
				infos = append(infos, system.ServerInfo)
			}
		}

		networks, err := lxd.GetUplinkInterfaces(context.Background(), bootstrap, infos)
		if err != nil {
			return nil, err
		}

		for peer, nets := range networks {
			if len(nets) > 0 {
				ifaceByPeer[peer] = nets[0].Name
			}
		}
	}

	if bootstrap && len(ifaceByPeer) < 3 {
		return nil, fmt.Errorf("Failed to find at least 3 interfaces on 3 machines for OVN configuration")
	}

	for peer, iface := range ifaceByPeer {
		system := systems[peer]
		if bootstrap {
			system.TargetNetworks = append(system.TargetNetworks, lxd.DefaultPendingOVNNetwork(iface))
			if s.Name == peer {
				uplink, ovn := lxd.DefaultOVNNetwork(p.OVN.IPv4Gateway, p.OVN.IPv4Range, p.OVN.IPv6Gateway)
				system.Networks = append(system.Networks, uplink, ovn)
			}
		} else {
			system.JoinConfig = append(system.JoinConfig, lxd.DefaultOVNNetworkJoinConfig(iface))
		}

		systems[peer] = system
	}

	// Setup FAN network if OVN not available.
	if len(ifaceByPeer) == 0 {
		for peer, system := range systems {
			if bootstrap {
				system.TargetNetworks = append(system.TargetNetworks, lxd.DefaultPendingFanNetwork())
				if s.Name == peer {
					final, err := lxd.DefaultFanNetwork()
					if err != nil {
						return nil, err
					}

					system.Networks = append(system.Networks, final)
				}
			}

			systems[peer] = system
		}
	}

	directCephMatches := map[string]int{}
	directZFSMatches := map[string]int{}
	for peer, system := range systems {
		directLocal := DirectStorage{}
		directCeph := []DirectStorage{}
		for _, sys := range p.Systems {
			if sys.Name == peer {
				directLocal = sys.Storage.Local
				directCeph = sys.Storage.Ceph
			}
		}

		// Setup directly specified disks for ZFS pool.
		if directLocal.Path != "" {
			if bootstrap {
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
			system.MicroCephDisks = append(system.MicroCephDisks, cephTypes.DisksPost{Path: disk.Path, Wipe: disk.Wipe})
		}

		// Setup ceph pool for disks specified to MicroCeph.
		if len(system.MicroCephDisks) > 0 {
			if bootstrap {
				system.TargetStoragePools = append(system.TargetStoragePools, lxd.DefaultPendingCephStoragePool())

				if s.Name == peer {
					system.StoragePools = append(system.StoragePools, lxd.DefaultCephStoragePool())
				}
			} else {
				system.JoinConfig = append(system.JoinConfig, lxd.DefaultCephStoragePoolJoinConfig())
			}

			directCephMatches[peer] = directCephMatches[peer] + 1
		}

		systems[peer] = system
	}

	allResources := map[string]*lxdAPI.Resources{}
	for peer, system := range systems {
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

		// Fetch system resources from LXD to find disks if we haven't directly set up disks.
		allResources[peer], err = s.Services[types.LXD].(*service.LXDService).GetResources(context.Background(), peer, system.ServerInfo.Address, system.ServerInfo.AuthSecret)
		if err != nil {
			return nil, fmt.Errorf("Failed to get system resources of peer %q: %w", peer, err)
		}
	}

	cephMatches := map[string]int{}
	zfsMatches := map[string]int{}
	cephMachines := map[string]bool{}
	zfsMachines := map[string]bool{}
	for peer, r := range allResources {
		system := systems[peer]

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
				system.MicroCephDisks = append(system.MicroCephDisks, cephTypes.DisksPost{Path: parseDiskPath(disk), Wipe: filter.Wipe})
				// There should only be one ceph pool per system.
				if !addedCephPool {
					if bootstrap {
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

		for _, filter := range p.Storage.Local {
			// No need to check filters anymore if each machine has a disk.
			if len(zfsMachines) == len(systems) {
				break
			}

			matched, err := filter.Match(disks)
			if err != nil {
				return nil, fmt.Errorf("Failed to apply filter for local disks: %w", err)
			}

			if len(matched) > 0 {
				zfsMachines[peer] = true
				if bootstrap {
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

		systems[peer] = system
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

	if bootstrap && len(cephMachines)+len(directCephMatches) > 0 && len(cephMachines)+len(directCephMatches) < 3 {
		return nil, fmt.Errorf("Failed to find at least 3 disks on 3 machines for MicroCeph configuration")
	}

	if bootstrap && len(zfsMachines)+len(directZFSMatches) > 0 && len(zfsMachines)+len(directZFSMatches) < len(systems) {
		return nil, fmt.Errorf("Failed to find at least 1 disk on each machine for local storage pool configuration")
	}

	return systems, nil
}
