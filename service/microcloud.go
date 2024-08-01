package service

import (
	"context"
	"fmt"
	"time"

	"github.com/canonical/lxd/lxd/db/schema"
	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared/api"
	cephTypes "github.com/canonical/microceph/microceph/api/types"
	microClient "github.com/canonical/microcluster/v2/client"
	"github.com/canonical/microcluster/v2/microcluster"
	"github.com/canonical/microcluster/v2/rest"
	"github.com/canonical/microcluster/v2/state"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/client"
	cloudClient "github.com/canonical/microcloud/microcloud/client"
)

// CloudService is a MicroCloud service.
type CloudService struct {
	client *microcluster.MicroCluster

	name        string
	address     string
	port        int64
	verbose     bool
	debug       bool
	socketGroup string
	config      map[string]string
}

// JoinConfig represents configuration for cluster joining.
type JoinConfig struct {
	Token      string
	LXDConfig  []api.ClusterMemberConfigKey
	CephConfig []cephTypes.DisksPost
}

// NewCloudService creates a new MicroCloud service with a client attached.
func NewCloudService(name string, addr string, dir string, verbose bool, debug bool, socketGroup string) (*CloudService, error) {
	client, err := microcluster.App(microcluster.Args{StateDir: dir})
	if err != nil {
		return nil, err
	}

	return &CloudService{
		client:      client,
		name:        name,
		address:     addr,
		port:        CloudPort,
		verbose:     verbose,
		debug:       debug,
		socketGroup: socketGroup,
		config:      make(map[string]string),
	}, nil
}

// StartCloud launches the MicroCloud daemon with the appropriate hooks.
func (s *CloudService) StartCloud(ctx context.Context, service *Handler, endpoints []rest.Endpoint) error {
	var servers = map[string]rest.Server{
		"extended": {
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
	}

	dargs := microcluster.DaemonArgs{
		Verbose:          s.verbose,
		Debug:            s.debug,
		SocketGroup:      s.socketGroup,
		Version:          types.APIVersion,
		ExtensionsSchema: []schema.Update{},
		APIExtensions:    []string{},
		ExtensionServers: servers,
		Hooks: &state.Hooks{
			PostBootstrap: func(ctx context.Context, s state.State, cfg map[string]string) error { return service.StopBroadcast() },
			PostJoin:      func(ctx context.Context, s state.State, cfg map[string]string) error { return service.StopBroadcast() },
			OnStart:       service.Start,
		},
	}

	return s.client.Start(ctx, dargs)
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

// IssueToken issues a token for the given peer.
func (s CloudService) IssueToken(ctx context.Context, peer string) (string, error) {
	return s.client.NewJoinToken(ctx, peer, time.Hour)
}

// RemoteIssueToken issues a token for the given peer on a remote MicroCloud where we are authorized by mDNS.
func (s CloudService) RemoteIssueToken(ctx context.Context, clusterAddress string, secret string, peer string, serviceType types.ServiceType) (string, error) {
	c, err := s.client.RemoteClient(util.CanonicalNetworkAddress(clusterAddress, CloudPort))
	if err != nil {
		return "", err
	}

	c, err = cloudClient.UseAuthProxy(c, secret, types.MicroCloud)
	if err != nil {
		return "", err
	}

	return client.RemoteIssueToken(ctx, c, serviceType, types.ServiceTokensPost{ClusterAddress: c.URL().URL.Host, JoinerName: peer})
}

// Join joins a cluster with the given token.
func (s CloudService) Join(ctx context.Context, joinConfig JoinConfig) error {
	return s.client.JoinCluster(ctx, s.name, util.CanonicalNetworkAddress(s.address, s.port), joinConfig.Token, nil)
}

// RequestJoin sends the signal to initiate a join to the remote system, or timeout after a maximum of 5 min.
func (s CloudService) RequestJoin(ctx context.Context, secret string, name string, joinConfig types.ServicesPut) error {
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
		c, err = s.client.RemoteClient(util.CanonicalNetworkAddress(joinConfig.Address, CloudPort))
		if err != nil {
			return err
		}

		c, err = cloudClient.UseAuthProxy(c, secret, types.MicroCloud)
		if err != nil {
			return err
		}
	}

	return client.JoinServices(ctx, c, joinConfig)
}

// RemoteClusterMembers returns a map of cluster member names and addresses from the MicroCloud at the given address, authenticated with the given secret.
func (s CloudService) RemoteClusterMembers(ctx context.Context, secret string, address string) (map[string]string, error) {
	client, err := s.client.RemoteClient(util.CanonicalNetworkAddress(address, CloudPort))
	if err != nil {
		return nil, err
	}

	client, err = cloudClient.UseAuthProxy(client, secret, types.MicroCloud)
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
