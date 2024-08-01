package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/canonical/lxd/client"
	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/microcluster/v2/microcluster"
	"golang.org/x/mod/semver"

	"github.com/canonical/microcloud/microcloud/api/types"
	cloudClient "github.com/canonical/microcloud/microcloud/client"
)

// LXDService is a LXD service.
type LXDService struct {
	m *microcluster.MicroCluster

	name    string
	address string
	port    int64
	config  map[string]string
}

// NewLXDService creates a new LXD service with a client attached.
func NewLXDService(name string, addr string, cloudDir string) (*LXDService, error) {
	client, err := microcluster.App(microcluster.Args{StateDir: cloudDir})
	if err != nil {
		return nil, err
	}

	return &LXDService{
		m:       client,
		name:    name,
		address: addr,
		port:    LXDPort,
		config:  make(map[string]string),
	}, nil
}

// Client returns a client to the LXD unix socket.
// The secret should be specified when the request is going to be forwarded to a remote address, such as with UseTarget.
func (s LXDService) Client(ctx context.Context, secret string) (lxd.InstanceServer, error) {
	c, err := s.m.LocalClient()
	if err != nil {
		return nil, err
	}

	return lxd.ConnectLXDUnixWithContext(ctx, s.m.FileSystem.ControlSocket().URL.Host, &lxd.ConnectionArgs{
		HTTPClient:    c.Client.Client,
		SkipGetServer: true,
		Proxy:         cloudClient.AuthProxy(secret, types.LXD),
	})
}

// remoteClient returns an https client for the given address:port.
func (s LXDService) remoteClient(secret string, address string, port int64) (lxd.InstanceServer, error) {
	c, err := s.m.RemoteClient(util.CanonicalNetworkAddress(address, port))
	if err != nil {
		return nil, err
	}

	serverCert, err := s.m.FileSystem.ServerCert()
	if err != nil {
		return nil, err
	}

	remoteURL := c.URL()
	client, err := lxd.ConnectLXD(remoteURL.String(), &lxd.ConnectionArgs{
		HTTPClient:         c.Client.Client,
		TLSClientCert:      string(serverCert.PublicKey()),
		TLSClientKey:       string(serverCert.PrivateKey()),
		InsecureSkipVerify: true,
		SkipGetServer:      true,
		Proxy:              cloudClient.AuthProxy(secret, types.LXD),
	})
	if err != nil {
		return nil, err
	}

	return client, nil
}

// Bootstrap bootstraps the LXD daemon on the default port.
func (s LXDService) Bootstrap(ctx context.Context) error {
	client, err := s.Client(ctx, "")
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
	if client.HasExtension("migration_stateful_default") {
		newServer.Config["instances.migration.stateful"] = "true"
	}

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

	err = op.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("Failed to initialize cluster: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("Timed out waiting for LXD cluster to initialize")
		default:
			names, err := client.GetClusterMemberNames()
			if err != nil {
				return err
			}

			if len(names) > 0 {
				return nil
			}

			time.Sleep(time.Second)
		}
	}
}

// Join joins a cluster with the given token.
func (s LXDService) Join(ctx context.Context, joinConfig JoinConfig) error {
	err := s.Restart(ctx, 30)
	if err != nil {
		return err
	}

	config, err := s.configFromToken(joinConfig.Token)
	if err != nil {
		return err
	}

	config.Cluster.MemberConfig = joinConfig.LXDConfig
	client, err := s.Client(ctx, "")
	if err != nil {
		return err
	}

	op, err := client.UpdateCluster(*config, "")
	if err != nil {
		return fmt.Errorf("Failed to join cluster: %w", err)
	}

	err = op.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("Failed to configure cluster: %w", err)
	}

	return nil
}

