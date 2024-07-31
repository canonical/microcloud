package types

import (
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/microceph/microceph/api/types"
)

// ServiceType represents supported services.
type ServiceType string

const (
	// MicroCloud represents a MicroCloud service.
	MicroCloud ServiceType = "MicroCloud"

	// MicroCeph represents a MicroCeph service.
	MicroCeph ServiceType = "MicroCeph"

	// MicroOVN represents a MicroOVN service.
	MicroOVN ServiceType = "MicroOVN"

	// LXD represents a LXD service.
	LXD ServiceType = "LXD"
)

// ServicesPut represents data for updating the cluster configuration of the MicroCloud services.
type ServicesPut struct {
	Tokens  []ServiceToken `json:"tokens" yaml:"tokens"`
	Address string         `json:"address" yaml:"address"`

	LXDConfig  []api.ClusterMemberConfigKey `json:"lxd_config" yaml:"lxd_config"`
	CephConfig []types.DisksPost            `json:"ceph_config" yaml:"ceph_config"`
	OVNConfig  map[string]string            `json:"ovn_config" yaml:"ovn_config"`
}

// ServiceToken represents a join token for a service join request.
type ServiceToken struct {
	Service   ServiceType `json:"service" yaml:"service"`
	JoinToken string      `json:"join_token" yaml:"join_token"`
}

// ServiceTokensPost represents a request to issue a join token for a MicroCloud service.
type ServiceTokensPost struct {
	ClusterAddress string `json:"cluster_address" yaml:"cluster_address"`
	JoinerName     string `json:"joiner_name"     yaml:"joiner_name"`
}
