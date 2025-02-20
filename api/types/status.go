package types

import (
	cephTypes "github.com/canonical/microceph/microceph/api/types"
	microTypes "github.com/canonical/microcluster/v2/rest/types"
	ovnTypes "github.com/canonical/microovn/microovn/api/types"
)

// Status is a set of status information from a cluster member.
type Status struct {
	// Name represents the cluster name for the member.
	Name string `json:"name" yaml:"name"`

	// Address represnts the cluster address for the member.
	Address string `json:"address" yaml:"address"`

	// Clusters is a list of cluster members for each component installed on the member.
	Clusters map[ComponentType][]microTypes.ClusterMember `json:"clusters" yaml:"clusters"`

	// OSDs is a list of all OSDs local to the member.
	OSDs cephTypes.Disks `json:"osds" yaml:"osds"`

	// CephComponents is a list of all ceph components running on this member.
	CephComponents cephTypes.Services `json:"ceph_components" yaml:"ceph_components"`

	// OVNComponents is a list of all ovn components running on this member.
	OVNComponents ovnTypes.Services `json:"ovn_components" yaml:"ovn_components"`
}
