package service

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcluster/microcluster"
	"github.com/lxc/lxd/client"
	"github.com/lxc/lxd/lxd/util"
	"github.com/lxc/lxd/shared"
	"github.com/lxc/lxd/shared/api"
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
	client, err := microcluster.App(ctx, microcluster.Args{StateDir: cloudDir})
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

	return lxd.ConnectLXDUnix(s.m.FileSystem.ControlSocket().URL.Host, &lxd.ConnectionArgs{
		HTTPClient: c.Client.Client,
		Proxy: func(r *http.Request) (*url.URL, error) {
			if !strings.HasPrefix(r.URL.Path, "/1.0/services/lxd") {
				r.URL.Path = "/1.0/services/lxd" + r.URL.Path
			}

			return shared.ProxyFromEnvironment(r)
		},
	})
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

// ClusterMembers returns a map of cluster member names and addresses.
func (s LXDService) ClusterMembers() (map[string]string, error) {
	client, err := s.client()
	if err != nil {
		return nil, err
	}

	members, err := client.GetClusterMembers()
	if err != nil {
		return nil, err
	}

	genericMembers := make(map[string]string, len(members))
	for _, member := range members {
		genericMembers[member.ServerName] = member.URL
	}

	return genericMembers, nil
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

// AddLocalPools adds local pending zfs storage pools on the target peers, with the given source disks.
func (s *LXDService) AddLocalPools(disks map[string]string) error {
	c, err := s.client()
	if err != nil {
		return err
	}

	for target, source := range disks {
		err := c.UseTarget(target).CreateStoragePool(api.StoragePoolsPost{
			Name:   "local",
			Driver: "zfs",
			StoragePoolPut: api.StoragePoolPut{
				Config: map[string]string{"source": source},
			},
		})

		if err != nil {
			return err
		}
	}

	return nil
}

// AddRemotePools adds pending Ceph storage pools for each of the target peers.
func (s *LXDService) AddRemotePools(targets []string) error {
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

// GetResources returns the system resources for the LXD target.
func (s *LXDService) GetResources(target string) (*api.Resources, error) {
	c, err := s.client()
	if err != nil {
		return nil, err
	}

	return c.UseTarget(target).GetServerResources()
}

// WipeDisk wipes the disk with the given device ID>
func (s *LXDService) WipeDisk(target string, deviceID string) error {
	c, err := s.m.LocalClient()
	if err != nil {
		return err
	}

	return client.WipeDisk(context.Background(), c.UseTarget(target), deviceID)
}

// Configure sets up the LXD storage pool (either remote ceph or local zfs), and adds the root and network devices to
// the default profile.
func (s *LXDService) Configure(addLocalPool bool, addRemotePool bool) error {
	profile := api.ProfilesPost{ProfilePut: api.ProfilePut{Devices: map[string]map[string]string{}}, Name: "default"}
	c, err := s.client()
	if err != nil {
		return err
	}

	if addRemotePool {
		storage := api.StoragePoolsPost{Name: "remote", Driver: "ceph"}
		err = c.CreateStoragePool(storage)
		if err != nil {
			return err
		}

		profile.Devices["root"] = map[string]string{"path": "/", "pool": "remote", "type": "disk"}
	}

	if addLocalPool {
		storage := api.StoragePoolsPost{Name: "local", Driver: "zfs"}
		err = c.CreateStoragePool(storage)
		if err != nil {
			return err
		}

		if profile.Devices["root"] == nil {
			profile.Devices["root"] = map[string]string{"path": "/", "pool": "local", "type": "disk"}
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
