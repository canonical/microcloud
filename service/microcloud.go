package service

import (
	"context"
	"crypto/x509"
	"fmt"
	"strconv"
	"time"

	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/logger"
	cephTypes "github.com/canonical/microceph/microceph/api/types"
	microClient "github.com/canonical/microcluster/v2/client"
	"github.com/canonical/microcluster/v2/microcluster"
	"github.com/canonical/microcluster/v2/rest"
	"github.com/canonical/microcluster/v2/state"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/client"
	cloudClient "github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcloud/microcloud/version"
)

// CloudService is a MicroCloud service.
type CloudService struct {
	client *microcluster.MicroCluster

	name    string
	address string
	port    int64
	config  map[string]string
}

// JoinConfig represents configuration for cluster joining.
type JoinConfig struct {
	Token      string
	LXDConfig  []api.ClusterMemberConfigKey
	CephConfig []cephTypes.DisksPost
	OVNConfig  map[string]string
}

// NewCloudService creates a new MicroCloud service with a client attached.
func NewCloudService(name string, addr string, dir string) (*CloudService, error) {
	client, err := microcluster.App(microcluster.Args{StateDir: dir})
	if err != nil {
		return nil, err
	}

	return &CloudService{
		client:  client,
		name:    name,
		address: addr,
		port:    CloudPort,
		config:  make(map[string]string),
	}, nil
}

// StartCloud launches the MicroCloud daemon with the appropriate hooks.
func (s *CloudService) StartCloud(ctx context.Context, service *Handler, endpoints []rest.Endpoint, verbose bool, debug bool) error {
	args := microcluster.DaemonArgs{
		Verbose:              verbose,
		Debug:                debug,
		Version:              version.Version,
		PreInitListenAddress: "[::]:" + strconv.FormatInt(CloudPort, 10),
		Hooks: &state.Hooks{
			PostBootstrap: func(ctx context.Context, s state.State, cfg map[string]string) error { return service.StopBroadcast() },
			PostJoin:      func(ctx context.Context, s state.State, cfg map[string]string) error { return service.StopBroadcast() },
			OnStart:       service.Start,
		},
		ExtensionServers: map[string]rest.Server{
			"microcloud": {
				CoreAPI:   true,
				PreInit:   true,
				ServeUnix: true,
				Resources: []rest.Resources{
					{
						PathPrefix: types.APIVersion,
						Endpoints:  endpoints,
					},
				},
			},
		},
	}

	return s.client.Start(ctx, args)
}

// Client returns a client to the MicroCloud unix socket.
func (s CloudService) Client() (*microClient.Client, error) {
	return s.client.LocalClient()
}

// Bootstrap bootstraps the MicroCloud daemon on the default port.
func (s CloudService) Bootstrap(ctx context.Context) error {
	err := s.client.NewCluster(ctx, s.name, util.CanonicalNetworkAddress(s.address, s.port), nil)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("Timed out waiting for MicroCloud cluster to initialize")
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
func (s CloudService) IssueToken(ctx context.Context, peer string) (string, error) {
	return s.client.NewJoinToken(ctx, peer, 5*time.Minute)
}

// DeleteToken deletes a token by its name.
func (s CloudService) DeleteToken(ctx context.Context, tokenName string, address string) error {
	var c *microClient.Client
	var err error
	if address != "" {
		c, err = s.client.RemoteClient(util.CanonicalNetworkAddress(address, CloudPort))
		if err != nil {
			return err
		}

		c, err = cloudClient.UseAuthProxy(c, types.MicroCloud, cloudClient.AuthConfig{})
	} else {
		c, err = s.client.LocalClient()
	}

	if err != nil {
		return err
	}

	return c.DeleteTokenRecord(ctx, tokenName)
}

// RemoteIssueToken issues a token for the given peer on a remote MicroCloud where we are authorized by mDNS.
func (s CloudService) RemoteIssueToken(ctx context.Context, clusterAddress string, peer string, serviceType types.ServiceType) (string, error) {
	c, err := s.client.RemoteClient(util.CanonicalNetworkAddress(clusterAddress, CloudPort))
	if err != nil {
		return "", err
	}

	c, err = cloudClient.UseAuthProxy(c, types.MicroCloud, cloudClient.AuthConfig{})
	if err != nil {
		return "", err
	}

	return client.RemoteIssueToken(ctx, c, serviceType, types.ServiceTokensPost{ClusterAddress: c.URL().URL.Host, JoinerName: peer})
}

// Join joins a cluster with the given token.
func (s CloudService) Join(ctx context.Context, joinConfig JoinConfig) error {
	return s.client.JoinCluster(ctx, s.name, util.CanonicalNetworkAddress(s.address, s.port), joinConfig.Token, nil)
}

// remoteClient returns an https client for the given address:port.
// It picks the cluster certificate if none is provided to verify the remote.
func (s CloudService) remoteClient(cert *x509.Certificate, address string) (*microClient.Client, error) {
	var err error
	var client *microClient.Client

	canonicalAddress := util.CanonicalNetworkAddress(address, CloudPort)
	if cert != nil {
		client, err = s.client.RemoteClientWithCert(canonicalAddress, cert)
	} else {
		client, err = s.client.RemoteClient(canonicalAddress)
	}

	if err != nil {
		return nil, err
	}

	return client, nil
}

// RequestJoin sends the signal to initiate a join to the remote system, or timeout after a maximum of 5 min.
func (s CloudService) RequestJoin(ctx context.Context, name string, cert *x509.Certificate, joinConfig types.ServicesPut) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()

	var c *microClient.Client
	var err error
	if name == s.name {
		c, err = s.client.LocalClient()
		if err != nil {
			return err
		}
	} else {
		c, err = s.remoteClient(cert, joinConfig.Address)
		if err != nil {
			return err
		}

		c, err = cloudClient.UseAuthProxy(c, types.MicroCloud, cloudClient.AuthConfig{})
		if err != nil {
			return err
		}
	}

	return client.JoinServices(ctx, c, joinConfig)
}

