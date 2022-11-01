package service

import (
	"context"
	"fmt"

	"github.com/canonical/microcluster/microcluster"
	"github.com/lxc/lxd/lxd/revert"
	"github.com/lxc/lxd/lxd/util"
	"github.com/lxc/lxd/shared/api"

	"github.com/canonical/microcloud/microcloud/client"
)

// LXDService is a LXD service.
type LXDService struct {
	m *microcluster.MicroCluster

	name    string
	address string
	port    int
}

// NewLXDService creates a new LXD service with a client attached.
func NewLXDService(ctx context.Context, name string, addr string, cloudDir string) (*LXDService, error) {
	client, err := microcluster.App(ctx, cloudDir, false, false)
	if err != nil {
		return nil, err
	}

	return &LXDService{
		m:       client,
		name:    name,
		address: addr,
		port:    LXDPort,
	}, nil
}

// client returns a client to the LXD unix socket.
func (s LXDService) client() (*client.LXDClient, error) {
	c, err := s.m.LocalClient()
	if err != nil {
		return nil, err
	}

	return client.NewLXDClient(c), nil
}

// Bootstrap bootstraps the LXD daemon on the default port.
func (s LXDService) Bootstrap() error {
	addr := util.CanonicalNetworkAddress(s.address, s.port)

	server := api.ServerPut{Config: map[string]any{"core.https_address": addr, "cluster.https_address": addr}}
	storage := api.StoragePoolsPost{Name: "local", Driver: "zfs"}
	network := api.NetworksPost{NetworkPut: api.NetworkPut{Config: map[string]string{"bridge.mode": "fan"}}, Name: "lxdfan0", Type: "bridge"}
	devices := map[string]map[string]string{
		"root": {"path": "/", "pool": "local", "type": "disk"},
		"eth0": {"name": "eth0", "network": "lxdfan0", "type": "nic"},
	}

	profile := api.ProfilesPost{ProfilePut: api.ProfilePut{Devices: devices}, Name: "default"}
	initData := api.InitLocalPreseed{
		ServerPut:    server,
		StoragePools: []api.StoragePoolsPost{storage},
		Networks:     []api.InitNetworksProjectPost{{NetworksPost: network}},
		Profiles:     []api.ProfilesPost{profile},
	}

	revert := revert.New()
	defer revert.Fail()

	client, err := s.client()
	if err != nil {
		return err
	}

	revertFunc, err := initDataNodeApply(client, initData)
	if err != nil {
		return fmt.Errorf("Failed to initialize local LXD: %w", err)
	}

	revert.Add(revertFunc)

	currentCluster, etag, err := client.GetCluster()
	if err != nil {
		return fmt.Errorf("Failed to retrieve current cluster config: %w", err)
	}

	if currentCluster.Enabled {
		return fmt.Errorf("This LXD server is already clustered")
	}

	err = client.UpdateCluster(api.ClusterPut{Cluster: api.Cluster{ServerName: s.name, Enabled: true}}, etag)
	if err != nil {
		return fmt.Errorf("Failed to enable clustering on local LXD: %w", err)
	}

	revert.Success()

	return nil
}

// Join joins a cluster with the given token.
func (s LXDService) Join(token string) error {
	config, err := s.configFromToken(token)
	if err != nil {
		return err
	}

	client, err := s.client()
	if err != nil {
		return err
	}

	err = client.UpdateCluster(*config, "")
	if err != nil {
		return fmt.Errorf("Failed to join cluster: %w", err)
	}

	return nil
}

// IssueToken issues a token for the given peer.
func (s LXDService) IssueToken(peer string) (string, error) {
	client, err := s.client()
	if err != nil {
		return "", err
	}

	joinToken, err := client.CreateClusterMember(api.ClusterMembersPost{ServerName: peer})
	if err != nil {
		return "", err
	}

	return joinToken.String(), nil
}

// Type returns the type of Service.
func (s LXDService) Type() ServiceType {
	return LXD
}

// Name returns the name of this Service instance.
func (s LXDService) Name() string {
	return s.name
}

// Address returns the address of this Service instance.
func (s LXDService) Address() string {
	return s.address
}

// Port returns the port of this Service instance.
func (s LXDService) Port() int {
	return s.port
}
