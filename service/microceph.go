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
	"github.com/canonical/lxd/shared/logger"
	cephTypes "github.com/canonical/microceph/microceph/api/types"
	cephClient "github.com/canonical/microceph/microceph/client"
	"github.com/canonical/microcluster/v2/client"
	"github.com/canonical/microcluster/v2/microcluster"

	"github.com/canonical/microcloud/microcloud/api/types"
	cloudClient "github.com/canonical/microcloud/microcloud/client"
)

// CephService is a MicroCeph service.
type CephService struct {
	m *microcluster.MicroCluster

	name    string
	address string
	port    int64
	config  map[string]string
}

// NewCephService creates a new MicroCeph service with a client attached.
func NewCephService(name string, addr string, cloudDir string) (*CephService, error) {
	proxy := func(r *http.Request) (*url.URL, error) {
		if !strings.HasPrefix(r.URL.Path, "/1.0/services/microceph") {
			r.URL.Path = "/1.0/services/microceph" + r.URL.Path
		}

		return shared.ProxyFromEnvironment(r)
	}

	client, err := microcluster.App(microcluster.Args{StateDir: cloudDir, Proxy: proxy})
	if err != nil {
		return nil, err
	}

	return &CephService{
		m:       client,
		name:    name,
		address: addr,
		port:    CephPort,
		config:  make(map[string]string),
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

	c, err = cloudClient.UseAuthProxy(c, secret, types.MicroCeph)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// Bootstrap bootstraps the MicroCeph daemon on the default port.
func (s CephService) Bootstrap(ctx context.Context) error {
	err := s.m.NewCluster(ctx, s.name, util.CanonicalNetworkAddress(s.address, s.port), s.config)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("Timed out waiting for MicroCeph cluster to initialize")
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

// IssueToken issues a token for the given peer. Each token will last 5 minutes in case the system joins the cluster very slowly.
func (s CephService) IssueToken(ctx context.Context, peer string) (string, error) {
	return s.m.NewJoinToken(ctx, peer, 5*time.Minute)
}

// DeleteToken deletes a token by its name.
func (s CephService) DeleteToken(ctx context.Context, tokenName string, address string, secret string) error {
	var c *client.Client
	var err error
	if address != "" && secret != "" {
		c, err = s.m.RemoteClient(util.CanonicalNetworkAddress(address, CloudPort))
		if err != nil {
			return err
		}

		c, err = cloudClient.UseAuthProxy(c, secret, types.MicroCeph)
	} else {
		c, err = s.m.LocalClient()
	}

	if err != nil {
		return err
	}

	return c.DeleteTokenRecord(ctx, tokenName)
}

// Join joins a cluster with the given token.
func (s CephService) Join(ctx context.Context, joinConfig JoinConfig) error {
	err := s.m.JoinCluster(ctx, s.name, util.CanonicalNetworkAddress(s.address, s.port), joinConfig.Token, nil)
	if err != nil {
		return err
	}

	c, err := s.Client("", "")
	if err != nil {
		return err
	}

	for _, disk := range joinConfig.CephConfig {
		_, err := cephClient.AddDisk(ctx, c, &disk)
		if err != nil {
			return err
		}
	}

	return nil
}

// RemoteClusterMembers returns a map of cluster member names and addresses from the MicroCloud at the given address, authenticated with the given secret.
func (s CephService) RemoteClusterMembers(ctx context.Context, secret string, address string) (map[string]string, error) {
	client, err := s.m.RemoteClient(util.CanonicalNetworkAddress(address, CloudPort))
	if err != nil {
		return nil, err
	}

	client, err = cloudClient.UseAuthProxy(client, secret, types.MicroCeph)
	if err != nil {
		return nil, err
	}

	return clusterMembers(ctx, client)
}

// ClusterMembers returns a map of cluster member names and addresses.
func (s CephService) ClusterMembers(ctx context.Context) (map[string]string, error) {
	client, err := s.Client("", "")
	if err != nil {
		return nil, err
	}

	return clusterMembers(ctx, client)
}

// DeleteClusterMember removes the given cluster member from the service.
func (s CephService) DeleteClusterMember(ctx context.Context, name string, force bool) error {
	c, err := s.m.LocalClient()
	if err != nil {
		return err
	}

	return c.DeleteClusterMember(ctx, name, force)
}

// ClusterConfig returns the Ceph cluster configuration.
func (s CephService) ClusterConfig(ctx context.Context, targetAddress string, targetSecret string) (map[string]string, error) {
	var c *client.Client
	var err error
	if targetAddress == "" && targetSecret == "" {
		c, err = s.Client("", "")
		if err != nil {
			return nil, err
		}
	} else {
		c, err = s.m.RemoteClient(util.CanonicalNetworkAddress(targetAddress, CloudPort))
		if err != nil {
			return nil, err
		}

		c, err = cloudClient.UseAuthProxy(c, targetSecret, types.MicroCeph)
		if err != nil {
			return nil, err
		}
	}

	data := cephTypes.Config{}
	cs, err := cephClient.GetConfig(ctx, c, &data)
	if err != nil {
		return nil, err
	}

	configs := make(map[string]string, len(cs))
	for _, c := range cs {
		configs[c.Key] = c.Value
	}

	return configs, nil
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
func (s CephService) Port() int64 {
	return s.port
}

// SetConfig sets the config of this Service instance.
func (s *CephService) SetConfig(config map[string]string) {
	if s.config == nil {
		s.config = make(map[string]string)
	}

	for key, value := range config {
		s.config[key] = value
	}
}

// SupportsFeature checks if the specified API feature of this Service instance if supported.
func (s *CephService) SupportsFeature(ctx context.Context, feature string) (bool, error) {
	server, err := s.m.Status(ctx)
	if err != nil {
		return false, fmt.Errorf("Failed to get MicroCeph server status while checking for features: %v", err)
	}

	if server.Extensions == nil {
		logger.Warnf("MicroCeph server does not expose API extensions")
		return false, nil
	}

	return server.Extensions.HasExtension(feature), nil
}
