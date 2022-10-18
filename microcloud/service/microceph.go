package service

import (
	"context"
	"time"

	"github.com/canonical/microcluster/microcluster"
	"github.com/lxc/lxd/lxd/util"
)

// CephService is a MicroCeph service.
type CephService struct {
	Client *microcluster.MicroCluster

	name    string
	address string
	port    int
}

// NewCephService creates a new MicroCeph service with a client attached.
func NewCephService(ctx context.Context, name string, addr string, dir string, verbose bool, debug bool) (*CephService, error) {
	client, err := microcluster.App(ctx, dir, verbose, debug)
	if err != nil {
		return nil, err
	}

	return &CephService{
		Client:  client,
		name:    name,
		address: addr,
		port:    CephPort,
	}, nil
}

// Bootstrap bootstraps the MicroCeph daemon on the default port.
func (s CephService) Bootstrap() error {
	return s.Client.NewCluster(s.name, util.CanonicalNetworkAddress(s.address, s.port), time.Second*30)
}

// IssueToken issues a token for the given peer.
func (s CephService) IssueToken(peer string) (string, error) {
	return s.Client.NewJoinToken(peer)
}

// Join joins a cluster with the given token.
func (s CephService) Join(token string) error {
	return s.Client.JoinCluster(s.name, util.CanonicalNetworkAddress(s.address, s.port), token, time.Second*30)
}

// Type returns the type of Service.
func (s CephService) Type() ServiceType {
	return MicroCeph
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
