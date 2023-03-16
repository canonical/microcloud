package types

import (
	"github.com/lxc/lxd/shared/api"
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

	LXDConfig []api.ClusterMemberConfigKey `json:"lxd_config" yaml:"lxd_config"`
}

// ServiceToken represents a join token for a service join request.
type ServiceToken struct {
	Service   ServiceType `json:"service" yaml:"service"`
	JoinToken string      `json:"join_token" yaml:"join_token"`
}
