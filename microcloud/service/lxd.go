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

	"github.com/canonical/microcluster/microcluster"
	"github.com/lxc/lxd/client"
	"github.com/lxc/lxd/lxd/util"
	"github.com/lxc/lxd/shared"
	"github.com/lxc/lxd/shared/api"

	"github.com/canonical/microcloud/microcloud/api/types"
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
func (s LXDService) client(secret string) (lxd.InstanceServer, error) {
	c, err := s.m.LocalClient()
	if err != nil {
		return nil, err
	}

	return lxd.ConnectLXDUnix(s.m.FileSystem.ControlSocket().URL.Host, &lxd.ConnectionArgs{
		HTTPClient: c.Client.Client,
		Proxy: func(r *http.Request) (*url.URL, error) {
			r.Header.Set("X-MicroCloud-Auth", secret)
			if !strings.HasPrefix(r.URL.Path, "/1.0/services/lxd") {
				r.URL.Path = "/1.0/services/lxd" + r.URL.Path
			}

			return shared.ProxyFromEnvironment(r)
		},
	})
}

// remoteClient returns an https client for the given address:port.
func (s LXDService) remoteClient(secret string, address string, port int) (lxd.InstanceServer, error) {
	c, err := s.m.RemoteClient(util.CanonicalNetworkAddress(address, port))
	if err != nil {
		return nil, err
	}

	remoteURL := c.URL()
	client, err := lxd.ConnectLXD(remoteURL.String(), &lxd.ConnectionArgs{
		HTTPClient:         c.Client.Client,
		InsecureSkipVerify: true,
		Proxy: func(r *http.Request) (*url.URL, error) {
			r.Header.Set("X-MicroCloud-Auth", secret)
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
	client, err := s.client("")
	if err != nil {
		return err
	}

	currentServer, etag, err := client.GetServer()
	if err != nil {
		return fmt.Errorf("Failed to retrieve current server configuration: %w", err)
	}

	// Prepare the update.
	addr := util.CanonicalNetworkAddress(s.address, s.port)

	newServer := currentServer.Writable()
	newServer.Config["core.https_address"] = addr
	newServer.Config["cluster.https_address"] = addr

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
			Description: "Default Ubuntu fan powered bridge",
		},
		Name: "lxdfan0",
		Type: "bridge",
	}

	err = client.CreateNetwork(network)
	if err != nil {
		return err
	}

	// Enable clustering.
	currentCluster, etag, err := client.GetCluster()
	if err != nil {
		return fmt.Errorf("Failed to retrieve current cluster config: %w", err)
	}

	if currentCluster.Enabled {
		return fmt.Errorf("This LXD server is already clustered")
	}

	cluster := api.ClusterPut{
		Cluster: api.Cluster{
			ServerName: s.name,
			Enabled:    true,
		},
	}

	op, err := client.UpdateCluster(cluster, etag)
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
func (s LXDService) Join(joinConfig JoinConfig) error {
	config, err := s.configFromToken(joinConfig.Token)
	if err != nil {
		return err
	}

	config.Cluster.MemberConfig = joinConfig.LXDConfig
	client, err := s.client("")
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
	client, err := s.client("")
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
	client, err := s.client("")
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
func (s LXDService) Type() types.ServiceType {
	return types.LXD
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

// AddLocalPools adds local zfs storage pools on the target peers, with the given source disks.
func (s *LXDService) AddLocalPool(source string, wipe bool) error {
	c, err := s.client("")
	if err != nil {
		return err
	}

	config := map[string]string{"source": source}
	if wipe {
		config["source.wipe"] = "true"
	}

	return c.CreateStoragePool(api.StoragePoolsPost{
		Name:   "local",
		Driver: "zfs",
		StoragePoolPut: api.StoragePoolPut{
			Config:      config,
			Description: "Local storage on ZFS",
		},
	})
}

// AddLocalVolumes creates the default local storage volumes for a new LXD service.
func (s *LXDService) AddLocalVolumes(target string, secret string) error {
	c, err := s.client(secret)
	if err != nil {
		return err
	}

	if s.Name() != target {
		c = c.UseTarget(target)
	}

	err = c.CreateStoragePoolVolume("local", api.StorageVolumesPost{Name: "images", Type: "custom"})
	if err != nil {
		return err
	}

	err = c.CreateStoragePoolVolume("local", api.StorageVolumesPost{Name: "backups", Type: "custom"})
	if err != nil {
		return err
	}

	server, _, err := c.GetServer()
	if err != nil {
		return err
	}

	newServer := server.Writable()
	newServer.Config["storage.backups_volume"] = "local/backups"
	newServer.Config["storage.images_volume"] = "local/images"
	err = c.UpdateServer(newServer, "")
	if err != nil {
		return err
	}

	return nil
}

// AddRemotePools adds pending Ceph storage pools for each of the target peers.
func (s *LXDService) AddRemotePools(targets map[string]string) error {
	if len(targets) == 0 {
		return nil
	}

	for target, secret := range targets {
		c, err := s.client(secret)
		if err != nil {
			return err
		}

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
func (s *LXDService) HasExtension(useRemote bool, target string, address string, secret string, apiExtension string) (bool, error) {
	var err error
	var client lxd.InstanceServer
	if !useRemote {
		client, err = s.client(secret)
		if err != nil {
			return false, err
		}

		if target != s.Name() {
			client = client.UseTarget(target)
		}
	} else {
		client, err = s.remoteClient(secret, address, CloudPort)
		if err != nil {
			return false, err
		}
	}

	return client.HasExtension(apiExtension), nil
}

// GetResources returns the system resources for the LXD target.
// As we cannot guarantee that LXD is available on this machine, the request is
// forwarded through MicroCloud on via the ListenPort argument.
func (s *LXDService) GetResources(useRemote bool, target string, address string, secret string) (*api.Resources, error) {
	var err error
	var client lxd.InstanceServer
	if !useRemote {
		client, err = s.client(secret)
		if err != nil {
			return nil, err
		}

		if target != s.Name() {
			client = client.UseTarget(target)
		}
	} else {
		client, err = s.remoteClient(secret, address, CloudPort)
		if err != nil {
			return nil, err
		}
	}

	return client.GetServerResources()
}

// Configure sets up the LXD storage pool (either remote ceph or local zfs), and adds the root and network devices to
// the default profile.
func (s *LXDService) Configure(bootstrap bool, localPoolTargets map[string]string, remotePoolTargets map[string]string, ovnConfig string, ovnTargets map[string]string) error {
	c, err := s.client("")
	if err != nil {
		return err
	}

	for peer, secret := range localPoolTargets {
		err = s.AddLocalVolumes(peer, secret)
		if err != nil {
			return err
		}
	}

	for peer, secret := range ovnTargets {
		err = s.SetConfig(peer, secret, map[string]string{"network.ovn.northbound_connection": ovnConfig})
		if err != nil {
			return err
		}
	}

	if bootstrap {
		err = s.SetConfig(s.Name(), "", map[string]string{"network.ovn.northbound_connection": ovnConfig})
		if err != nil {
			return err
		}

		profile := api.ProfilesPost{ProfilePut: api.ProfilePut{Devices: map[string]map[string]string{}}, Name: "default"}
		if len(localPoolTargets) > 0 {
			err = s.AddLocalVolumes(s.Name(), "")
			if err != nil {
				return err
			}

			profile.Devices["root"] = map[string]string{"path": "/", "pool": "local", "type": "disk"}
		}

		err = s.AddRemotePools(remotePoolTargets)
		if err != nil {
			return err
		}

		storage := api.StoragePoolsPost{
			Name:   "remote",
			Driver: "ceph",
			StoragePoolPut: api.StoragePoolPut{
				Config: map[string]string{
					"ceph.rbd.du":       "false",
					"ceph.rbd.features": "layering,striping,exclusive-lock,object-map,fast-diff,deep-flatten",
				},
				Description: "Distributed storage on Ceph",
			},
		}

		err = c.CreateStoragePool(storage)
		if err != nil {
			return err
		}

		profile.Devices["root"] = map[string]string{"path": "/", "pool": "remote", "type": "disk"}
		profile.Devices["eth0"] = map[string]string{"name": "eth0", "network": "lxdfan0", "type": "nic"}
		profiles, err := c.GetProfileNames()
		if err != nil {
			return err
		}
		if !shared.StringInSlice(profile.Name, profiles) {
			err = c.CreateProfile(profile)
		} else {
			err = c.UpdateProfile(profile.Name, profile.ProfilePut, "")
		}

		if err != nil {
			return err
		}
	}

	return nil
}

// SetConfig applies the new config key/value pair to the given target.
func (s *LXDService) SetConfig(target string, secret string, config map[string]string) error {
	c, err := s.client(secret)
	if err != nil {
		return err
	}

	if s.Name() != target {
		c = c.UseTarget(target)
	}

	server, _, err := c.GetServer()
	if err != nil {
		return err
	}

	newServer := server.Writable()
	for k, v := range config {
		newServer.Config[k] = v
	}

	return c.UpdateServer(newServer, "")
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
