package service

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/canonical/lxd/client"
	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/microcluster/v2/microcluster"
	microTypes "github.com/canonical/microcluster/v2/rest/types"

	"github.com/canonical/microcloud/microcloud/api/types"
	cloudClient "github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcloud/microcloud/cmd/tui"
	"github.com/canonical/microcloud/microcloud/version"
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
func (s LXDService) Client(ctx context.Context) (lxd.InstanceServer, error) {
	c, err := s.m.LocalClient()
	if err != nil {
		return nil, err
	}

	return lxd.ConnectLXDUnixWithContext(ctx, s.m.FileSystem.ControlSocket().URL.Host, &lxd.ConnectionArgs{
		HTTPClient:    c.Client.Client,
		SkipGetServer: true,
		Proxy:         cloudClient.AuthProxy("", types.LXD),
	})
}

// remoteClient returns an https client for the given address:port.
// It picks the cluster certificate if none is provided to verify the remote.
func (s LXDService) remoteClient(cert *x509.Certificate, address string, port int64) (lxd.InstanceServer, error) {
	c, err := s.m.RemoteClient(util.CanonicalNetworkAddress(address, port))
	if err != nil {
		return nil, err
	}

	serverCert, err := s.m.FileSystem.ServerCert()
	if err != nil {
		return nil, err
	}

	// Use the cluster certificate if none is provided.
	if cert == nil {
		clusterCert, err := s.m.FileSystem.ClusterCert()
		if err != nil {
			return nil, err
		}

		cert, err = clusterCert.PublicKeyX509()
		if err != nil {
			return nil, err
		}
	}

	remoteURL := c.URL()
	client, err := lxd.ConnectLXD(remoteURL.String(), &lxd.ConnectionArgs{
		HTTPClient:    c.Client.Client,
		TLSClientCert: string(serverCert.PublicKey()),
		TLSClientKey:  string(serverCert.PrivateKey()),
		TLSServerCert: microTypes.X509Certificate{Certificate: cert}.String(),
		SkipGetServer: true,
		Proxy:         cloudClient.AuthProxy("", types.LXD),
	})
	if err != nil {
		return nil, err
	}

	return client, nil
}

