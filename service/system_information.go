package service

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/canonical/lxd/shared/api"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/mdns"
)

// SystemInformation represents all information MicroCloud needs from a system in order to set it up as part of the MicroCloud.
type SystemInformation struct {
	// ExistingServices is a map of cluster members for each service currently installed on the system.
	ExistingServices map[types.ServiceType]map[string]string

	// ClusterName is the name of the system in MicroCloud.
	ClusterName string

	// ClusterAddress is the default cluster address used for MicroCloud.
	ClusterAddress string

	// AvailableDisks is the list of disks available for use on the system.
	AvailableDisks map[string]api.ResourcesStorageDisk

	// AvailableUplinkInterfaces is the list of networks that can be used for the OVN uplink network.
	AvailableUplinkInterfaces map[string]api.Network

	// AvailableCephInterfaces is the list of networks that can be used for the Ceph cluster network.
	AvailableCephInterfaces map[string]DedicatedInterface

	// AvailableOVNInterfaces is the list of networks that can be used for an OVN underlay network.
	AvailableOVNInterfaces map[string]DedicatedInterface

	// LXDLocalConfig is the local configuration of LXD on this system.
	LXDLocalConfig map[string]any

	// LXDConfig is the cluster configuration of LXD on this system.
	LXDConfig map[string]any

	// CephConfig is the MicroCeph configuration on this system.
	CephConfig map[string]string

	// existingLocalPool is the current storage pool named "local" on this system.
	existingLocalPool *api.StoragePool

	// existingRemotePool is the current storage pool named "remote" on this system.
	existingRemotePool *api.StoragePool

	// existingRemoteFSPool is the current storage pool named "remote-fs" on this system.
	existingRemoteFSPool *api.StoragePool

	// existingFanNetwork is the current network named "lxdfan0" on this system.
	existingFanNetwork *api.Network

	// existingOVNNetwork is the current network named "default" on this system.
	existingOVNNetwork *api.Network

	// existingUplinkNetwork is the current network named "UPLINK" on this system.
	existingUplinkNetwork *api.Network
}

