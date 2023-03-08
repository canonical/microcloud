package service

import (
	"context"
	"strconv"
	"time"

	"github.com/canonical/microcluster/config"
	"github.com/canonical/microcluster/microcluster"
	"github.com/canonical/microcluster/rest"
	"github.com/lxc/lxd/lxd/util"

	"github.com/canonical/microcloud/microcloud/mdns"
)

// CloudService is a MicroCloud service.
type CloudService struct {
	client *microcluster.MicroCluster

	name    string
	address string
	port    int
}

// JoinConfig represents configuration for cluster joining.
type JoinConfig struct {
	Token     string
	LXDConfig []api.ClusterMemberConfigKey
}

// NewCloudService creates a new MicroCloud service with a client attached.
func NewCloudService(ctx context.Context, name string, addr string, dir string, verbose bool, debug bool) (*CloudService, error) {
	client, err := microcluster.App(ctx, microcluster.Args{StateDir: dir, ListenPort: strconv.Itoa(CloudPort)})
	if err != nil {
		return nil, err
	}

	return &CloudService{
		client:  client,
		name:    name,
		address: addr,
		port:    CloudPort,
	}, nil
}

// StartCloud launches the MicroCloud daemon with the appropriate hooks.
func (s *CloudService) StartCloud(service *ServiceHandler, endpoints []rest.Endpoint) error {
	return s.client.Start(endpoints, nil, &config.Hooks{
		OnBootstrap: service.Bootstrap,
		OnJoin:      service.Bootstrap,
		OnStart:     service.Start,
	})
}

// Bootstrap bootstraps the MicroCloud daemon on the default port.
func (s CloudService) Bootstrap() error {
	return s.client.NewCluster(s.name, util.CanonicalNetworkAddress(s.address, s.port), time.Second*120)
}

// IssueToken issues a token for the given peer.
func (s CloudService) IssueToken(peer string) (string, error) {
	return s.client.NewJoinToken(peer)
}

// Join joins a cluster with the given token.
func (s CloudService) Join(joinConfig JoinConfig) error {
	return s.client.JoinCluster(s.name, util.CanonicalNetworkAddress(s.address, s.port), joinConfig.Token, 5*time.Minute)
}

// ClusterMembers returns a map of cluster member names and addresses.
func (s CloudService) ClusterMembers() (map[string]string, error) {
	client, err := s.client.LocalClient()
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
func (s CloudService) Type() ServiceType {
	return MicroCloud
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
func (s CloudService) Port() int {
	return s.port
}