// Bootstrap bootstraps the LXD daemon on the default port.
func (s LXDService) Bootstrap(ctx context.Context) error {
	client, err := s.Client(ctx)
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
	newServer.Config["user.microcloud"] = version.RawVersion
	if client.HasExtension("instances_migration_stateful") {
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
		return errors.New("This LXD server is already clustered")
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
			return errors.New("Timed out waiting for LXD cluster to initialize")
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
	config, err := s.configFromToken(joinConfig.Token)
	if err != nil {
		return err
	}

	config.MemberConfig = joinConfig.LXDConfig
	client, err := s.Client(ctx)
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

	// Set the local server's core.https_address to be consistent with the
	// bootstrap member's wildcard
	currentServer, etag, err := client.GetServer()
	if err != nil {
		return fmt.Errorf("Failed to retrieve LXD config: %w", err)
	}

	newServer := currentServer.Writable()
	newServer.Config["core.https_address"] = "[::]:8443"

	err = client.UpdateServer(newServer, etag)
	if err != nil {
		return fmt.Errorf("Failed to update server configuration: %w", err)
	}

	return nil
}

// IssueToken issues a token for the given peer.
func (s LXDService) IssueToken(ctx context.Context, peer string) (string, error) {
	client, err := s.Client(ctx)
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

// DeleteToken deletes a token by its name.
func (s LXDService) DeleteToken(ctx context.Context, tokenName string, address string) error {
	var c lxd.InstanceServer
	var err error
	if address != "" {
		c, err = s.remoteClient(nil, address, CloudPort)
	} else {
		c, err = s.Client(ctx)
	}

	if err != nil {
		return err
	}

	// Get the cluster member join tokens. Use default project as join tokens are created in default project.
	ops, err := c.GetOperations()
	if err != nil {
		return err
	}

	for _, op := range ops {
		if op.Class != api.OperationClassToken {
			continue
		}

		if op.StatusCode != api.Running {
			continue // Tokens are single use, so if cancelled but not deleted yet its not available.
		}

		joinToken, err := op.ToClusterJoinToken()
		if err != nil {
			continue // Operation is not a valid cluster member join token operation.
		}

		if joinToken.ServerName == tokenName {
			// Delete the operation
			err = c.DeleteOperation(op.ID)
			if err != nil {
				return err
			}

			return nil
		}
	}

	return fmt.Errorf("No corresponding join token operation found for %q", tokenName)
}

// RemoteClusterMembers returns a map of cluster member names and addresses from the MicroCloud at the given address.
// Provide the certificate of the remote server for mTLS.
func (s LXDService) RemoteClusterMembers(ctx context.Context, cert *x509.Certificate, address string) (map[string]string, error) {
	client, err := s.remoteClient(cert, address, CloudPort)
	if err != nil {
		return nil, err
	}

	return s.clusterMembers(client)
}

// ClusterMembers returns a map of cluster member names.
func (s LXDService) ClusterMembers(ctx context.Context) (map[string]string, error) {
	client, err := s.Client(ctx)
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
	c, err := s.Client(ctx)
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
func (s *LXDService) HasExtension(ctx context.Context, target string, address string, cert *x509.Certificate, apiExtension string) (bool, error) {
	var err error
	var client lxd.InstanceServer
	if s.Name() == target {
		client, err = s.Client(ctx)
		if err != nil {
			return false, err
		}
	} else {
		client, err = s.remoteClient(cert, address, CloudPort)
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
func (s *LXDService) GetResources(ctx context.Context, target string, address string, cert *x509.Certificate) (*api.Resources, error) {
	var err error
	var client lxd.InstanceServer
	if s.Name() == target {
		client, err = s.Client(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		client, err = s.remoteClient(cert, address, CloudPort)
		if err != nil {
			return nil, err
		}
	}

	return client.GetServerResources()
}

// GetStoragePools fetches the list of all storage pools from LXD, keyed by pool name.
func (s LXDService) GetStoragePools(ctx context.Context, name string, address string, cert *x509.Certificate) (map[string]api.StoragePool, error) {
	var err error
	var client lxd.InstanceServer
	if name == s.Name() {
		client, err = s.Client(ctx)
	} else {
		client, err = s.remoteClient(cert, address, CloudPort)
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
func (s LXDService) GetConfig(ctx context.Context, clustered bool, name string, address string, cert *x509.Certificate) (localConfig map[string]any, globalConfig map[string]any, err error) {
	var client lxd.InstanceServer
	if name == s.Name() {
		client, err = s.Client(ctx)
	} else {
		client, err = s.remoteClient(cert, address, CloudPort)
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

// defaultNetworkInterfacesFilter filters a network based on default rules and returns whether it should be skipped.
func defaultNetworkInterfacesFilter(network api.Network, state *api.NetworkState) bool {
	// Skip managed networks.
	if network.Managed {
		return false
	}

	// Can't use interfaces that aren't up.
	if state.State != "up" {
		return false
	}

	return true
}

// ovnNetworkInterfacesFilter filters a network based on OVN specific rules and returns whether it should be skipped.
func ovnNetworkInterfacesFilter(network api.Network, state *api.NetworkState) bool {
	// OpenVswitch only supports physical ethernet or VLAN interfaces, LXD also supports plugging in bridges.
	if !slices.Contains([]string{"physical", "bridge", "bond", "vlan"}, network.Type) {
		return false
	}

	// OpenVswitch only works with full L2 devices.
	if state.Type != "broadcast" {
		return false
	}

	return true
}

// DedicatedInterface represents a dedicated interface for OVN.
type DedicatedInterface struct {
	Type      string
	Network   api.Network
	Addresses []string
}

// GetNetworkInterfaces fetches the list of networks from LXD and returns the following:
// - A map of uplink compatible networks keyed by interface name.
// - A map of ceph compatible networks keyed by interface name.
// - A map of ovn compatible networks keyed by interface name.
// - The list of all networks.
func (s LXDService) GetNetworkInterfaces(ctx context.Context, name string, address string, cert *x509.Certificate) (map[string]api.Network, map[string]DedicatedInterface, []api.Network, error) {
	var err error
	var client lxd.InstanceServer
	if name == s.Name() {
		client, err = s.Client(ctx)
	} else {
		client, err = s.remoteClient(cert, address, CloudPort)
	}

	if err != nil {
		return nil, nil, nil, err
	}

	networks, err := client.GetNetworks()
	if err != nil {
		return nil, nil, nil, err
	}

	uplinkInterfaces := map[string]api.Network{}
	dedicatedInterfaces := map[string]DedicatedInterface{}
	for _, network := range networks {
		state, err := client.GetNetworkState(network.Name)
		if err != nil {
			return nil, nil, nil, err
		}

		// Apply default filter rules and ignore invalid interfaces.
		filtered := defaultNetworkInterfacesFilter(network, state)
		if !filtered {
			continue
		}

		// Get a list of addresses configured on this interface.
		addresses := []string{}
		for _, address := range state.Addresses {
			if address.Scope != "global" {
				continue
			}

			addresses = append(addresses, fmt.Sprintf("%s/%s", address.Address, address.Netmask))
		}

		// Apply OVN specific filter rules.
		ovnFiltered := ovnNetworkInterfacesFilter(network, state)

		if len(addresses) > 0 {
			dedicatedInterfaces[network.Name] = DedicatedInterface{
				Type:      network.Type,
				Network:   network,
				Addresses: addresses,
			}

			// Special case as LXD can plug into the bridge using a veth pair.
			if ovnFiltered && network.Type == "bridge" {
				uplinkInterfaces[network.Name] = network
			}
		} else {
			// Accept all filtered interfaces without address as uplink.
			if ovnFiltered {
				uplinkInterfaces[network.Name] = network
			}
		}
	}

	return uplinkInterfaces, dedicatedInterfaces, networks, nil
}

// ValidateCephInterfaces validates the given interfaces map against the given Ceph network subnet
// and returns a map of peer name to interfaces that are in the subnet.
func (s *LXDService) ValidateCephInterfaces(cephNetworkSubnetStr string, peerInterfaces map[string]map[string]DedicatedInterface) (map[string][][]string, error) {
	_, subnet, err := net.ParseCIDR(cephNetworkSubnetStr)
	if err != nil {
		return nil, fmt.Errorf("Invalid CIDR subnet %q: %w", cephNetworkSubnetStr, err)
	}

	ones, bits := subnet.Mask.Size()
	if bits-ones == 0 {
		return nil, errors.New("Invalid Ceph network subnet (must have more than one address)")
	}

	data := make(map[string][][]string)
	for peer, ifaceByName := range peerInterfaces {
		for name, iface := range ifaceByName {
			for _, addr := range iface.Addresses {
				ip := net.ParseIP(addr)
				if ip == nil {
					// Attempt to parse the IP address as a CIDR.
					ip, _, err = net.ParseCIDR(addr)
					if err != nil {
						return nil, fmt.Errorf("Could not parse either IP nor CIDR notation for address %q: %v", addr, err)
					}
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
		tui.PrintWarning("No network interfaces found with IPs in the specified subnet. Skipping Ceph network setup")
	}

	return data, nil
}

// GetVersion gets the installed daemon version of the service, and returns an error if the version is not supported.
func (s LXDService) GetVersion(ctx context.Context) (string, error) {
	client, err := s.Client(ctx)
	if err != nil {
		return "", err
	}

	server, _, err := client.GetServer()
	if err != nil {
		return "", fmt.Errorf("Failed to retrieve current server configuration: %w", err)
	}

	err = validateVersion(s.Type(), server.Environment.ServerVersion)
	if err != nil {
		return "", err
	}

	return server.Environment.ServerVersion, nil
}

// IsInitialized returns whether the service is initialized for a given context timeout.
// If LXD is not reachable for given context timeout, an error is returned.
func (s LXDService) IsInitialized(ctx context.Context) (bool, error) {
	c, err := s.Client(ctx)
	if err != nil {
		return false, err
	}

	err = s.WaitReady(ctx, c, false, false)
	if err != nil && api.StatusErrorCheck(err, http.StatusNotFound) {
		return false, fmt.Errorf("Unix socket not found. Check if %s is installed", s.Type())
	}

	if err != nil {
		return false, err
	}

	isInit, err := s.isInitialized(c)
	if err != nil {
		return false, fmt.Errorf("Failed to check LXD initialization: %w", err)
	}

	return isInit, nil
}

// isInitialized checks if LXD is initialized by fetching the storage pools, and cluster status.
// If none exist, that means LXD has not yet been set up.
func (s *LXDService) isInitialized(c lxd.InstanceServer) (bool, error) {
	server, _, err := c.GetServer()
	if err != nil {
		return false, err
	}

	if server.Environment.ServerClustered {
		return true, nil
	}

	pools, err := c.GetStoragePoolNames()
	if err != nil {
		return false, err
	}

	return len(pools) != 0, nil
}

// WaitReady repeatedly (500ms intervals) asks LXD if it is ready, up to the given context timeout.
// It waits up to ctx timeout for LXD to start, before failing.
// Furthermore the caller can wait for both network and storage to be ready.
func (s *LXDService) WaitReady(ctx context.Context, c lxd.InstanceServer, network bool, storage bool) error {
	url := api.NewURL().Path("internal", "ready")
	if network {
		url.WithQuery("network", "1")
	}

	if storage {
		url.WithQuery("storage", "1")
	}

	for {
		_, _, err := c.RawQuery("GET", url.String(), nil, "")
		if err != nil {
			if ctx.Err() == nil {
				time.Sleep(500 * time.Millisecond)
				continue
			}

			return fmt.Errorf("Failed waiting for LXD to start: %w", err)
		}

		break
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
		return nil, "", errors.New("No default IPv4 gateway available")
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
			return nil, "", errors.New("More than one IPv4 subnet on default interface")
		}

		subnet = addrNet
	}

	if subnet == nil {
		return nil, "", errors.New("No IPv4 subnet on default interface")
	}

	return subnet, ifaceName, nil
}

// SupportsFeature checks if the specified API feature of this Service instance if supported.
func (s LXDService) SupportsFeature(ctx context.Context, feature string) (bool, error) {
	c, err := s.Client(ctx)
	if err != nil {
		return false, err
	}

	server, _, err := c.GetServer()
	if err != nil {
		return false, err
	}

	if server.APIExtensions == nil {
		return false, errors.New("API extensions not available when checking for a LXD feature")
	}

	return slices.Contains(server.APIExtensions, feature), nil
}
