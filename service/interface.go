package service

import (
	"context"
	"crypto/x509"

	"github.com/canonical/microcloud/microcloud/api/types"
)

// Service represents a common interface for all MicroCloud services.
type Service interface {
	Bootstrap(ctx context.Context) error
	Join(ctx context.Context, config JoinConfig) error

	IssueToken(ctx context.Context, peer string) (string, error)
	DeleteToken(ctx context.Context, tokenName string, address string) error

	ClusterMembers(ctx context.Context) (map[string]string, error)
	// RemoteClusterMembers is called during the pre-init phase of microcluster.
	// It allows providing the certificate of the remote microcluster member for mTLS verification.
	RemoteClusterMembers(ctx context.Context, cert *x509.Certificate, address string) (map[string]string, error)
	DeleteClusterMember(ctx context.Context, name string, force bool) error

	Type() types.ServiceType
	Name() string
	Address() string
	Port() int64
	SetConfig(config map[string]string)
	SupportsFeature(ctx context.Context, feature string) (bool, error)
}
