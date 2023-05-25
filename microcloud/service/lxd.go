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
	"github.com/canonical/microcloud/microcloud/mdns"
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
	newServer.Config["core.https_address"] = "[::]:8443"
	newServer.Config["cluster.https_address"] = addr

	// Apply it.
	err = client.UpdateServer(newServer, etag)
	if err != nil {
		return fmt.Errorf("Failed to update server configuration: %w", err)
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
func (s *LXDService) HasExtension(target string, address string, secret string, apiExtension string) (bool, error) {
	var err error
	var client lxd.InstanceServer
	if s.Name() == target {
		client, err = s.client(secret)
		if err != nil {
			return false, err
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
func (s *LXDService) GetResources(target string, address string, secret string) (*api.Resources, error) {
	var err error
	var client lxd.InstanceServer
	if s.Name() == target {
		client, err = s.client(secret)
		if err != nil {
			return nil, err
		}
	} else {
		client, err = s.remoteClient(secret, address, CloudPort)
		if err != nil {
			return nil, err
		}
	}

	return client.GetServerResources()
}

func (s LXDService) GetUplinkInterfaces(bootstrap bool, peers map[string]mdns.ServerInfo) (map[string][]api.Network, error) {
	clients := map[string]lxd.InstanceServer{}
	networks := map[string][]api.Network{}
	if bootstrap {
		client, err := s.client("")
		if err != nil {
			return nil, err
		}

		networks[s.Name()], err = client.GetNetworks()
		if err != nil {
			return nil, err
		}

		clients[s.Name()] = client
	}

	for _, info := range peers {
		client, err := s.remoteClient(info.AuthSecret, info.Address, CloudPort)
		if err != nil {
			return nil, err
		}

		networks[info.Name], err = client.GetNetworks()
		if err != nil {
			return nil, err
		}

		clients[info.Name] = client
	}

	candidates := map[string][]api.Network{}
	for peer, nets := range networks {
		for _, network := range nets {
			// Skip managed networks.
			if network.Managed {
				continue
			}

			// OpenVswitch only supports physical ethernet or VLAN interfaces, LXD also supports plugging in bridges.
			if !shared.StringInSlice(network.Type, []string{"physical", "bridge", "bond", "vlan"}) {
				continue
			}

			state, err := clients[peer].GetNetworkState(network.Name)
			if err != nil {
				continue
			}

			// OpenVswitch only works with full L2 devices.
			if state.Type != "broadcast" {
				continue
			}

			// Can't use interfaces that aren't up.
			if state.State != "up" {
				continue
			}

			// Make sure the interface isn't in use by ensuring there's no global addresses on it.
			addresses := []string{}

			if network.Type != "bridge" {
				for _, address := range state.Addresses {
					if address.Scope != "global" {
						continue
					}

					addresses = append(addresses, address.Address)
				}
			}

			if len(addresses) > 0 {
				continue
			}

			candidates[peer] = append(candidates[peer], network)
		}
	}

	return candidates, nil
}

func (s LXDService) SetupNetwork(uplinkNetworks map[string]string, networkConfig map[string]string) error {
	client, err := s.client("")
	if err != nil {
		return err
	}

	if uplinkNetworks[s.Name()] == "" {
		err = client.UseTarget(s.Name()).CreateNetwork(api.NetworksPost{Name: "lxdfan0", Type: "bridge"})
		if err != nil {
			return err
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
	} else {
		err = client.UseTarget(s.Name()).CreateNetwork(api.NetworksPost{
			NetworkPut: api.NetworkPut{Config: map[string]string{"parent": uplinkNetworks[s.Name()]}},
			Name:       "UPLINK",
			Type:       "physical",
		})
		if err != nil {
			return err
		}

		network := api.NetworkPut{Config: map[string]string{}, Description: "Uplink for OVN networks"}
		for gateway, ipRange := range networkConfig {
			ip, _, err := net.ParseCIDR(gateway)
			if err != nil {
				return err
			}

			if ip.To4() != nil {
				network.Config["ipv4.gateway"] = gateway
				network.Config["ipv4.ovn.ranges"] = ipRange
			} else {
				network.Config["ipv6.gateway"] = gateway
			}
		}

		err = client.CreateNetwork(api.NetworksPost{
			NetworkPut: network,
			Name:       "UPLINK",
			Type:       "physical",
		})
		if err != nil {
			return err
		}

		err = client.CreateNetwork(api.NetworksPost{
			NetworkPut: api.NetworkPut{Config: map[string]string{"network": "UPLINK"}, Description: "Default OVN network"},
			Name:       "default",
			Type:       "ovn",
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// Configure sets up the LXD storage pool (either remote ceph or local zfs), and adds the root and network devices to
// the default profile.
func (s *LXDService) Configure(bootstrap bool, localPoolTargets map[string]string, remotePoolTargets map[string]string, ovnConfig string, networkTargets map[string]string, uplinkNetworks map[string]string, networkConfig map[string]string) error {
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

	if bootstrap {
		err = s.SetConfig(s.Name(), "", map[string]string{"network.ovn.northbound_connection": ovnConfig})
		if err != nil {
			return err
		}

		for peer, secret := range networkTargets {
			err = s.SetConfig(peer, secret, map[string]string{"network.ovn.northbound_connection": ovnConfig})
			if err != nil {
				return err
			}

			if uplinkNetworks[peer] != "" {
				client, err := s.client(secret)
				if err != nil {
					return err
				}

				err = client.UseTarget(peer).CreateNetwork(api.NetworksPost{
					NetworkPut: api.NetworkPut{Config: map[string]string{"parent": uplinkNetworks[peer]}},
					Name:       "UPLINK",
					Type:       "physical",
				})
				if err != nil {
					return err
				}
			} else {
				client, err := s.client(secret)
				if err != nil {
					return err
				}

				err = client.UseTarget(peer).CreateNetwork(api.NetworksPost{Name: "lxdfan0", Type: "bridge"})
				if err != nil {
					return err
				}
			}
		}

		err = s.SetupNetwork(uplinkNetworks, networkConfig)
		if err != nil {
			return err
		}

		profile := api.ProfilesPost{ProfilePut: api.ProfilePut{Devices: map[string]map[string]string{}}, Name: "default"}
		if uplinkNetworks[s.Name()] != "" {
			profile.Devices["eth0"] = map[string]string{"name": "eth0", "network": "default", "type": "nic"}
		} else {
			profile.Devices["eth0"] = map[string]string{"name": "eth0", "network": "lxdfan0", "type": "nic"}
		}

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

		if len(remotePoolTargets) > 0 {
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
		}

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
