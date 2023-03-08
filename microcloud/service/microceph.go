package service

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcluster/client"
	"github.com/canonical/microcluster/microcluster"
	"github.com/lxc/lxd/lxd/util"
	"github.com/lxc/lxd/shared"
)

// CephService is a MicroCeph service.
type CephService struct {
	m *microcluster.MicroCluster

	name    string
	address string
	port    int
}

// NewCephService creates a new MicroCeph service with a client attached.
func NewCephService(ctx context.Context, name string, addr string, cloudDir string) (*CephService, error) {
	proxy := func(r *http.Request) (*url.URL, error) {
		if !strings.HasPrefix(r.URL.Path, "/1.0/services/microceph") {
			r.URL.Path = "/1.0/services/microceph" + r.URL.Path
		}

		return shared.ProxyFromEnvironment(r)
	}

	client, err := microcluster.App(ctx, microcluster.Args{StateDir: cloudDir, Proxy: proxy})
	if err != nil {
		return nil, err
	}

	return &CephService{
		m:       client,
		name:    name,
		address: addr,
		port:    CephPort,
	}, nil
}

// client returns a client to the Ceph unix socket.
func (s CephService) Client() (*client.Client, error) {
	return s.m.LocalClient()
}

// Bootstrap bootstraps the MicroCeph daemon on the default port.
func (s CephService) Bootstrap() error {
	return s.m.NewCluster(s.name, util.CanonicalNetworkAddress(s.address, s.port), time.Second*120)
}

// IssueToken issues a token for the given peer.
func (s CephService) IssueToken(peer string) (string, error) {
	return s.m.NewJoinToken(peer)
}

// Join joins a cluster with the given token.
func (s CephService) Join(joinConfig JoinConfig) error {
	return s.m.JoinCluster(s.name, util.CanonicalNetworkAddress(s.address, s.port), joinConfig.Token, 5*time.Minute)
}

// ClusterMembers returns a map of cluster member names and addresses.
func (s CephService) ClusterMembers() (map[string]string, error) {
	client, err := s.Client()
	if err != nil {
		return nil, err
	}

	members, err := client.GetClusterMembers(context.Background())
	if err != nil {
		return nil, err
	}

	genericMembers := make(map[string]string, len(members))
	for _, member := range members {
		genericMembers[member.Name] = member.Address.String()
	}

	return genericMembers, nil
}

// Type returns the type of Service.
func (s CephService) Type() types.ServiceType {
	return types.MicroCeph
}

// Name returns the name of this Service instance.
func (s CephService) Name() string {
	return s.name
}

// Address returns the address of this Service instance.
func (s CephService) Address() string {
	return s.address
}

// Port returns the port of this Service instance.
func (s CephService) Port() int {
	return s.port
}
