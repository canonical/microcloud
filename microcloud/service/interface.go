package service

import (
	"github.com/canonical/microcloud/microcloud/mdns"
)

// Service represents a common interface for all MicroCloud services.
type Service interface {
	Bootstrap() error
	IssueToken(peer string) (string, error)
	Join(config mdns.JoinConfig) error
	ClusterMembers() (map[string]string, error)

	Type() ServiceType
	Name() string
	Address() string
	Port() int
}
