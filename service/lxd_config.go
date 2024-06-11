package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/canonical/lxd/shared/api"
)

// lxdMinVersion is the minimum version of LXD that fully supports all MicroCloud features.
const lxdMinVersion = "5.21"

// DefaultUplinkNetwork is the name of the default OVN uplink network.
const DefaultUplinkNetwork = "UPLINK"

// DefaultOVNNetwork is the name of the default OVN network.
const DefaultOVNNetwork = "default"

// DefaultFANNetwork is the name of the default FAN network.
const DefaultFANNetwork = "lxdfan0"

// DefaultZFSPool is the name of the default ZFS storage pool.
const DefaultZFSPool = "local"

// DefaultCephPool is the name of the default Ceph storage pool.
const DefaultCephPool = "remote"

// DefaultCephFSPool is the name of the default CephFS storage pool.
const DefaultCephFSPool = "remote-fs"

// DefaultPendingFanNetwork returns the default Ubuntu Fan network configuration when
// creating a pending network on a specific cluster member target.
func (s LXDService) DefaultPendingFanNetwork() api.NetworksPost {
	return api.NetworksPost{Name: DefaultFANNetwork, Type: "bridge"}
}

// DefaultFanNetwork returns the default Ubuntu Fan network configuration when
// creating the finalized network.
func (s LXDService) DefaultFanNetwork() (api.NetworksPost, error) {
	network := s.defaultFanNetwork()

	underlay, _, err := defaultGatewaySubnetV4()
	if err != nil {
		return api.NetworksPost{}, fmt.Errorf("Could not determine Fan overlay subnet: %w", err)
	}

	underlaySize, _ := underlay.Mask.Size()
	if underlaySize != 16 && underlaySize != 24 {
		// Override to /16 as that will almost always lead to working Fan network.
		underlay.Mask = net.CIDRMask(16, 32)
		underlay.IP = underlay.IP.Mask(underlay.Mask)
	}

	network.Config["fan.underlay_subnet"] = underlay.String()

	return network, nil
}

// defaultFanNetwork is the bare payload for the FAN network without the underlay subnet.
func (s LXDService) defaultFanNetwork() api.NetworksPost {
	return api.NetworksPost{
		NetworkPut: api.NetworkPut{
			Config: map[string]string{
				"bridge.mode": "fan",
			},
			Description: "Default Ubuntu fan powered bridge",
		},
		Name: DefaultFANNetwork,
		Type: "bridge",
	}
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
func (s LXDService) DefaultPendingCephStoragePool() api.StoragePoolsPost {
	return api.StoragePoolsPost{
		Name:   DefaultCephPool,
		Driver: "ceph",
		StoragePoolPut: api.StoragePoolPut{
			Config: map[string]string{
				"source": "lxd_remote",
			},
		},
	}
}

// DefaultCephStoragePool returns the default remote storage configuration when
// creating the finalized pool.
func (s LXDService) DefaultCephStoragePool() api.StoragePoolsPost {
	return api.StoragePoolsPost{
		Name:   DefaultCephPool,
		Driver: "ceph",
		StoragePoolPut: api.StoragePoolPut{
			Config: map[string]string{
				"ceph.rbd.du":       "false",
				"ceph.rbd.features": "layering,striping,exclusive-lock,object-map,fast-diff,deep-flatten",
			},
			Description: "Distributed storage on Ceph",
		},
	}
}

// DefaultCephStoragePoolJoinConfig returns the default remote storage configuration when
// joining an existing cluster.
func (s LXDService) DefaultCephStoragePoolJoinConfig() api.ClusterMemberConfigKey {
	return api.ClusterMemberConfigKey{
		Entity: "storage-pool",
		Name:   DefaultCephPool,
		Key:    "source",
		Value:  "lxd_remote",
	}
}

// DefaultPendingCephFSStoragePool returns the default cephfs storage configuration when
// creating a pending pool on a specific cluster member target.
func (s LXDService) DefaultPendingCephFSStoragePool() api.StoragePoolsPost {
	return api.StoragePoolsPost{
		Name:   DefaultCephFSPool,
		Driver: "cephfs",
		StoragePoolPut: api.StoragePoolPut{
			Config: map[string]string{
				"source": "lxd_cephfs",
			},
		},
	}
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
				"cephfs.meta_pool":      "lxd_cephfs_meta",
				"cephfs.data_pool":      "lxd_cephfs_data",
			},
			Description: "Distributed file-system storage using CephFS",
		},
	}
}

