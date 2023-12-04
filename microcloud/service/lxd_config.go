package service

import (
	"fmt"
	"net"
	"strconv"

	"github.com/canonical/lxd/shared/api"
)

// OVNNetworkType represents the type of OVN network.
type OVNNetworkType int

const (
	// OVNUplinkNetwork represents the OVN uplink network (north-south traffic).
	OVNUplinkNetwork OVNNetworkType = iota
	// OVNUnderlayNetwork represents the OVN underlay network (east-west traffic).
	OVNUnderlayNetwork
)

// String returns the string representation of the OVNNetworkType.
func (o OVNNetworkType) String() string {
	switch o {
	case OVNUplinkNetwork:
		return "UPLINK"
	case OVNUnderlayNetwork:
		return "UNDERLAY"
	default:
		return ""
	}
}

// DefaultPendingFanNetwork returns the default Ubuntu Fan network configuration when
// creating a pending network on a specific cluster member target.
func (s LXDService) DefaultPendingFanNetwork() api.NetworksPost {
	return api.NetworksPost{Name: "lxdfan0", Type: "bridge"}
}

// DefaultFanNetwork returns the default Ubuntu Fan network configuration when
// creating the finalized network.
func (s LXDService) DefaultFanNetwork() (api.NetworksPost, error) {
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

	return api.NetworksPost{
		NetworkPut: api.NetworkPut{
			Config: map[string]string{
				"bridge.mode":         "fan",
				"fan.underlay_subnet": underlay.String(),
			},
			Description: "Default Ubuntu fan powered bridge",
		},
		Name: "lxdfan0",
		Type: "bridge",
	}, nil
}

// PendingOVNNetwork either returns the OVN uplink or underlay network configuration when
// creating a pending network on a specific cluster member target.
func (s LXDService) PendingOVNNetwork(ovnNetworkType OVNNetworkType, parent string) api.NetworksPost {
	return api.NetworksPost{
		NetworkPut: api.NetworkPut{Config: map[string]string{"parent": parent}},
		Name:       ovnNetworkType.String(),
		Type:       "physical",
	}
}

// OVNNetworkJoinConfig either returns the OVN uplink or underlay network configuration when
// joining an existing cluster.
func (s LXDService) OVNNetworkJoinConfig(ovnNetworkType OVNNetworkType, parent string) api.ClusterMemberConfigKey {
	return api.ClusterMemberConfigKey{
		Entity: "network",
		Name:   ovnNetworkType.String(),
		Key:    "parent",
		Value:  parent,
	}
}

// OVNNetwork either returns the default uplink
// (or an underlay, if specified) OVN network configuration when
// creating the finalized network.
// Returns both the physical link (uplink or underlay) configuration as the first argument,
// and the OVN network configuration as the second argument.
func (s LXDService) OVNNetwork(ovnNetworkType OVNNetworkType, ipv4Gateway string, ipv4Range string, ipv6Gateway string) (api.NetworksPost, api.NetworksPost) {
	var physLinkCfgDesc string
	if ovnNetworkType == OVNUplinkNetwork {
		physLinkCfgDesc = "Uplink for OVN networks"
	} else {
		physLinkCfgDesc = "Underlay for OVN networks"
	}

	physLinkCfg := api.NetworksPost{
		NetworkPut: api.NetworkPut{
			Config:      map[string]string{},
			Description: physLinkCfgDesc},
		Name: ovnNetworkType.String(),
		Type: "physical",
	}

	if ipv4Gateway != "" && ipv4Range != "" {
		physLinkCfg.Config["ipv4.gateway"] = ipv4Gateway
		physLinkCfg.Config["ipv4.ovn.ranges"] = ipv4Range
	}

	if ipv6Gateway != "" {
		physLinkCfg.Config["ipv6.gateway"] = ipv6Gateway
	}

	var ovnNetworkName string
	var ovnNetworkDesc string
	if ovnNetworkType == OVNUplinkNetwork {
		ovnNetworkName = "default"
		ovnNetworkDesc = "Default OVN network"
	} else {
		ovnNetworkName = "underlay"
		ovnNetworkDesc = "Underlay OVN network"
	}

	ovnNetwork := api.NetworksPost{
		NetworkPut: api.NetworkPut{Config: map[string]string{"network": ovnNetworkType.String()}, Description: ovnNetworkDesc},
		Name:       ovnNetworkName,
		Type:       "ovn",
	}

	return physLinkCfg, ovnNetwork
}

// DefaultPendingZFSStoragePool returns the default local storage configuration when
// creating a pending pool on a specific cluster member target.
func (s LXDService) DefaultPendingZFSStoragePool(wipe bool, path string) api.StoragePoolsPost {
	return api.StoragePoolsPost{
		Name:   "local",
		Driver: "zfs",
		StoragePoolPut: api.StoragePoolPut{
			Config:      map[string]string{"source": path, "source.wipe": strconv.FormatBool(wipe)},
			Description: "Local storage on ZFS",
		},
	}
}

// DefaultZFSStoragePool returns the default local storage configuration when
// creating the finalized pool.
func (s LXDService) DefaultZFSStoragePool() api.StoragePoolsPost {
	return api.StoragePoolsPost{
		Name:   "local",
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
		Name:   "local",
		Key:    "source.wipe",
		Value:  "true",
	}

	sourceTemplate := api.ClusterMemberConfigKey{
		Entity: "storage-pool",
		Name:   "local",
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
		Name:   "remote",
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
		Name:   "remote",
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
		Name:   "remote",
		Key:    "source",
		Value:  "lxd_remote",
	}
}
