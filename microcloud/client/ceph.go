package client

import (
	"context"
	"fmt"
	"time"

	cephTypes "github.com/canonical/microceph/microceph/api/types"
	"github.com/canonical/microcluster/client"
	"github.com/canonical/microcluster/rest/types"
	"github.com/lxc/lxd/shared/api"
)

// CephClient is a Client with helpers for proxying MicroCeph from the MicroCloud daemon.
type CephClient struct {
	*client.Client
}

// NewCephClient returns a CephClient with the underlying Client.
func NewCephClient(c *client.Client) *CephClient {
	return &CephClient{Client: c}
}

// ControlPost represents the internal Control type passed to the MicroCeph control API.
type ControlPost struct {
	Bootstrap bool           `json:"bootstrap" yaml:"bootstrap"`
	JoinToken string         `json:"join_token" yaml:"join_token"`
	Address   types.AddrPort `json:"address" yaml:"address"`
	Name      string         `json:"name" yaml:"name"`
}

// TokenRecord represents the internal TokenRecord type passed to the MicroCeph public API.
type TokenRecord struct {
	Name string `json:"name" yaml:"name"`
}

// ClusterMember represents the internal ClusterMember type passed to the MicroCeph public API.
type ClusterMember struct {
	ClusterMemberLocal
	Role          string    `json:"role" yaml:"role"`
	SchemaVersion int       `json:"schema_version" yaml:"schema_version"`
	LastHeartbeat time.Time `json:"last_heartbeat" yaml:"last_heartbeat"`
	Status        string    `json:"status" yaml:"status"`
	Secret        string    `json:"secret" yaml:"secret"`
}

// ClusterMemberLocal represents the internal ClusterMemberLocal type passed to the MicroCeph public API.
type ClusterMemberLocal struct {
	Name        string                `json:"name" yaml:"name"`
	Address     types.AddrPort        `json:"address" yaml:"address"`
	Certificate types.X509Certificate `json:"certificate" yaml:"certificate"`
}

// NewCluster bootstrapps a brand new cluster with this daemon as its only member.
func (c *CephClient) NewCluster(name string, address string, timeout time.Duration) error {
	addr, err := types.ParseAddrPort(address)
	if err != nil {
		return fmt.Errorf("Received invalid address %q: %w", address, err)
	}

	if timeout == 0 {
		return c.Query(context.TODO(), "POST", api.NewURL().Path("services", "microceph", "cluster", "control"), ControlPost{Bootstrap: true, Address: addr, Name: name}, nil)
	}

	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()

	return c.Query(ctx, "POST", api.NewURL().Path("services", "microceph", "cluster", "control"), ControlPost{Bootstrap: true, Address: addr, Name: name}, nil)
}

// JoinCluster joins an existing cluster with a join token supplied by an existing cluster member.
func (c *CephClient) JoinCluster(name string, address string, token string, timeout time.Duration) error {
	addr, err := types.ParseAddrPort(address)
	if err != nil {
		return fmt.Errorf("Received invalid address %q: %w", address, err)
	}

	if timeout == 0 {
		return c.Query(context.TODO(), "POST", api.NewURL().Path("services", "microceph", "cluster", "control"), ControlPost{JoinToken: token, Address: addr, Name: name}, nil)
	}

	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()

	return c.Query(ctx, "POST", api.NewURL().Path("services", "microceph", "cluster", "control"), ControlPost{JoinToken: token, Address: addr, Name: name}, nil)
}

// NewJoinToken creates and records a new join token containing all the necessary credentials for joining a cluster.
// Join tokens are tied to the server certificate of the joining node, and will be deleted once the node has joined the
// cluster.
func (c *CephClient) NewJoinToken(name string) (string, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*5)
	defer cancel()

	var token string
	err := c.Query(ctx, "POST", api.NewURL().Path("services", "microceph", "cluster", "1.0", "tokens"), TokenRecord{Name: name}, &token)
	if err != nil {
		return "", err
	}

	return token, nil
}

// GetClusterMembers returns the database record of cluster members.
func (c *CephClient) GetClusterMembers(ctx context.Context) ([]ClusterMember, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	var members []ClusterMember
	err := c.Query(ctx, "GET", api.NewURL().Path("services", "microceph", "cluster", "1.0", "cluster"), nil, &members)
	if err != nil {
		return nil, err
	}

	return members, nil
}

// AddDisk requests Ceph sets up a new OSD.
func (c *CephClient) AddDisk(ctx context.Context, data *cephTypes.DisksPost) error {
	queryCtx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	err := c.Query(queryCtx, "POST", api.NewURL().Path("services", "microceph", "1.0", "disks"), data, nil)
	if err != nil {
		return fmt.Errorf("Failed adding new disk: %w", err)
	}

	return nil
}

// GetDisks returns the list of configured disks.
func (c *CephClient) GetDisks(ctx context.Context) (cephTypes.Disks, error) {
	queryCtx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	disks := cephTypes.Disks{}

	err := c.Query(queryCtx, "GET", api.NewURL().Path("services", "microceph", "1.0", "disks"), nil, &disks)
	if err != nil {
		return nil, fmt.Errorf("Failed listing disks: %w", err)
	}

	return disks, nil
}

// GetResources returns the list of storage devices on the system.
func (c *CephClient) GetResources(ctx context.Context) (*api.ResourcesStorage, error) {
	queryCtx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	storage := api.ResourcesStorage{}

	err := c.Query(queryCtx, "GET", api.NewURL().Path("services", "microceph", "1.0", "resources"), nil, &storage)
	if err != nil {
		return nil, fmt.Errorf("Failed listing storage devices: %w", err)
	}

	return &storage, nil
}

// UseTarget returns a new client with the query "?target=name" set.
func (c *CephClient) UseTarget(name string) *CephClient {
	newClient := c.Client.UseTarget(name)

	return &CephClient{Client: newClient}
}
