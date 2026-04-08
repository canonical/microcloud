package service

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/canonical/lxd/shared/api"
)

const (
	// DefaultFANNetwork is the name of the default FAN network.
	DefaultFANNetwork = "lxdfan0"

	// DefaultUplinkNetwork is the name of the default OVN uplink network.
	DefaultUplinkNetwork = "UPLINK"

	// DefaultOVNNetwork is the name of the default OVN network.
	DefaultOVNNetwork = "default"

	// DefaultZFSPool is the name of the default ZFS storage pool.
	DefaultZFSPool = "local"

	// DefaultCephPool is the name of the default Ceph storage pool.
	DefaultCephPool = "remote"

	// DefaultCephFSPool is the name of the default CephFS storage pool.
	DefaultCephFSPool = "remote-fs"

	// DefaultCephFSOSDPool is the default OSD pool name used for the CephFS storage pool.
	DefaultCephFSOSDPool = "lxd_cephfs"

	// DefaultCephOSDPool is the default OSD pool name used for the Ceph storage pool.
	DefaultCephOSDPool = "lxd_remote"

	// DefaultCephFSDataOSDPool is the default OSD pool name used for the CephFS's underlying data pool.
	DefaultCephFSDataOSDPool = "lxd_cephfs_data"

	// DefaultCephFSMetaOSDPool is the default OSD pool name used for the CephFS's underlying metadata pool.
	DefaultCephFSMetaOSDPool = "lxd_cephfs_meta"

	// DefaultMgrOSDPool is the reserved .mgr OSD pool created by Ceph.
	DefaultMgrOSDPool = ".mgr"
)

// DefaultPendingFanNetwork returns the default Ubuntu Fan network configuration when
// creating a pending network on a specific cluster member target.
func (s LXDService) DefaultPendingFanNetwork() api.NetworksPost {
	return api.NetworksPost{Name: DefaultFANNetwork, Type: "bridge"}
}

// FanNetworkUsable checks if the current host is capable of using a Fan network.
// It actually checks if there is a default IPv4 gateway available.
func FanNetworkUsable() (available bool, ifaceName string, err error) {
	file, err := os.Open("/proc/net/route")
	if err != nil {
		return false, "", err
	}

	defer func() { _ = file.Close() }()

	scanner := bufio.NewReader(file)
	for {
		line, _, err := scanner.ReadLine()
		if err != nil {
			break
		}

		fields := strings.Fields(string(line))
		if len(fields) < 8 {
			break
		}

		if fields[1] == "00000000" && fields[7] == "00000000" {
			ifaceName = fields[0]
			break
		}
	}

	if ifaceName == "" {
		return false, "", nil // There is no default gateway for IPv4
	}

	return true, ifaceName, nil
}

// DefaultFanNetwork returns the default Ubuntu Fan network configuration when
// creating the finalized network.
func (s LXDService) DefaultFanNetwork() (api.NetworksPost, error) {
	underlay, _, err := s.defaultGatewaySubnetV4()
	if err != nil {
		return api.NetworksPost{}, fmt.Errorf("Could not determine Fan overlay subnet: %w", err)
	}

	underlaySize, _ := underlay.Mask.Size()
	if underlaySize != 16 && underlaySize != 24 {
		// Override to /16 as that will almost always lead to working Fan network.
		underlay.Mask = net.CIDRMask(16, 32)
		underlay.IP = underlay.IP.Mask(underlay.Mask)
	}

	return api.NetworksPost{
		NetworkPut: api.NetworkPut{
			Config: map[string]string{
				"bridge.mode":         "fan",
				"fan.underlay_subnet": underlay.String(),
			},
			Description: "Default Ubuntu fan powered bridge",
		},
		Name: DefaultFANNetwork,
		Type: "bridge",
	}, nil
}

// DefaultPendingOVNNetwork returns the default OVN uplink network configuration when
// creating a pending network on a specific cluster member target.
func (s LXDService) DefaultPendingOVNNetwork(parent string) api.NetworksPost {
	return api.NetworksPost{
		NetworkPut: api.NetworkPut{Config: map[string]string{"parent": parent}},
		Name:       DefaultUplinkNetwork,
		Type:       "physical",
	}
}