// IssueToken issues a token for the given peer.
func (s LXDService) IssueToken(ctx context.Context, peer string) (string, error) {
	client, err := s.Client(ctx, "")
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

// RemoteClusterMembers returns a map of cluster member names and addresses from the MicroCloud at the given address, authenticated with the given secret.
func (s LXDService) RemoteClusterMembers(ctx context.Context, secret string, address string) (map[string]string, error) {
	client, err := s.remoteClient(secret, address, CloudPort)
	if err != nil {
		return nil, err
	}

	return s.clusterMembers(client)
}

// ClusterMembers returns a map of cluster member names.
func (s LXDService) ClusterMembers(ctx context.Context) (map[string]string, error) {
	client, err := s.Client(ctx, "")
	if err != nil {
		return nil, err
	}

	return s.clusterMembers(client)
}

// clusterMembers returns a map of cluster member names and addresses.
// If LXD is not clustered, it returns a 503 http error similar to microcluster.
func (s LXDService) clusterMembers(client lxd.InstanceServer) (map[string]string, error) {
	server, _, err := client.GetServer()
	if err != nil {
		return nil, err
	}

	if !server.Environment.ServerClustered {
		return nil, api.StatusErrorf(http.StatusServiceUnavailable, "LXD is not part of a cluster")
	}

	members, err := client.GetClusterMembers()
	if err != nil {
		return nil, err
	}

	genericMembers := make(map[string]string, len(members))
	for _, member := range members {
		url, err := url.Parse(member.URL)
		if err != nil {
			return nil, err
		}

		genericMembers[member.ServerName] = url.Host
	}

	return genericMembers, nil
}

// DeleteClusterMember removes the given cluster member from the service.
func (s LXDService) DeleteClusterMember(ctx context.Context, name string, force bool) error {
	c, err := s.Client(ctx, "")
	if err != nil {
		return err
	}

	return c.DeleteClusterMember(name, force)
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
func (s LXDService) Port() int64 {
	return s.port
}

// SetConfig sets the config of this Service instance.
func (s *LXDService) SetConfig(config map[string]string) {
	if s.config == nil {
		s.config = make(map[string]string)
	}

	for key, value := range config {
		s.config[key] = value
	}
}

// HasExtension checks if the server supports the API extension.
func (s *LXDService) HasExtension(ctx context.Context, target string, address string, secret string, apiExtension string) (bool, error) {
	var err error
	var client lxd.InstanceServer
	if s.Name() == target {
		client, err = s.Client(ctx, secret)
		if err != nil {
			return false, err
		}
	} else {
		client, err = s.remoteClient(secret, address, CloudPort)
		if err != nil {
			return false, err
		}
	}

	// Fill the cache of API extensions.
	// If the client's internal `server` field isn't yet populated
	// a call to HasExtension will always return true for any extension.
	_, _, err = client.GetServer()
	if err != nil {
		return false, err
	}

	return client.HasExtension(apiExtension), nil
}

// GetResources returns the system resources for the LXD target.
// As we cannot guarantee that LXD is available on this machine, the request is
// forwarded through MicroCloud on via the ListenPort argument.
func (s *LXDService) GetResources(ctx context.Context, target string, address string, secret string) (*api.Resources, error) {
	var err error
	var client lxd.InstanceServer
	if s.Name() == target {
		client, err = s.Client(ctx, secret)
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

// GetStoragePools fetches the list of all storage pools from LXD, keyed by pool name.
func (s LXDService) GetStoragePools(ctx context.Context, name string, address string, secret string) (map[string]api.StoragePool, error) {
	var err error
	var client lxd.InstanceServer
	if name == s.Name() {
		client, err = s.Client(ctx, "")
	} else {
		client, err = s.remoteClient(secret, address, CloudPort)
	}

	if err != nil {
		return nil, err
	}

	pools, err := client.GetStoragePools()
	if err != nil {
		return nil, err
	}

	poolMap := make(map[string]api.StoragePool, len(pools))
	for _, pool := range pools {
		poolMap[pool.Name] = pool
	}

	return poolMap, nil
}

// GetConfig returns the member-specific and cluster-wide configurations of LXD.
// If LXD is not clustered, it just returns the member-specific configuration.
func (s LXDService) GetConfig(ctx context.Context, clustered bool, name string, address string, secret string) (localConfig map[string]any, globalConfig map[string]any, err error) {
	var client lxd.InstanceServer
	if name == s.Name() {
		client, err = s.Client(ctx, "")
	} else {
		client, err = s.remoteClient(secret, address, CloudPort)
	}

	if err != nil {
		return nil, nil, err
	}

	if clustered {
		server, _, err := client.GetServer()
		if err != nil {
			return nil, nil, err
		}

		localServer, _, err := client.UseTarget(name).GetServer()
		if err != nil {
			return nil, nil, err
		}

		return localServer.Writable().Config, server.Writable().Config, nil
	}

	server, _, err := client.GetServer()
	if err != nil {
		return nil, nil, err
	}

	return server.Writable().Config, nil, nil
}

// defaultNetworkInterfacesFilter filters a network based on default rules and return whether it should be skipped and the addresses on the interface.
func defaultNetworkInterfacesFilter(client lxd.InstanceServer, network api.Network) (bool, []string) {
	// Skip managed networks.
	if network.Managed {
		return true, []string{}
	}

	// OpenVswitch only supports physical ethernet or VLAN interfaces, LXD also supports plugging in bridges.
	if !shared.ValueInSlice(network.Type, []string{"physical", "bridge", "bond", "vlan"}) {
		return true, []string{}
	}

	state, err := client.GetNetworkState(network.Name)
	if err != nil {
		return true, []string{}
	}

	// OpenVswitch only works with full L2 devices.
	if state.Type != "broadcast" {
		return true, []string{}
	}

	// Can't use interfaces that aren't up.
	if state.State != "up" {
		return true, []string{}
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

	return false, addresses
}

// CephDedicatedInterface represents a dedicated interface for Ceph.
type CephDedicatedInterface struct {
	Type      string
	Addresses []string
}

// GetNetworkInterfaces fetches the list of networks from LXD and returns the following:
// - A map of uplink compatible networks keyed by interface name.
// - A map of ceph compatible networks keyed by interface name.
// - The list of all networks.
func (s LXDService) GetNetworkInterfaces(ctx context.Context, name string, address string, secret string) (map[string]api.Network, map[string]CephDedicatedInterface, []api.Network, error) {
	var err error
	var client lxd.InstanceServer
	if name == s.Name() {
		client, err = s.Client(ctx, "")
	} else {
		client, err = s.remoteClient(secret, address, CloudPort)
	}

	if err != nil {
		return nil, nil, nil, err
	}

	networks, err := client.GetNetworks()
	if err != nil {
		return nil, nil, nil, err
	}

	uplinkInterfaces := map[string]api.Network{}
	cephInterfaces := map[string]CephDedicatedInterface{}
	for _, network := range networks {
		filtered, addresses := defaultNetworkInterfacesFilter(client, network)
		if filtered {
			continue
		}

		if len(addresses) == 0 {
			uplinkInterfaces[network.Name] = network
		} else {
			cephInterfaces[network.Name] = CephDedicatedInterface{
				Type:      network.Type,
				Addresses: addresses,
			}
		}
	}

	return uplinkInterfaces, cephInterfaces, networks, nil
}

// ValidateCephInterfaces validates the given interfaces map against the given Ceph network subnet
// and returns a map of peer name to interfaces that are in the subnet.
func (s *LXDService) ValidateCephInterfaces(cephNetworkSubnetStr string, peerInterfaces map[string]map[string]CephDedicatedInterface) (map[string][][]string, error) {
	_, subnet, err := net.ParseCIDR(cephNetworkSubnetStr)
	if err != nil {
		return nil, fmt.Errorf("Invalid CIDR subnet: %v", err)
	}

	ones, bits := subnet.Mask.Size()
	if bits-ones == 0 {
		return nil, fmt.Errorf("Invalid Ceph network subnet (must have more than one address)")
	}

	data := make(map[string][][]string)
	for peer, ifaceByName := range peerInterfaces {
		for name, iface := range ifaceByName {
			for _, addr := range iface.Addresses {
				ip := net.ParseIP(addr)
				if ip == nil {
					return nil, fmt.Errorf("Invalid IP address: %v", addr)
				}

				if (subnet.IP.To4() != nil && ip.To4() != nil && subnet.Contains(ip)) || (subnet.IP.To16() != nil && ip.To16() != nil && subnet.Contains(ip)) {
					_, ok := data[peer]
					if !ok {
						data[peer] = [][]string{{peer, name, addr, iface.Type}}
					} else {
						data[peer] = append(data[peer], []string{peer, name, addr, iface.Type})
					}
				}
			}
		}
	}

	if len(data) == 0 {
		fmt.Println("No network interfaces found with IPs in the specified subnet, skipping Ceph network setup")
	}

	return data, nil
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
func (s *LXDService) Restart(ctx context.Context, timeoutSeconds int) error {
	c, err := s.Client(ctx, "")
	if err != nil {
		return err
	}

	isInit, err := s.isInitialized(c)
	if err != nil {
		return fmt.Errorf("Failed to check LXD initialization: %w", err)
	}

	if isInit {
		return fmt.Errorf("Detected pre-existing LXD storage pools. LXD might have already been initialized")
	}

	server, _, err := c.GetServer()
	if err != nil {
		return fmt.Errorf("Failed to get LXD server information: %w", err)
	}

	// As of LXD 5.21, the LXD snap should support content interfaces to automatically detect the presence of MicroOVN and MicroCeph.
	// For older LXDs, we must restart to trigger the snap's detection of MicroOVN and MicroCeph to properly set up LXD's snap environment to work with them.
	// semver.Compare will return 1 if the first argument is larger, 0 if the arguments are the same, and -1 if the first argument is smaller.
	lxdVersion := semver.Canonical(fmt.Sprintf("v%s", server.Environment.ServerVersion))
	expectedVersion := semver.Canonical(fmt.Sprintf("v%s", lxdMinVersion))
	if semver.Compare(lxdVersion, expectedVersion) >= 0 {
		return nil
	}

	logger.Warnf("Detected LXD at version %q (older than %q), attempting restart to detect MicroOVN and MicroCeph integration", lxdVersion, expectedVersion)

	_, _, err = c.RawQuery("PUT", "/internal/shutdown", nil, "")
	if err != nil && err.Error() != "Shutdown already in progress" {
		return fmt.Errorf("Failed to send shutdown request to LXD: %w", err)
	}

	err = s.waitReady(ctx, c, timeoutSeconds)
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
func (s *LXDService) waitReady(ctx context.Context, c lxd.InstanceServer, timeoutSeconds int) error {
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

	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	if timeoutSeconds > 0 {
		select {
		case <-finger:
			break
		case <-ctx.Done():
			return fmt.Errorf("LXD is still not running after %ds timeout (%v)", timeoutSeconds, errLast)
		}
	} else {
		<-finger
	}

	return nil
}

// defaultGatewaySubnetV4 returns subnet of default gateway interface.
func (s LXDService) defaultGatewaySubnetV4() (*net.IPNet, string, error) {
	available, ifaceName, err := FanNetworkUsable()
	if err != nil {
		return nil, "", err
	}

	if !available {
		return nil, "", fmt.Errorf("No default IPv4 gateway available")
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
