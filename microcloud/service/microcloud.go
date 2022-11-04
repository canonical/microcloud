package service

import (
	"context"
	"time"

	"github.com/canonical/microcluster/config"
	"github.com/canonical/microcluster/microcluster"
	"github.com/canonical/microcluster/rest"
	"github.com/lxc/lxd/lxd/util"

	"github.com/canonical/microcloud/microcloud/api"
)

// CloudService is a MicroCloud service.
type CloudService struct {
	client *microcluster.MicroCluster

	name    string
	address string
	port    int
}

// NewCloudService creates a new MicroCloud service with a client attached.
func NewCloudService(ctx context.Context, name string, addr string, dir string, verbose bool, debug bool) (*CloudService, error) {
	client, err := microcluster.App(ctx, dir, verbose, debug)
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
func (s *CloudService) StartCloud(service *ServiceHandler) error {
	endpoints := []rest.Endpoint{
		api.CephClusterCmd,
		api.CephControlCmd,
		api.CephTokensCmd,

		api.LXDProxy,
		api.CephProxy,
	}

	return s.client.Start(endpoints, nil, &config.Hooks{
		OnBootstrap: service.Bootstrap,
		OnStart:     service.Start,
	})
}

// Bootstrap bootstraps the MicroCloud daemon on the default port.
func (s CloudService) Bootstrap() error {
	return s.client.NewCluster(s.name, util.CanonicalNetworkAddress(s.address, s.port), time.Second*30)
}

// IssueToken issues a token for the given peer.
func (s CloudService) IssueToken(peer string) (string, error) {
	return s.client.NewJoinToken(peer)
}

// Join joins a cluster with the given token.
func (s CloudService) Join(token string) error {
	return s.client.JoinCluster(s.name, util.CanonicalNetworkAddress(s.address, s.port), token, time.Second*30)
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