// DefaultOVNNetworkJoinConfig returns the default OVN uplink network configuration when
// joining an existing cluster.
func (s LXDService) DefaultOVNNetworkJoinConfig(parent string) api.ClusterMemberConfigKey {
	return api.ClusterMemberConfigKey{
		Entity: "network",
		Name:   DefaultUplinkNetwork,
		Key:    "parent",
		Value:  parent,
	}
}

// DefaultOVNNetwork returns the default OVN network configuration when
// creating the finalized network.
// Returns both the finalized uplink configuration as the first argument,
// and the default OVN network configuration as the second argument.
func (s LXDService) DefaultOVNNetwork(ipv4Gateway string, ipv4Range string, ipv6Gateway string, dnsServers string) (api.NetworksPost, api.NetworksPost) {
	finalUplinkCfg := api.NetworksPost{
		NetworkPut: api.NetworkPut{
			Config:      map[string]string{},
			Description: "Uplink for OVN networks"},
		Name: DefaultUplinkNetwork,
		Type: "physical",
	}

	if ipv4Gateway != "" && ipv4Range != "" {
		finalUplinkCfg.Config["ipv4.gateway"] = ipv4Gateway
		finalUplinkCfg.Config["ipv4.ovn.ranges"] = ipv4Range
	}

	if ipv6Gateway != "" {
		finalUplinkCfg.Config["ipv6.gateway"] = ipv6Gateway
	}

	if dnsServers != "" {
		finalUplinkCfg.Config["dns.nameservers"] = dnsServers
	}

	ovnNetwork := api.NetworksPost{
		NetworkPut: api.NetworkPut{Config: map[string]string{"network": DefaultUplinkNetwork}, Description: "Default OVN network"},
		Name:       DefaultOVNNetwork,
		Type:       "ovn",
	}

	return finalUplinkCfg, ovnNetwork
}

// DefaultPendingZFSStoragePool returns the default local storage configuration when
// creating a pending pool on a specific cluster member target.
func (s LXDService) DefaultPendingZFSStoragePool(wipe bool, path string) api.StoragePoolsPost {
	cfg := map[string]string{"source": path}
	if wipe {
		cfg["source.wipe"] = strconv.FormatBool(wipe)
	}

	return api.StoragePoolsPost{
		Name:   DefaultZFSPool,
		Driver: "zfs",
		StoragePoolPut: api.StoragePoolPut{
			Config:      cfg,
			Description: "Local storage on ZFS",
		},
	}
}

// DefaultZFSStoragePool returns the default local storage configuration when
// creating the finalized pool.
func (s LXDService) DefaultZFSStoragePool() api.StoragePoolsPost {
	return api.StoragePoolsPost{
		Name:   DefaultZFSPool,
		Driver: "zfs",
		StoragePoolPut: api.StoragePoolPut{
			Description: "Local storage on ZFS",
		},
	}
}

// DefaultZFSStoragePoolJoinConfig returns the default local storage configuration when
// joining an existing cluster.
func (s LXDService) DefaultZFSStoragePoolJoinConfig(wipe bool, path string) []api.ClusterMemberConfigKey {
	wipeDisk := api.ClusterMemberConfigKey{
		Entity: "storage-pool",
		Name:   DefaultZFSPool,
		Key:    "source.wipe",
		Value:  "true",
	}

	sourceTemplate := api.ClusterMemberConfigKey{
		Entity: "storage-pool",
		Name:   DefaultZFSPool,
		Key:    "source",
	}

	sourceTemplate.Value = path
	joinConfig := []api.ClusterMemberConfigKey{sourceTemplate}
	if wipe {
		joinConfig = append(joinConfig, wipeDisk)
	}

	return joinConfig
}