// RemoteClusterMembers returns a map of cluster member names and addresses from the MicroCloud at the given address.
// Provide the certificate of the remote server for mTLS.
func (s CloudService) RemoteClusterMembers(ctx context.Context, cert *x509.Certificate, address string) (map[string]string, error) {
	client, err := s.remoteClient(cert, address)
	if err != nil {
		return nil, err
	}

	client, err = cloudClient.UseAuthProxy(client, types.MicroCloud, cloudClient.AuthConfig{})
	if err != nil {
		return nil, err
	}

	return clusterMembers(ctx, client)
}

// ClusterMembers returns a map of cluster member names and addresses.
func (s CloudService) ClusterMembers(ctx context.Context) (map[string]string, error) {
	client, err := s.client.LocalClient()
	if err != nil {
		return nil, err
	}

	return clusterMembers(ctx, client)
}

// clusterMembers returns a map of cluster member names and addresses.
func clusterMembers(ctx context.Context, client *microClient.Client) (map[string]string, error) {
	members, err := client.GetClusterMembers(ctx)
	if err != nil {
		return nil, err
	}

	genericMembers := make(map[string]string, len(members))
	for _, member := range members {
		genericMembers[member.Name] = member.Address.String()
	}

	return genericMembers, nil
}

// DeleteClusterMember removes the given cluster member from the service.
func (s CloudService) DeleteClusterMember(ctx context.Context, name string, force bool) error {
	c, err := s.client.LocalClient()
	if err != nil {
		return err
	}

	return c.DeleteClusterMember(ctx, name, force)
}

// Type returns the type of Service.
func (s CloudService) Type() types.ServiceType {
	return types.MicroCloud
}

// Name returns the name of this Service instance.
func (s CloudService) Name() string {
	return s.name
}

// Address returns the address of this Service instance.
func (s CloudService) Address() string {
	return s.address
}

// Port returns the port of this Service instance.
func (s CloudService) Port() int64 {
	return s.port
}

// SetConfig sets the config of this Service instance.
func (s *CloudService) SetConfig(config map[string]string) {
	if s.config == nil {
		s.config = make(map[string]string)
	}

	for key, value := range config {
		s.config[key] = value
	}
}

// SupportsFeature checks if the specified API feature of this Service instance if supported.
func (s *CloudService) SupportsFeature(ctx context.Context, feature string) (bool, error) {
	server, err := s.client.Status(ctx)
	if err != nil {
		return false, fmt.Errorf("Failed to get MicroCloud server status while checking for features: %v", err)
	}

	if server.Extensions == nil {
		logger.Warnf("MicroCloud server does not expose API extensions")
		return false, nil
	}

	return server.Extensions.HasExtension(feature), nil
}
