package service

import (
	"context"

	"github.com/canonical/microcloud/microcloud/api/types"
)

// Service represents a common interface for all MicroCloud services.
type Service interface {
	Bootstrap(ctx context.Context) error
	IssueToken(ctx context.Context, peer string) (string, error)
	Join(ctx context.Context, config JoinConfig) error

	ClusterMembers(ctx context.Context) (map[string]string, error)
	RemoteClusterMembers(ctx context.Context, secret string, address string) (map[string]string, error)

	Type() types.ServiceType
	Name() string
	Address() string
	Port() int64
}
