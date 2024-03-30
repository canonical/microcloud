package service

import (
	"context"
	"crypto/tls"
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
	port    int64
	config  map[string]string
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
func (s CephService) Bootstrap(ctx context.Context) error {
	err := s.m.NewCluster(s.name, util.CanonicalNetworkAddress(s.address, s.port), s.config, 2*time.Minute)
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

// IssueToken issues a token for the given peer.
func (s CephService) IssueToken(ctx context.Context, peer string) (string, error) {
	return s.m.NewJoinToken(peer)
}

// Join joins a cluster with the given token.
func (s CephService) Join(ctx context.Context, joinConfig JoinConfig) error {
	err := s.m.JoinCluster(s.name, util.CanonicalNetworkAddress(s.address, s.port), joinConfig.Token, nil, 5*time.Minute)
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

	client.Client.Client.Transport = &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		DisableKeepAlives: true,
		Proxy: func(r *http.Request) (*url.URL, error) {
			r.Header.Set("X-MicroCloud-Auth", secret)
			if !strings.HasPrefix(r.URL.Path, "/1.0/services/microceph") {
				r.URL.Path = "/1.0/services/microceph" + r.URL.Path
			}

			return shared.ProxyFromEnvironment(r)
		},
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
