package client

import (
	"context"
	"time"

	"github.com/canonical/microcluster/client"
	"github.com/lxc/lxd/shared/api"
)

// LXDClient is a Client with helpers for proxying LXD from the MicroCloud daemon.
type LXDClient struct {
	*client.Client
}

// NewLXDClient returns a LXDClient with the underlying Client.
func NewLXDClient(c *client.Client) *LXDClient {
	return &LXDClient{Client: c}
}

// GetServer returns the server status as a Server struct.
func (c *LXDClient) GetServer() (*api.Server, string, error) {
	server := api.Server{}
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	err := c.Query(ctx, "GET", api.NewURL().Path("services", "lxd", "1.0"), nil, &server)
	if err != nil {
		return nil, "", err
	}

	return &server, "", nil
}

// UpdateServer updates the server status to match the provided Server struct.
func (c *LXDClient) UpdateServer(server api.ServerPut, ETag string) error {
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	return c.Query(ctx, "PUT", api.NewURL().Path("services", "lxd", "1.0"), server, nil)
}

// GetStoragePoolNames returns the names of all storage pools.
func (c *LXDClient) GetStoragePoolNames() ([]string, error) {
	urls := []string{}
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	err := c.Query(ctx, "GET", api.NewURL().Path("services", "lxd", "1.0", "storage-pools"), nil, &urls)
	if err != nil {
		return nil, err
	}

	return urls, nil
}

// DeleteStoragePool deletes a storage pool.
func (c *LXDClient) DeleteStoragePool(name string) error {
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	return c.Query(ctx, "DELETE", api.NewURL().Path("services", "lxd", "1.0", "storage-pools", name), nil, nil)
}

// GetStoragePool returns a StoragePool entry for the provided pool name.
func (c *LXDClient) GetStoragePool(name string) (*api.StoragePool, string, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	pool := api.StoragePool{}
	err := c.Query(ctx, "GET", api.NewURL().Path("services", "lxd", "1.0", "storage-pools", name), nil, &pool)
	if err != nil {
		return nil, "", err
	}

	return &pool, "", nil
}

// UpdateStoragePool updates the pool to match the provided StoragePool struct.
func (c *LXDClient) UpdateStoragePool(name string, pool api.StoragePoolPut, ETag string) error {
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	return c.Query(ctx, "PUT", api.NewURL().Path("services", "lxd", "1.0", "storage-pools", name), pool, nil)
}

// CreateStoragePool defines a new storage pool using the provided StoragePool struct.
func (c *LXDClient) CreateStoragePool(pool api.StoragePoolsPost) error {
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	return c.Query(ctx, "POST", api.NewURL().Path("services", "lxd", "1.0", "storage-pools"), pool, nil)
}

// GetProfileNames returns a list of available profile names.
func (c *LXDClient) GetProfileNames() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	urls := []string{}
	err := c.Query(ctx, "GET", api.NewURL().Path("services", "lxd", "1.0", "profiles"), nil, &urls)
	if err != nil {
		return nil, err
	}

	return urls, nil
}

// GetProfile returns a Profile entry for the provided name.
func (c *LXDClient) GetProfile(name string) (*api.Profile, string, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	profile := api.Profile{}
	err := c.Query(ctx, "GET", api.NewURL().Path("services", "lxd", "1.0", "profiles", name), nil, &profile)
	if err != nil {
		return nil, "", err
	}

	return &profile, "", nil
}

// UpdateProfile updates the profile to match the provided Profile struct.
func (c *LXDClient) UpdateProfile(name string, profile api.ProfilePut, ETag string) error {
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	return c.Query(ctx, "PUT", api.NewURL().Path("services", "lxd", "1.0", "profiles", name), profile, nil)
}

// CreateProfile defines a new container profile.
func (c *LXDClient) CreateProfile(profile api.ProfilesPost) error {
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	return c.Query(ctx, "POST", api.NewURL().Path("services", "lxd", "1.0", "profiles"), profile, nil)
}

// DeleteProfile deletes a profile.
func (c *LXDClient) DeleteProfile(name string) error {
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	return c.Query(ctx, "DELETE", api.NewURL().Path("services", "lxd", "1.0", "profiles", name), nil, nil)
}

// GetNetwork returns a Network entry for the provided name.
func (c *LXDClient) GetNetwork(name string) (*api.Network, string, error) {
	network := api.Network{}
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	err := c.Query(ctx, "GET", api.NewURL().Path("services", "lxd", "1.0", "networks", name), nil, &network)
	if err != nil {
		return nil, "", err
	}

	return &network, "", nil
}

// DeleteNetwork deletes an existing network.
func (c *LXDClient) DeleteNetwork(name string) error {
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	return c.Query(ctx, "DELETE", api.NewURL().Path("services", "lxd", "1.0", "networks", name), nil, nil)
}

// UpdateNetwork updates the network to match the provided Network struct.
func (c *LXDClient) UpdateNetwork(name string, network api.NetworkPut, ETag string) error {
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	return c.Query(ctx, "PUT", api.NewURL().Path("services", "lxd", "1.0", "networks", name), network, nil)
}

// CreateNetwork defines a new network using the provided Network struct.
func (c *LXDClient) CreateNetwork(network api.NetworksPost) error {
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	return c.Query(ctx, "POST", api.NewURL().Path("services", "lxd", "1.0", "networks"), network, nil)
}

// GetCluster returns information about a cluster
//
// If this client is not trusted, the password must be supplied.
func (c *LXDClient) GetCluster() (*api.Cluster, string, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	cluster := api.Cluster{}
	err := c.Query(ctx, "GET", api.NewURL().Path("services", "lxd", "1.0", "cluster"), nil, &cluster)
	if err != nil {
		return nil, "", err
	}

	return &cluster, "", nil
}

// UpdateCluster requests to bootstrap a new cluster or join an existing one.
func (c *LXDClient) UpdateCluster(cluster api.ClusterPut, ETag string) error {
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	return c.Query(ctx, "PUT", api.NewURL().Path("services", "lxd", "1.0", "cluster"), cluster, nil)
}

// CreateClusterMember generates a join token to add a cluster member.
func (c *LXDClient) CreateClusterMember(cluster api.ClusterMembersPost) (*api.ClusterMemberJoinToken, error) {
	token := api.ClusterMemberJoinToken{}
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	err := c.Query(ctx, "POST", api.NewURL().Path("services", "lxd", "1.0", "cluster", "members"), cluster, &token)
	if err != nil {
		return nil, err
	}

	return &token, nil
}
