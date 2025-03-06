package service

import (
	"context"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/logger"
	cephTypes "github.com/canonical/microceph/microceph/api/types"
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
func (s CephService) Client(target string) (*client.Client, error) {
	c, err := s.m.LocalClient()
	if err != nil {
		return nil, err
	}

	if target != "" {
		c = c.UseTarget(target)
	}

	c, err = cloudClient.UseAuthProxy(c, types.MicroCeph, cloudClient.AuthConfig{})
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
func (s CephService) DeleteToken(ctx context.Context, tokenName string, address string) error {
	var c *client.Client
	var err error
	if address != "" {
		c, err = s.m.RemoteClient(util.CanonicalNetworkAddress(address, CloudPort))
		if err != nil {
			return err
		}

		c, err = cloudClient.UseAuthProxy(c, types.MicroCeph, cloudClient.AuthConfig{})
	} else {
		c, err = s.m.LocalClient()
	}

	if err != nil {
		return err
	}

	return c.DeleteTokenRecord(ctx, tokenName)
}

// AddDisk requests Ceph sets up a new OSD.
func (s CephService) AddDisk(ctx context.Context, data cephTypes.DisksPost, target string) (cephTypes.DiskAddResponse, error) {
	response := cephTypes.DiskAddResponse{}

	c, err := s.Client(target)
	if err != nil {
		return response, err
	}

	err = c.Query(ctx, "POST", types.APIVersion, api.NewURL().Path("disks"), data, &response)
	if err != nil {
		return response, fmt.Errorf("Failed to request disk addition: %w", err)
	}

	return response, nil
}

// GetServices returns the list of configured ceph services.
func (s CephService) GetServices(ctx context.Context, target string) (cephTypes.Services, error) {
	c, err := s.Client(target)
	if err != nil {
		return nil, err
	}

	services := cephTypes.Services{}

	err = c.Query(ctx, "GET", types.APIVersion, api.NewURL().Path("services"), nil, &services)
	if err != nil {
		return nil, fmt.Errorf("Failed listing services: %w", err)
	}

	return services, nil
}

// GetDisks returns the list of configured disks.
func (s CephService) GetDisks(ctx context.Context, target string) (cephTypes.Disks, error) {
	c, err := s.Client(target)
	if err != nil {
		return nil, err
	}

	disks := cephTypes.Disks{}

	err = c.Query(ctx, "GET", types.APIVersion, api.NewURL().Path("disks"), nil, &disks)
	if err != nil {
		return nil, fmt.Errorf("Failed listing disks: %w", err)
	}

	return disks, nil
}

// GetPools returns a list of pools.
func (s CephService) GetPools(ctx context.Context, target string) ([]cephTypes.Pool, error) {
	c, err := s.Client(target)
	if err != nil {
		return nil, err
	}

	var pools []cephTypes.Pool

	err = c.Query(ctx, "GET", types.APIVersion, api.NewURL().Path("pools"), nil, &pools)
	if err != nil {
		return nil, fmt.Errorf("Failed listing OSD pools: %w", err)
	}

	return pools, nil
}

// PoolSetReplicationFactor sets the replication factor for the given pools.
func (s CephService) PoolSetReplicationFactor(ctx context.Context, data cephTypes.PoolPut, target string) error {
	c, err := s.Client(target)
	if err != nil {
		return err
	}

	err = c.Query(ctx, "PUT", types.APIVersion, api.NewURL().Path("pools-op"), data, nil)
	if err != nil {
		return fmt.Errorf("Failed setting replication factor: %w", err)
	}

	return nil
}

// GetConfig returns the requested config.
// It allows passing a certificate in case the cluster config is derived directly from the remote
// before the MicroCloud cluster is being formed.
// If the cluster is already formed and the trust got established this isn't required anymore.
func (s CephService) GetConfig(ctx context.Context, data cephTypes.Config, target string, cert *x509.Certificate) (cephTypes.Configs, error) {
	var c *client.Client
	var err error

	if target == "" {
		c, err = s.Client("")
		if err != nil {
			return nil, err
		}
	} else {
		c, err = s.remoteClient(cert, target)
		if err != nil {
			return nil, err
		}

		c, err = cloudClient.UseAuthProxy(c, types.MicroCeph, cloudClient.AuthConfig{})
		if err != nil {
			return nil, err
		}
	}

	configs := cephTypes.Configs{}

	err = c.Query(ctx, "GET", types.APIVersion, api.NewURL().Path("configs"), data, &configs)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch cluster config key %q: %w", data.Key, err)
	}

	return configs, nil
}

// Join joins a cluster with the given token.
func (s CephService) Join(ctx context.Context, joinConfig JoinConfig) error {
	err := s.m.JoinCluster(ctx, s.name, util.CanonicalNetworkAddress(s.address, s.port), joinConfig.Token, nil)
	if err != nil {
		return err
	}

	for _, disk := range joinConfig.CephConfig {
		_, err := s.AddDisk(ctx, disk, "")
		if err != nil {
			return err
		}
	}

	return nil
}

// remoteClient returns an https client for the given address:port.
// It picks the cluster certificate if none is provided to verify the remote.
func (s CephService) remoteClient(cert *x509.Certificate, address string) (*client.Client, error) {
	var err error
	var client *client.Client

	canonicalAddress := util.CanonicalNetworkAddress(address, CloudPort)
	if cert != nil {
		client, err = s.m.RemoteClientWithCert(canonicalAddress, cert)
	} else {
		client, err = s.m.RemoteClient(canonicalAddress)
	}

	if err != nil {
		return nil, err
	}

	return client, nil
}

// RemoteClusterMembers returns a map of cluster member names and addresses from the MicroCloud at the given address.
// Provide the certificate of the remote server for mTLS.
func (s CephService) RemoteClusterMembers(ctx context.Context, cert *x509.Certificate, address string) (map[string]string, error) {
	client, err := s.remoteClient(cert, address)
	if err != nil {
		return nil, err
	}

	client, err = cloudClient.UseAuthProxy(client, types.MicroCeph, cloudClient.AuthConfig{})
	if err != nil {
		return nil, err
	}

	return clusterMembers(ctx, client)
}

// ClusterMembers returns a map of cluster member names and addresses.
func (s CephService) ClusterMembers(ctx context.Context) (map[string]string, error) {
	client, err := s.Client("")
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
func (s CephService) ClusterConfig(ctx context.Context, targetAddress string, cert *x509.Certificate) (map[string]string, error) {
	data := cephTypes.Config{}
	cs, err := s.GetConfig(ctx, data, targetAddress, cert)
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

// GetVersion gets the installed daemon version of the service, and returns an error if the version is not supported.
func (s CephService) GetVersion(ctx context.Context) (string, error) {
	status, err := s.m.Status(ctx)
	if err != nil && api.StatusErrorCheck(err, http.StatusNotFound) {
		return "", fmt.Errorf("The installed version of %s is not supported", s.Type())
	}

	if err != nil {
		return "", err
	}

	err = validateVersion(s.Type(), status.Version)
	if err != nil {
		return "", err
	}

	return status.Version, nil
}

// IsInitialized returns whether the service is initialized.
func (s CephService) IsInitialized(ctx context.Context) (bool, error) {
	err := s.m.Ready(ctx)
	if err != nil && api.StatusErrorCheck(err, http.StatusNotFound) {
		return false, fmt.Errorf("Unix socket not found. Check if %s is installed", s.Type())
	}

	if err != nil {
		return false, fmt.Errorf("Failed to wait for %s to get ready: %w", s.Type(), err)
	}

	status, err := s.m.Status(ctx)
	if err != nil {
		return false, fmt.Errorf("Failed to get %s status: %w", s.Type(), err)
	}

	return status.Ready, nil
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