// DefaultCephFSStoragePoolJoinConfig returns the default cephfs storage configuration when
// joining an existing cluster.
func (s LXDService) DefaultCephFSStoragePoolJoinConfig() api.ClusterMemberConfigKey {
	return api.ClusterMemberConfigKey{
		Entity: "storage-pool",
		Name:   DefaultCephFSPool,
		Key:    "source",
		Value:  "lxd_cephfs",
	}
}

// SupportsPool checks whether LXD has the storage pools that matches what MicroCloud expects, or has no storage pools at all, which is equally supported.
func (s LXDService) SupportsPool(ctx context.Context, name string) (poolSuppoted bool, poolExists bool, err error) {
	poolMap := map[string]api.StoragePoolsPost{
		DefaultCephFSPool: s.DefaultCephFSStoragePool(),
		DefaultZFSPool:    s.DefaultZFSStoragePool(),
		DefaultCephPool:   s.DefaultCephStoragePool(),
	}

	c, err := s.Client(ctx, "")
	if err != nil {
		return false, false, err
	}

	cfg, ok := poolMap[name]
	if !ok {
		return false, false, fmt.Errorf("Pool %q is not a supported MicroCloud pool", name)
	}

	pool, _, err := c.GetStoragePool(name)
	// If the pool can't be found, then we can create it so return nil.
	if err != nil && api.StatusErrorCheck(err, http.StatusNotFound) {
		return true, false, nil
	}

	if err != nil {
		return false, false, err
	}

	if cfg.Driver != pool.Driver {
		return false, true, fmt.Errorf("Pool %q does not have the correct driver", name)
	}

	if pool.Status != "Created" {
		return false, true, fmt.Errorf("Pool %q is not fully set up", name)
	}

	for k, v := range cfg.Config {
		if pool.Config[k] != v {
			return false, true, fmt.Errorf("Pool %q has the wrong value for key %q, expected %q but got %q", name, k, v, pool.Config[k])
		}
	}

	return true, true, nil
}

// SupportsNetwork checks whetherLXD has the networks that matches what MicroCloud expects, or has no networks at all, which is equally supported.
func (s LXDService) SupportsNetwork(ctx context.Context, name string) (netSupported bool, netExists bool, err error) {
	uplink, ovn := s.DefaultOVNNetwork("", "", "", "")
	netMap := map[string]api.NetworksPost{
		DefaultUplinkNetwork: uplink,
		DefaultOVNNetwork:    ovn,
		DefaultFANNetwork:    s.defaultFanNetwork(),
	}

	c, err := s.Client(ctx, "")
	if err != nil {
		return false, false, err
	}

	cfg, ok := netMap[name]
	if !ok {
		return false, false, fmt.Errorf("Network %q is not a supported MicroCloud network", name)
	}

	net, _, err := c.GetNetwork(name)
	// If the network can't be found, then we can create it so return nil.
	if err != nil && api.StatusErrorCheck(err, http.StatusNotFound) {
		return true, false, nil
	}

	if err != nil {
		return false, false, err
	}

	if cfg.Type != net.Type {
		return false, true, fmt.Errorf("Network %q does not have the correct type", name)
	}

	if net.Status != "Created" {
		return false, true, fmt.Errorf("Network %q is not fully set up", name)
	}

	for k, v := range cfg.Config {
		if net.Config[k] != v {
			return false, true, fmt.Errorf("Network %q has the wrong value for key %q, expected %q but got %q", name, k, v, net.Config[k])
		}
	}

	return true, true, nil
}