// CollectSystemInformation fetches the current cluster information of the system specified by the connection info.
func (sh *Handler) CollectSystemInformation(ctx context.Context, connectInfo mdns.ServerInfo) (*SystemInformation, error) {
	if connectInfo.Name == "" || connectInfo.Address == "" {
		return nil, fmt.Errorf("Connection information is incomplete")
	}

	localSystem := sh.Name == connectInfo.Name

	s := &SystemInformation{
		ExistingServices:          map[types.ServiceType]map[string]string{},
		ClusterName:               connectInfo.Name,
		ClusterAddress:            connectInfo.Address,
		AvailableDisks:            map[string]api.ResourcesStorageDisk{},
		AvailableUplinkInterfaces: map[string]api.Network{},
		AvailableCephInterfaces:   map[string]DedicatedInterface{},
		AvailableOVNInterfaces:    map[string]DedicatedInterface{},
	}

	var err error
	s.ExistingServices, err = sh.GetExistingClusters(ctx, connectInfo)
	if err != nil {
		return nil, fmt.Errorf("Failed to check for existing clusters on %q: %w", s.ClusterName, err)
	}

	var allResources *api.Resources
	lxd := sh.Services[types.LXD].(*LXDService)
	if localSystem {
		allResources, err = lxd.GetResources(ctx, s.ClusterName, "", nil)
	} else {
		allResources, err = lxd.GetResources(ctx, s.ClusterName, s.ClusterAddress, connectInfo.Certificate)
	}

	if err != nil {
		return nil, fmt.Errorf("Failed to get system resources of peer %q: %w", s.ClusterName, err)
	}

	if allResources != nil {
		for _, disk := range allResources.Storage.Disks {
			if len(disk.Partitions) == 0 {
				s.AvailableDisks[disk.ID] = disk
			}
		}
	}

	var allNets []api.Network
	uplinkInterfaces, dedicatedInterfaces, allNets, err := lxd.GetNetworkInterfaces(ctx, s.ClusterName, s.ClusterAddress, connectInfo.Certificate)
	if err != nil {
		return nil, fmt.Errorf("Failed to get network interfaces on %q: %w", s.ClusterName, err)
	}

	s.AvailableUplinkInterfaces = uplinkInterfaces
	s.AvailableCephInterfaces = dedicatedInterfaces
	s.AvailableOVNInterfaces = dedicatedInterfaces

	for _, network := range allNets {
		if network.Name == DefaultFANNetwork {
			s.existingFanNetwork = &network
			continue
		}

		if network.Name == DefaultOVNNetwork {
			s.existingOVNNetwork = &network
			continue
		}

		if network.Name == DefaultUplinkNetwork {
			s.existingUplinkNetwork = &network
			continue
		}
	}

	pools, err := lxd.GetStoragePools(ctx, s.ClusterName, s.ClusterAddress, connectInfo.Certificate)
	if err != nil {
		return nil, fmt.Errorf("Failed to get storage pools on %q: %w", s.ClusterName, err)
	}

	pool, ok := pools[DefaultZFSPool]
	if ok {
		poolCopy := pool
		s.existingLocalPool = &poolCopy
	}

	pool, ok = pools[DefaultCephPool]
	if ok {
		poolCopy := pool
		s.existingRemotePool = &poolCopy
	}

	pool, ok = pools[DefaultCephFSPool]
	if ok {
		poolCopy := pool
		s.existingRemoteFSPool = &poolCopy
	}

	if len(s.ExistingServices[types.MicroCeph]) > 0 {
		microceph := sh.Services[types.MicroCeph].(*CephService)

		if localSystem {
			s.CephConfig, err = microceph.ClusterConfig(ctx, "", nil)
		} else {
			s.CephConfig, err = microceph.ClusterConfig(ctx, s.ClusterAddress, connectInfo.Certificate)
		}

		if err != nil && !api.StatusErrorCheck(err, http.StatusServiceUnavailable) {
			return nil, fmt.Errorf("Failed to get Ceph configuration on %q: %w", s.ClusterName, err)
		}
	}

	if localSystem {
		s.LXDLocalConfig, s.LXDConfig, err = lxd.GetConfig(ctx, s.ServiceClustered(types.LXD), s.ClusterName, "", nil)
	} else {
		s.LXDLocalConfig, s.LXDConfig, err = lxd.GetConfig(ctx, s.ServiceClustered(types.LXD), s.ClusterName, s.ClusterAddress, connectInfo.Certificate)
	}

	if err != nil {
		return nil, fmt.Errorf("Failed to get LXD configuration on %q: %w", s.ClusterName, err)
	}

	return s, nil
}

// GetExistingClusters checks against the services reachable by the specified ServerInfo,
// and returns a map of cluster members for each service supported by the Handler.
// If a service is not clustered, its map will be nil.
func (sh *Handler) GetExistingClusters(ctx context.Context, connectInfo mdns.ServerInfo) (map[types.ServiceType]map[string]string, error) {
	localSystem := sh.Name == connectInfo.Name
	var err error
	existingServices := map[types.ServiceType]map[string]string{}
	for service := range sh.Services {
		var existingCluster map[string]string
		if localSystem {
			existingCluster, err = sh.Services[service].ClusterMembers(ctx)
		} else {
			existingCluster, err = sh.Services[service].RemoteClusterMembers(ctx, connectInfo.Certificate, connectInfo.Address)
		}

		if err != nil && !api.StatusErrorCheck(err, http.StatusServiceUnavailable) {
			return nil, fmt.Errorf("Failed to reach %s on system %q: %w", service, connectInfo.Name, err)
		}

		// If a service isn't clustered, this loop will be skipped.

		for k, v := range existingCluster {
			if existingServices[service] == nil {
				existingServices[service] = map[string]string{}
			}

			host, _, err := net.SplitHostPort(v)
			if err != nil {
				return nil, err
			}

			existingServices[service][k] = host
		}
	}

	return existingServices, nil
}