// DefaultPendingCephStoragePool returns the default remote storage configuration when
// creating a pending pool on a specific cluster member target.
// If extra source config is not required by the used version of LXD, only the pool definition is returned.
func (s LXDService) DefaultPendingCephStoragePool() (*api.StoragePoolsPost, error) {
	req := &api.StoragePoolsPost{
		Name:   DefaultCephPool,
		Driver: "ceph",
	}

	hasStorageRemoteDropSource, err := s.HasExtension(context.Background(), s.name, s.address, nil, "storage_remote_drop_source")
	if err != nil {
		return nil, err
	}

	// Use the old approach of specifying the pool name in "source".
	if !hasStorageRemoteDropSource {
		req.StoragePoolPut = api.StoragePoolPut{
			Config: map[string]string{
				"source": DefaultCephOSDPool,
			},
		}
	}

	return req, nil
}

// DefaultCephStoragePool returns the default remote storage configuration when
// creating the finalized pool.
func (s LXDService) DefaultCephStoragePool() (*api.StoragePoolsPost, error) {
	req := api.StoragePoolsPost{
		Name:   DefaultCephPool,
		Driver: "ceph",
		StoragePoolPut: api.StoragePoolPut{
			Config: map[string]string{
				"ceph.rbd.du":        "false",
				"ceph.osd.pool_name": DefaultCephOSDPool,
			},
			Description: "Distributed storage on Ceph",
		},
	}

	hasExtension, err := s.HasExtension(context.Background(), s.name, s.address, nil, "storage_ceph_use_rbd_defaults")
	if err != nil {
		return nil, fmt.Errorf("Failed to check for storage_ceph_use_rbd_defaults extension: %w", err)
	}

	// Set the features in case LXD isn't using the Ceph cluster's defaults.
	if !hasExtension {
		req.Config["ceph.rbd.features"] = "layering,striping,exclusive-lock,object-map,fast-diff,deep-flatten"
	}

	return &req, nil
}

// DefaultPendingCephFSStoragePool returns the default cephfs storage configuration when
// creating a pending pool on a specific cluster member target.
// If extra source config is not required by the used version of LXD, only the pool definition is returned.
func (s LXDService) DefaultPendingCephFSStoragePool() (*api.StoragePoolsPost, error) {
	req := &api.StoragePoolsPost{
		Name:   DefaultCephFSPool,
		Driver: "cephfs",
	}

	hasStorageRemoteDropSource, err := s.HasExtension(context.Background(), s.name, s.address, nil, "storage_remote_drop_source")
	if err != nil {
		return nil, err
	}

	// Use the old approach of specifying the FS path.
	if !hasStorageRemoteDropSource {
		req.StoragePoolPut = api.StoragePoolPut{
			Config: map[string]string{
				"source": DefaultCephFSOSDPool,
			},
		}
	}

	return req, nil
}

// DefaultCephFSStoragePool returns the default cephfs storage configuration when
// creating the finalized pool.
func (s LXDService) DefaultCephFSStoragePool() api.StoragePoolsPost {
	return api.StoragePoolsPost{
		Name:   DefaultCephFSPool,
		Driver: "cephfs",
		StoragePoolPut: api.StoragePoolPut{
			Config: map[string]string{
				"cephfs.create_missing": "true",
				"cephfs.meta_pool":      DefaultCephFSMetaOSDPool,
				"cephfs.data_pool":      DefaultCephFSDataOSDPool,
				"cephfs.path":           DefaultCephFSOSDPool,
			},
			Description: "Distributed file-system storage using CephFS",
		},
	}
}

// DefaultCephFSStoragePoolJoinConfig returns the default cephfs storage configuration when joining an existing cluster.
// If not required by the used version of LXD, nil is returned.
func (s LXDService) DefaultCephFSStoragePoolJoinConfig() (*api.ClusterMemberConfigKey, error) {
	hasStorageRemoteDropSource, err := s.HasExtension(context.Background(), s.name, s.address, nil, "storage_remote_drop_source")
	if err != nil {
		return nil, err
	}

	// "source" is no longer valid when LXD has the storage_remote_drop_source extension.
	if !hasStorageRemoteDropSource {
		return &api.ClusterMemberConfigKey{
			Entity: "storage-pool",
			Name:   "remote-fs",
			Key:    "source",
			Value:  DefaultCephFSOSDPool,
		}, nil
	}

	// This version of LXD doesn't require any config when joining a CephFS pool.
	return nil, nil
}
