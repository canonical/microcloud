package types

import (
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/microceph/microceph/api/types"
)

// ComponentType represents supported components.
type ComponentType string

const (
	// MicroCloud represents a MicroCloud component.
	MicroCloud ComponentType = "MicroCloud"

	// MicroCeph represents a MicroCeph component.
	MicroCeph ComponentType = "MicroCeph"

	// MicroOVN represents a MicroOVN component.
	MicroOVN ComponentType = "MicroOVN"

	// LXD represents a LXD component.
	LXD ComponentType = "LXD"
)

// ComponentsPut represents data for updating the cluster configuration of the MicroCloud components.
type ComponentsPut struct {
	Tokens  []ComponentToken `json:"tokens" yaml:"tokens"`
	Address string           `json:"address" yaml:"address"`

	LXDConfig  []api.ClusterMemberConfigKey `json:"lxd_config" yaml:"lxd_config"`
	CephConfig []types.DisksPost            `json:"ceph_config" yaml:"ceph_config"`
	OVNConfig  map[string]string            `json:"ovn_config" yaml:"ovn_config"`
}

// ComponentToken represents a join token for a component join request.
type ComponentToken struct {
	Component ComponentType `json:"component" yaml:"component"`
	JoinToken string        `json:"join_token" yaml:"join_token"`
}

// ComponentTokensPost represents a request to issue a join token for a MicroCloud component.
type ComponentTokensPost struct {
	ClusterAddress string `json:"cluster_address" yaml:"cluster_address"`
	JoinerName     string `json:"joiner_name"     yaml:"joiner_name"`
}