// SupportsLocalPool checks if the SystemInformation supports a MicroCloud configured local storage pool.
// Additionally returns whether such a pool already exists.
func (s *SystemInformation) SupportsLocalPool() (hasPool bool, supportsPool bool) {
	if s.existingLocalPool == nil {
		return false, true
	}

	if s.existingLocalPool.Driver == "zfs" && s.existingLocalPool.Status == "Created" {
		return true, true
	}

	return true, false
}

// SupportsRemotePool checks if the SystemInformation supports a MicroCloud configured remote storage pool.
// Additionally returns whether such a pool already exists.
func (s *SystemInformation) SupportsRemotePool() (hasPool bool, supportsPool bool) {
	if s.existingRemotePool == nil {
		return false, true
	}

	if s.existingRemotePool.Driver == "ceph" && s.existingRemotePool.Status == "Created" {
		return true, true
	}

	return true, false
}

// SupportsRemoteFSPool checks if the SystemInformation supports a MicroCloud configured remote-fs storage pool.
// Additionally returns whether such a pool already exists.
func (s *SystemInformation) SupportsRemoteFSPool() (hasPool bool, supportsPool bool) {
	if s.existingRemoteFSPool == nil {
		return false, true
	}

	if s.existingRemoteFSPool.Driver == "cephfs" && s.existingRemoteFSPool.Status == "Created" {
		return true, true
	}

	return true, false
}

// SupportsOVNNetwork checks if the SystemInformation supports MicroCloud configured default and UPLINK networks.
// Additionally returns whether such networks already exist.
func (s *SystemInformation) SupportsOVNNetwork() (hasNet bool, supportsNet bool) {
	if s.existingOVNNetwork == nil && s.existingUplinkNetwork == nil {
		return false, true
	}

	if s.existingOVNNetwork.Type == "ovn" && s.existingOVNNetwork.Status == "Created" && s.existingUplinkNetwork.Type == "physical" && s.existingUplinkNetwork.Status == "Created" {
		return true, true
	}

	return true, false
}

// SupportsFANNetwork checks if the SystemInformation supports a MicroCloud configured lxdfan0 network.
// Additionally returns whether such a network already exists.
// If checkUsable is set, it will also check /proc/net/route to see if an interface that can support the FAN network is present.
func (s *SystemInformation) SupportsFANNetwork(checkUsable bool) (hasNet bool, supportsNet bool, err error) {
	if s.existingFanNetwork == nil {
		if checkUsable {
			available, _, err := FanNetworkUsable()
			if err != nil {
				return false, false, err
			}

			return false, available, nil
		}

		return false, true, nil
	}

	if s.existingFanNetwork.Type == "bridge" && s.existingFanNetwork.Status == "Created" {
		return true, true, nil
	}

	return true, false, nil
}

// ServiceClustered returns whether or not a particular service is already clustered
// by checking if there are any cluster members in-memory.
func (s *SystemInformation) ServiceClustered(service types.ServiceType) bool {
	return len(s.ExistingServices[service]) > 0
}

// ClustersConflict compares the cluster members reported by each system in the list of systems, for each given service.
// If two distinct clusters exist for any service, this function returns true, with the name of the service.
func ClustersConflict(systems map[string]SystemInformation, services []types.ServiceType) (bool, types.ServiceType) {
	firstEncounteredClusters := map[types.ServiceType]map[string]string{}
	for _, info := range systems {
		for _, service := range services {
			// If a service is not clustered, it cannot conflict.
			if !info.ServiceClustered(service) {
				continue
			}

			// Record the first encountered cluster for each service.
			cluster, encountered := firstEncounteredClusters[service]
			if !encountered {
				firstEncounteredClusters[service] = info.ExistingServices[service]

				continue
			}

			// Check if the first encountered cluster for this service is identical to each system's record.
			for name, addr := range info.ExistingServices[service] {
				if cluster[name] != addr {
					return true, service
				}
			}

			if len(cluster) != len(info.ExistingServices[service]) {
				return true, service
			}
		}
	}

	return false, ""
}
