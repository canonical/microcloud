package service

import (
	"context"
	"time"

	"github.com/canonical/microcluster/microcluster"
	"github.com/lxc/lxd/lxd/util"

	"github.com/canonical/microcloud/microcloud/client"
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
	client, err := microcluster.App(ctx, cloudDir, false, false)
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

// client returns a client to the Ceph unix socket.
func (s CephService) Client() (*client.CephClient, error) {
	c, err := s.m.LocalClient()
	if err != nil {
		return nil, err
	}

	return client.NewCephClient(c), nil
}

// Bootstrap bootstraps the MicroCeph daemon on the default port.
func (s CephService) Bootstrap() error {
	client, err := s.Client()
	if err != nil {
		return err
	}

	return client.NewCluster(s.name, util.CanonicalNetworkAddress(s.address, s.port), time.Second*30)
}

// IssueToken issues a token for the given peer.
func (s CephService) IssueToken(peer string) (string, error) {
	client, err := s.Client()
	if err != nil {
		return "", err
	}

	return client.NewJoinToken(peer)
}

// Join joins a cluster with the given token.
func (s CephService) Join(token string) error {
	client, err := s.Client()
	if err != nil {
		return err
	}

	return client.JoinCluster(s.name, util.CanonicalNetworkAddress(s.address, s.port), token, time.Second*30)
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
