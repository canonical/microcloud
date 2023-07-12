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
	"time"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/microcluster/microcluster"

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

// Client returns a client to the LXD unix socket.
// The secret should be specified when the request is going to be forwarded to a remote address, such as with UseTarget.
func (s LXDService) Client(secret string) (lxd.InstanceServer, error) {
	c, err := s.m.LocalClient()
	if err != nil {
		return nil, err
	}

	return lxd.ConnectLXDUnix(s.m.FileSystem.ControlSocket().URL.Host, &lxd.ConnectionArgs{
		HTTPClient:    c.Client.Client,
		SkipGetServer: true,
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
		SkipGetServer:      true,
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
	client, err := s.Client("")
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
	err := s.Restart(30)
	if err != nil {
		return err
	}

	config, err := s.configFromToken(joinConfig.Token)
	if err != nil {
		return err
	}

	config.Cluster.MemberConfig = joinConfig.LXDConfig
	client, err := s.Client("")
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
	client, err := s.Client("")
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
	client, err := s.Client("")
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

// HasExtension checks if the server supports the API extension.
func (s *LXDService) HasExtension(target string, address string, secret string, apiExtension string) (bool, error) {
	var err error
	var client lxd.InstanceServer
	if s.Name() == target {
		client, err = s.Client(secret)
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
		client, err = s.Client(secret)
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

// GetUplinkInterfaces returns a map of peer name to slice of api.Network that may be used with OVN.
func (s LXDService) GetUplinkInterfaces(bootstrap bool, peers []mdns.ServerInfo) (map[string][]api.Network, error) {
	clients := map[string]lxd.InstanceServer{}
	networks := map[string][]api.Network{}
	if bootstrap {
		client, err := s.Client("")
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
		// Don't include a local interface unless we are bootstrapping, in which case we shouldn't use the remote client.
		if info.Name == s.Name() {
			continue
		}

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

// isInitialized checks if LXD is initialized by fetching the storage pools.
// If none exist, that means LXD has not yet been set up.
func (s *LXDService) isInitialized(c lxd.InstanceServer) (bool, error) {
	pools, err := c.GetStoragePoolNames()
	if err != nil {
		return false, err
	}

	return len(pools) != 0, nil
}

// Restart requests LXD to shutdown, then waits until it is ready.
func (s *LXDService) Restart(timeoutSeconds int) error {
	c, err := s.Client("")
	if err != nil {
		return err
	}

	isInit, err := s.isInitialized(c)
	if err != nil {
		return fmt.Errorf("Failed to check LXD initialization: %w", err)
	}

	if isInit {
		return fmt.Errorf("LXD has already been initialized")
	}

	_, _, err = c.RawQuery("PUT", "/internal/shutdown", nil, "")
	if err != nil {
		return fmt.Errorf("Failed to send shutdown request to LXD: %w", err)
	}

	err = s.waitReady(c, timeoutSeconds)
	if err != nil {
		return err
	}

	// A sleep might be necessary here on slower machines?
	_, _, err = c.GetServer()
	if err != nil {
		return fmt.Errorf("Failed to initialize LXD server: %w", err)
	}

	return nil
}

// waitReady repeatedly (500ms intervals) asks LXD if it is ready, up to the given timeout.
func (s *LXDService) waitReady(c lxd.InstanceServer, timeoutSeconds int) error {
	finger := make(chan error, 1)
	var errLast error
	go func() {
		for i := 0; ; i++ {
			// Start logging only after the 10'th attempt (about 5
			// seconds). Then after the 30'th attempt (about 15
			// seconds), log only only one attempt every 10
			// attempts (about 5 seconds), to avoid being too
			// verbose.
			doLog := false
			if i > 10 {
				doLog = i < 30 || ((i % 10) == 0)
			}

			if doLog {
				logger.Debugf("Checking if LXD daemon is ready (attempt %d)", i)
			}

			_, _, err := c.RawQuery("GET", "/internal/ready", nil, "")
			if err != nil {
				errLast = err
				if doLog {
					logger.Warnf("Failed to check if LXD daemon is ready (attempt %d): %v", i, err)
				}

				time.Sleep(500 * time.Millisecond)
				continue
			}

			finger <- nil
			return
		}
	}()

	if timeoutSeconds > 0 {
		select {
		case <-finger:
			break
		case <-time.After(time.Second * time.Duration(timeoutSeconds)):
			return fmt.Errorf("LXD is still not running after %ds timeout (%v)", timeoutSeconds, errLast)
		}
	} else {
		<-finger
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
