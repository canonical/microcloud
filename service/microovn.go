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
	"github.com/canonical/microcluster/client"
	"github.com/canonical/microcluster/microcluster"

	"github.com/canonical/microcloud/microcloud/api/types"
	cloudClient "github.com/canonical/microcloud/microcloud/client"
)

// OVNService is a MicroOVN service.
type OVNService struct {
	m *microcluster.MicroCluster

	name    string
	address string
	port    int64
	config  map[string]string
}

// NewOVNService creates a new MicroOVN service with a client attached.
func NewOVNService(name string, addr string, cloudDir string, config map[string]string) (*OVNService, error) {
	proxy := func(r *http.Request) (*url.URL, error) {
		if !strings.HasPrefix(r.URL.Path, "/1.0/services/microovn") {
			r.URL.Path = "/1.0/services/microovn" + r.URL.Path
		}

		return shared.ProxyFromEnvironment(r)
	}

	client, err := microcluster.App(microcluster.Args{StateDir: cloudDir, Proxy: proxy})
	if err != nil {
		return nil, err
	}

	return &OVNService{
		m:       client,
		name:    name,
		address: addr,
		port:    OVNPort,
		config:  config,
	}, nil
}

// Client returns a client to the OVN unix socket.
func (s OVNService) Client() (*client.Client, error) {
	return s.m.LocalClient()
}

// Bootstrap bootstraps the MicroOVN daemon on the default port.
func (s OVNService) Bootstrap(ctx context.Context) error {
	err := s.m.NewCluster(ctx, s.name, util.CanonicalNetworkAddress(s.address, s.port), s.config)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("Timed out waiting for MicroOVN cluster to initialize")
		default:
			names, err := s.ClusterMembers(ctx)
			if err != nil {
				return err
			}

			if len(names) > 0 {
				return nil
			}

			time.Sleep(time.Second)
		}
	}
}

// IssueToken issues a token for the given peer.
func (s OVNService) IssueToken(ctx context.Context, peer string) (string, error) {
	return s.m.NewJoinToken(ctx, peer)
}

// Join joins a cluster with the given token.
func (s OVNService) Join(ctx context.Context, joinConfig JoinConfig) error {
	return s.m.JoinCluster(ctx, s.name, util.CanonicalNetworkAddress(s.address, s.port), joinConfig.Token, nil)
}

// RemoteClusterMembers returns a map of cluster member names and addresses from the MicroCloud at the given address, authenticated with the given secret.
func (s OVNService) RemoteClusterMembers(ctx context.Context, secret string, address string) (map[string]string, error) {
	client, err := s.m.RemoteClient(util.CanonicalNetworkAddress(address, CloudPort))
	if err != nil {
		return nil, err
	}

	client, err = cloudClient.UseAuthProxy(client, secret, types.MicroOVN)
	if err != nil {
		return nil, err
	}

	return clusterMembers(ctx, client)
}

// ClusterMembers returns a map of cluster member names and addresses.
func (s OVNService) ClusterMembers(ctx context.Context) (map[string]string, error) {
	client, err := s.Client()
	if err != nil {
		return nil, err
	}

	return clusterMembers(ctx, client)
}

// Type returns the type of Service.
func (s OVNService) Type() types.ServiceType {
	return types.MicroOVN
}

// Name returns the name of this Service instance.
func (s OVNService) Name() string {
	return s.name
}

// Address returns the address of this Service instance.
func (s OVNService) Address() string {
	return s.address
}

// Port returns the port of this Service instance.
func (s OVNService) Port() int64 {
	return s.port
}

// SetConfig sets the config of this Service instance.
func (s *OVNService) SetConfig(config map[string]string) {
	if s.config == nil {
		s.config = make(map[string]string)
	}

	for key, value := range config {
		s.config[key] = value
	}
}
