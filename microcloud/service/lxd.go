package service

import (
	"context"
	"fmt"

	"github.com/canonical/microcluster/microcluster"
	"github.com/lxc/lxd/client"
	"github.com/lxc/lxd/lxd/util"
	"github.com/lxc/lxd/shared"
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
func (s LXDService) client() (lxd.InstanceServer, error) {
	c, err := s.m.LocalClient()
	if err != nil {
		return nil, err
	}

	return client.NewLXDClient(s.m.FileSystem.ControlSocket().URL.Host, c)
}

// Bootstrap bootstraps the LXD daemon on the default port.
func (s LXDService) Bootstrap() error {
	addr := util.CanonicalNetworkAddress(s.address, s.port)
	server := api.ServerPut{Config: map[string]any{"core.https_address": addr, "cluster.https_address": addr}}
	client, err := s.client()
	if err != nil {
		return err
	}

	currentServer, etag, err := client.GetServer()
	if err != nil {
		return fmt.Errorf("Failed to retrieve current server configuration: %w", err)
	}

	// Prepare the update.
	newServer := api.ServerPut{}
	err = shared.DeepCopy(currentServer.Writable(), &newServer)
	if err != nil {
		return fmt.Errorf("Failed to copy server configuration: %w", err)
	}

	for k, v := range server.Config {
		newServer.Config[k] = fmt.Sprintf("%v", v)
	}

	// Apply it.
	err = client.UpdateServer(newServer, etag)
	if err != nil {
		return fmt.Errorf("Failed to update server configuration: %w", err)
	}

	network := api.NetworksPost{
		NetworkPut: api.NetworkPut{Config: map[string]string{"bridge.mode": "fan"}},
		Name:       "lxdfan0",
		Type:       "bridge",
	}

	err = client.CreateNetwork(network)
	if err != nil {
		return err
	}

	currentCluster, etag, err := client.GetCluster()
	if err != nil {
		return fmt.Errorf("Failed to retrieve current cluster config: %w", err)
	}

	if currentCluster.Enabled {
		return fmt.Errorf("This LXD server is already clustered")
	}

	op, err := client.UpdateCluster(api.ClusterPut{Cluster: api.Cluster{ServerName: s.name, Enabled: true}}, etag)
	if err != nil {
		return fmt.Errorf("Failed to enable clustering on local LXD: %w", err)
	}

	err = op.Wait()
	if err != nil {
		return fmt.Errorf("Failed to initialize cluster: %w", err)
	}

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

	op, err := client.UpdateCluster(*config, "")
	if err != nil {
		return fmt.Errorf("Failed to join cluster: %w", err)
	}

	err = op.Wait()
	if err != nil {
		return fmt.Errorf("Failed to configure cluster :%w", err)
	}

	return nil
}

// IssueToken issues a token for the given peer.
func (s LXDService) IssueToken(peer string) (string, error) {
	client, err := s.client()
	if err != nil {
		return "", err
	}

	op, err := client.CreateClusterMember(api.ClusterMembersPost{ServerName: peer})
	if err != nil {
		return "", err
	}

	opAPI := op.Get()
	joinToken, err := opAPI.ToClusterJoinToken()
	if err != nil {
		return "", fmt.Errorf("Failed converting token operation to join token: %w", err)
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

// AddPendingPools adds pending Ceph storage pools for each of the target peers.
func (s *LXDService) AddPendingPools(targets []string) error {
	c, err := s.client()
	if err != nil {
		return err
	}

	for _, target := range targets {
		err = c.UseTarget(target).CreateStoragePool(api.StoragePoolsPost{
			Name:   "remote",
			Driver: "ceph",
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// Configure sets up the LXD storage pool (either remote ceph or local zfs), and adds the root and network devices to
// the default profile.
func (s *LXDService) Configure(addPool bool) error {
	profile := api.ProfilesPost{ProfilePut: api.ProfilePut{Devices: map[string]map[string]string{}}, Name: "default"}
	c, err := s.client()
	if err != nil {
		return err
	}

	if addPool {
		profile.Devices["root"] = map[string]string{"path": "/", "pool": "remote", "type": "disk"}
		storage := api.StoragePoolsPost{Name: "remote", Driver: "ceph"}
		err = c.CreateStoragePool(storage)
		if err != nil {
			return err
		}
	}

	profile.Devices["eth0"] = map[string]string{"name": "eth0", "network": "lxdfan0", "type": "nic"}
	profiles, err := c.GetProfileNames()
	if err != nil {
		return err
	}
	if !shared.StringInSlice(profile.Name, profiles) {
		err = c.CreateProfile(profile)
	} else {
		err = c.UpdateProfile("default", profile.ProfilePut, "")
	}

	if err != nil {
		return err
	}

	return nil
}
