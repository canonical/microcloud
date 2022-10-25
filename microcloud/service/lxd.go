package service

import (
	"fmt"
	"path/filepath"

	"github.com/lxc/lxd/client"
	"github.com/lxc/lxd/lxd/revert"
	"github.com/lxc/lxd/lxd/util"
	"github.com/lxc/lxd/shared/api"
)

// LXDService is a LXD service.
type LXDService struct {
	dir string

	name    string
	address string
	port    int
}

// NewLXDService creates a new LXD service with a client attached.
func NewLXDService(name string, addr string, dir string) (*LXDService, error) {

	return &LXDService{
		dir:     dir,
		name:    name,
		address: addr,
		port:    LXDPort,
	}, nil
}

// client returns a client to the LXD unix socket.
func (s LXDService) client() (lxd.InstanceServer, error) {
	client, err := lxd.ConnectLXDUnix(filepath.Join(s.dir, "unix.socket"), nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to local LXD: %w", err)
	}

	return client, nil
}

// Bootstrap bootstraps the LXD daemon on the default port.
func (s LXDService) Bootstrap() error {
	addr := util.CanonicalNetworkAddress(s.address, s.port)

	server := api.ServerPut{Config: map[string]any{"core.https_address": addr, "cluster.https_address": addr}}
	rootDisk := map[string]map[string]string{"root": {"path": "/", "pool": "local", "type": "disk"}}
	profile := api.ProfilesPost{ProfilePut: api.ProfilePut{Devices: rootDisk}, Name: "default"}
	storage := api.StoragePoolsPost{Name: "local", Driver: "dir"}

	initData := initDataNode{
		ServerPut:    server,
		StoragePools: []api.StoragePoolsPost{storage},
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

	op, err := client.UpdateCluster(api.ClusterPut{Cluster: api.Cluster{ServerName: s.name, Enabled: true}}, etag)
	if err != nil {
		return fmt.Errorf("Failed to enable clustering on local LXD: %w", err)
	}

	err = op.Wait()
	if err != nil {
		return fmt.Errorf("Failed to configure cluster :%w", err)
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

	// Connect to existing cluster
	serverCert, err := util.LoadServerCert(s.dir)
	if err != nil {
		return err
	}

	err = SetupTrust(serverCert, config.ServerName, config.ClusterAddress, config.ClusterCertificate, token)
	if err != nil {
		return fmt.Errorf("Failed to setup trust relationship with cluster: %w", err)
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
