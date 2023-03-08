package service

import (
	"github.com/canonical/microcloud/microcloud/api/types"
)

// Service represents a common interface for all MicroCloud services.
type Service interface {
	Bootstrap() error
	IssueToken(peer string) (string, error)
	Join(config JoinConfig) error
	ClusterMembers() (map[string]string, error)

	Type() types.ServiceType
	Name() string
	Address() string
	Port() int
}
