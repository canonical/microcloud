package service

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcloud/microcloud/mdns"
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

// remoteClient returns an https client for the given address:port.
func (s LXDService) remoteClient(address string, port int) (lxd.InstanceServer, error) {
	c, err := s.m.RemoteClient(util.CanonicalNetworkAddress(address, port))
	if err != nil {
		return nil, err
	}

	remoteURL := c.URL()
	client, err := lxd.ConnectLXD(remoteURL.String(), &lxd.ConnectionArgs{
		HTTPClient:         c.Client.Client,
		InsecureSkipVerify: true,
		Proxy: func(r *http.Request) (*url.URL, error) {
			if !strings.HasPrefix(r.URL.Path, "/1.0/services/lxd") {
				r.URL.Path = "/1.0/services/lxd" + r.URL.Path
			}

			return shared.ProxyFromEnvironment(r)
		},
	})
	if err != nil {
		return nil, err
	}

	return client, nil
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

	// Setup networking.
	underlay, _, err := defaultGatewaySubnetV4()
	if err != nil {
		return fmt.Errorf("Couldn't determine Fan overlay subnet: %w", err)
	}

	underlaySize, _ := underlay.Mask.Size()
	if underlaySize != 16 && underlaySize != 24 {
		// Override to /16 as that will almost always lead to working Fan network.
		underlay.Mask = net.CIDRMask(16, 32)
		underlay.IP = underlay.IP.Mask(underlay.Mask)
	}

	network := api.NetworksPost{
		NetworkPut: api.NetworkPut{
			Config: map[string]string{
				"bridge.mode":         "fan",
				"fan.underlay_subnet": underlay.String(),
			},
		},
		Name: "lxdfan0",
		Type: "bridge",
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
func (s LXDService) Join(joinConfig mdns.JoinConfig) error {
	config, err := s.configFromToken(joinConfig.Token)
	if err != nil {
		return err
	}

	config.Cluster.MemberConfig = joinConfig.LXDConfig
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
			StoragePoolPut: api.StoragePoolPut{
				Config: map[string]string{
					"source": "lxd_remote",
				},
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// HasExtension checks if the server supports the API extension.
func (s *LXDService) HasExtension(useRemote bool, target string, address string, apiExtension string) (bool, error) {
	var err error
	var client lxd.InstanceServer
	if !useRemote {
		client, err = s.client()
		if err != nil {
			return false, err
		}

		if target != s.Name() {
			client = client.UseTarget(target)
		}
	} else {
		client, err = s.remoteClient(address, CloudPort)
		if err != nil {
			return false, err
		}
	}

	return client.HasExtension(apiExtension), nil
}

// GetResources returns the system resources for the LXD target.
// As we cannot guarantee that LXD is available on this machine, the request is
// forwarded through MicroCloud on via the ListenPort argument.
func (s *LXDService) GetResources(useRemote bool, target string, address string) (*api.Resources, error) {
	var err error
	var client lxd.InstanceServer
	if !useRemote {
		client, err = s.client()
		if err != nil {
			return nil, err
		}

		if target != s.Name() {
			client = client.UseTarget(target)
		}
	} else {
		client, err = s.remoteClient(address, CloudPort)
		if err != nil {
			return nil, err
		}
	}

	return client.GetServerResources()
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
		storage := api.StoragePoolsPost{
			Name:   "remote",
			Driver: "ceph",
			StoragePoolPut: api.StoragePoolPut{
				Config: map[string]string{
					"ceph.rbd.du":       "false",
					"ceph.rbd.features": "layering,striping,exclusive-lock,object-map,fast-diff,deep-flatten",
				},
			},
		}
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

// defaultGatewaySubnetV4 returns subnet of default gateway interface.
func defaultGatewaySubnetV4() (*net.IPNet, string, error) {
	file, err := os.Open("/proc/net/route")
	if err != nil {
		return nil, "", err
	}

	defer func() { _ = file.Close() }()

	ifaceName := ""

	scanner := bufio.NewReader(file)
	for {
		line, _, err := scanner.ReadLine()
		if err != nil {
			break
		}

		fields := strings.Fields(string(line))

		if fields[1] == "00000000" && fields[7] == "00000000" {
			ifaceName = fields[0]
			break
		}
	}

	if ifaceName == "" {
		return nil, "", fmt.Errorf("No default gateway for IPv4")
	}

	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return nil, "", err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, "", err
	}

	var subnet *net.IPNet

	for _, addr := range addrs {
		addrIP, addrNet, err := net.ParseCIDR(addr.String())
		if err != nil {
			return nil, "", err
		}

		if addrIP.To4() == nil {
			continue
		}

		if subnet != nil {
			return nil, "", fmt.Errorf("More than one IPv4 subnet on default interface")
		}

		subnet = addrNet
	}

	if subnet == nil {
		return nil, "", fmt.Errorf("No IPv4 subnet on default interface")
	}

	return subnet, ifaceName, nil
}
