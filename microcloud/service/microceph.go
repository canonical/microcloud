package service

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared"
	cephClient "github.com/canonical/microceph/microceph/client"
	"github.com/canonical/microcluster/client"
	"github.com/canonical/microcluster/microcluster"

	"github.com/canonical/microcloud/microcloud/api/types"
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

// Client returns a client to the Ceph unix socket. If target is specified, it will be added to the query params.
// If secret is specified, it will be added to the request header.
func (s CephService) Client(target string, secret string) (*client.Client, error) {
	c, err := s.m.LocalClient()
	if err != nil {
		return nil, err
	}

	if target != "" {
		c = c.UseTarget(target)
	}

	if secret != "" {
		c.Client.Client.Transport.(*http.Transport).Proxy = func(r *http.Request) (*url.URL, error) {
			r.Header.Set("X-MicroCloud-Auth", secret)
			if !strings.HasPrefix(r.URL.Path, "/1.0/services/microceph") {
				r.URL.Path = "/1.0/services/microceph" + r.URL.Path
			}

			return shared.ProxyFromEnvironment(r)
		}
	}

	return c, nil
}

// Bootstrap bootstraps the MicroCeph daemon on the default port.
func (s CephService) Bootstrap() error {
	err := s.m.NewCluster(s.name, util.CanonicalNetworkAddress(s.address, s.port), 2*time.Minute)
	if err != nil {
		return err
	}

	for {
		select {
		case <-time.After(30 * time.Second):
			return fmt.Errorf("Timed out waiting for MicroCeph cluster to initialize")
		default:
			names, err := s.ClusterMembers()
			if err != nil {
				return err
			}

			if len(names) > 0 {
				return nil
			}
		}
	}
}

// IssueToken issues a token for the given peer.
func (s CephService) IssueToken(peer string) (string, error) {
	return s.m.NewJoinToken(peer)
}

// Join joins a cluster with the given token.
func (s CephService) Join(joinConfig JoinConfig) error {
	err := s.m.JoinCluster(s.name, util.CanonicalNetworkAddress(s.address, s.port), joinConfig.Token, 5*time.Minute)
	if err != nil {
		return err
	}

	c, err := s.Client("", "")
	if err != nil {
		return err
	}

	for _, disk := range joinConfig.CephConfig {
		err := cephClient.AddDisk(context.Background(), c, &disk)
		if err != nil {
			return err
		}
	}

	return nil
}

// ClusterMembers returns a map of cluster member names and addresses.
func (s CephService) ClusterMembers() (map[string]string, error) {
	client, err := s.Client("", "")
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
