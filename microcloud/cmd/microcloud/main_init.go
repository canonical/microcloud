package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared"
	lxdAPI "github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/logger"
	cephTypes "github.com/canonical/microceph/microceph/api/types"
	cephClient "github.com/canonical/microceph/microceph/client"
	"github.com/canonical/microcluster/client"
	ovnClient "github.com/canonical/microovn/microovn/client"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/mdns"
	"github.com/canonical/microcloud/microcloud/service"
)

// InitSystem represents the configuration passed to individual systems that join via the Handler.
type InitSystem struct {
	ServerInfo mdns.ServerInfo // Data reported by mDNS about this system.

	AvailableDisks []lxdAPI.ResourcesStorageDisk // Disks as reported by LXD.

	MicroCephDisks     []cephTypes.DisksPost                  // Disks intended to be passed to MicroCeph.
	TargetNetworks     []lxdAPI.NetworksPost                  // Target specific network configuration.
	TargetStoragePools []lxdAPI.StoragePoolsPost              // Target specific storage pool configuration.
	Networks           []lxdAPI.NetworksPost                  // Cluster-wide network configuration.
	StoragePools       []lxdAPI.StoragePoolsPost              // Cluster-wide storage pool configuration.
	StorageVolumes     map[string][]lxdAPI.StorageVolumesPost // Cluster wide storage volume configuration.

	JoinConfig []lxdAPI.ClusterMemberConfigKey // LXD Config for joining members.
}

type cmdInit struct {
	common *CmdControl

	flagAutoSetup    bool
	flagWipeAllDisks bool
	flagAddress      string
}

func (c *cmdInit) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "init",
		Aliases: []string{"bootstrap"},
		Short:   "Initialize the network endpoint and create a new cluster",
		RunE:    c.Run,
	}

	cmd.Flags().BoolVar(&c.flagAutoSetup, "auto", false, "Automatic setup with default configuration")
	cmd.Flags().BoolVar(&c.flagWipeAllDisks, "wipe", false, "Wipe disks to add to MicroCeph")
	cmd.Flags().StringVar(&c.flagAddress, "address", "", "Address to use for MicroCloud")

	return cmd
}

func (c *cmdInit) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
	}

	// Initially restart LXD so that the correct MicroCloud service related state is set by the LXD snap.
	fmt.Println("Waiting for LXD to start...")
	lxdService, err := service.NewLXDService(context.Background(), "", "", c.common.FlagMicroCloudDir)
	if err != nil {
		return err
	}

	err = lxdService.Restart(30)
	if err != nil {
		return err
	}

	systems := map[string]InitSystem{}

	addr, subnet, err := c.common.askAddress(c.flagAutoSetup, c.flagAddress)
	if err != nil {
		return err
	}

	name, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Failed to retrieve system hostname: %w", err)
	}

	systems[name] = InitSystem{
		ServerInfo: mdns.ServerInfo{
			Name:    name,
			Address: addr,
		},
	}

	services := []types.ServiceType{types.MicroCloud, types.LXD}
	optionalServices := map[types.ServiceType]string{
		types.MicroCeph: api.MicroCephDir,
		types.MicroOVN:  api.MicroOVNDir,
	}

	services, err = c.common.askMissingServices(services, optionalServices, c.flagAutoSetup)
	if err != nil {
		return err
	}

	s, err := service.NewHandler(name, addr, c.common.FlagMicroCloudDir, c.common.FlagLogDebug, c.common.FlagLogVerbose, services...)
	if err != nil {
		return err
	}

	err = lookupPeers(s, c.flagAutoSetup, subnet, systems)
	if err != nil {
		return err
	}

	err = c.common.askDisks(s, systems, c.flagAutoSetup, c.flagWipeAllDisks)
	if err != nil {
		return err
	}

	err = c.common.askNetwork(s, systems, c.flagAutoSetup)
	if err != nil {
		return err
	}

	err = setupCluster(s, systems)
	if err != nil {
		return err
	}

	return nil
}

func lookupPeers(s *service.Handler, autoSetup bool, subnet *net.IPNet, systems map[string]InitSystem) error {
	header := []string{"NAME", "IFACE", "ADDR"}
	var table *SelectableTable
	var answers []string

	tableCh := make(chan error)
	selectionCh := make(chan error)
	if !autoSetup {
		go func() {
			err := <-tableCh
			if err != nil {
				selectionCh <- err
				return
			}

			answers, err = table.GetSelections()
			selectionCh <- err
		}()
	}

	var timeAfter <-chan time.Time
	if autoSetup {
		timeAfter = time.After(5 * time.Second)
	}

	fmt.Println("Scanning for eligible servers ...")
	totalPeers := map[string]mdns.ServerInfo{}
	done := false
	for !done {
		select {
		case <-timeAfter:
			done = true
		case err := <-selectionCh:
			if err != nil {
				return err
			}

			done = true
		default:
			peers, err := mdns.LookupPeers(context.Background(), mdns.Version, s.Name)
			if err != nil {
				return err
			}

			skipPeers := map[string]bool{}
			for key, info := range peers {
				_, ok := totalPeers[key]
				if !ok {
					serviceMap := make(map[types.ServiceType]bool, len(info.Services))
					for _, service := range info.Services {
						serviceMap[service] = true
					}

					for service := range s.Services {
						if !serviceMap[service] {
							skipPeers[info.Name] = true
							logger.Infof("Skipping peer %q due to missing services (%s)", info.Name, string(service))
							break
						}
					}

					if subnet != nil && !subnet.Contains(net.ParseIP(info.Address)) {
						continue
					}

					if !skipPeers[info.Name] {
						totalPeers[key] = info
						if autoSetup {
							continue
						}

						if len(totalPeers) == 1 {
							table = NewSelectableTable(header, [][]string{{info.Name, info.Interface, info.Address}})
							table.Render(table.rows)
							time.Sleep(100 * time.Millisecond)
							tableCh <- nil
						} else {
							table.Update([]string{info.Name, info.Interface, info.Address})
						}
					}
				}
			}
		}
	}

	if len(totalPeers) == 0 {
		return fmt.Errorf("Found no available systems")
	}

	for _, answer := range answers {
		peer := table.SelectionValue(answer, "NAME")
		addr := table.SelectionValue(answer, "ADDR")
		iface := table.SelectionValue(answer, "IFACE")
		for _, info := range totalPeers {
			if info.Name == peer && info.Address == addr && info.Interface == iface {
				systems[peer] = InitSystem{
					ServerInfo: info,
				}
			}
		}
	}

	if autoSetup {
		for _, info := range totalPeers {
			systems[info.Name] = InitSystem{
				ServerInfo: info,
			}
		}

		// Add a space between the CLI and the response.
		fmt.Println("")
	}

	for _, info := range systems {
		fmt.Printf(" Selected %q at %q\n", info.ServerInfo.Name, info.ServerInfo.Address)
	}

	// Add a space between the CLI and the response.
	fmt.Println("")

	return nil
}

func AddPeers(sh *service.Handler, systems map[string]InitSystem) error {
	joinConfig := make(map[string]types.ServicesPut, len(systems))
	secrets := make(map[string]string, len(systems))
	for peer, info := range systems {
		if peer == sh.Name {
			continue
		}

		joinConfig[peer] = types.ServicesPut{
			Tokens:     []types.ServiceToken{},
			Address:    info.ServerInfo.Address,
			LXDConfig:  info.JoinConfig,
			CephConfig: info.MicroCephDisks,
		}

		secrets[peer] = info.ServerInfo.AuthSecret
	}

	mut := sync.Mutex{}
	err := sh.RunConcurrent(false, false, func(s service.Service) error {
		for peer := range joinConfig {
			token, err := s.IssueToken(peer)
			if err != nil {
				return fmt.Errorf("Failed to issue %s token for peer %q: %w", s.Type(), peer, err)
			}

			mut.Lock()
			cfg := joinConfig[peer]
			cfg.Tokens = append(joinConfig[peer].Tokens, types.ServiceToken{Service: s.Type(), JoinToken: token})
			joinConfig[peer] = cfg
			mut.Unlock()
		}

		return nil
	})
	if err != nil {
		return err
	}

	fmt.Println("Awaiting cluster formation ...")
	// Initially add just 2 peers for each dqlite service to handle issues with role reshuffling while another
	// node is joining the cluster.
	initialSize := 2
	if len(joinConfig) > initialSize {
		initialCfg := map[string]types.ServicesPut{}
		for peer, info := range joinConfig {
			initialCfg[peer] = info

			if len(initialCfg) == initialSize {
				break
			}
		}

		initialSecrets := make(map[string]string, len(initialCfg))
		for peer := range initialCfg {
			delete(joinConfig, peer)
			initialSecrets[peer] = secrets[peer]
			delete(secrets, peer)
		}

		err := waitForCluster(sh, initialSecrets, initialCfg)
		if err != nil {
			return err
		}

		// Sleep 3 seconds to give the cluster roles time to reshuffle before adding more members.
		time.Sleep(3 * time.Second)
	}

	err = waitForCluster(sh, secrets, joinConfig)
	if err != nil {
		return err
	}

	fmt.Println("Cluster initialization is complete")
	cloudService, ok := sh.Services[types.MicroCloud]
	if !ok {
		return fmt.Errorf("Missing MicroCloud service")
	}

	cloudCluster, err := cloudService.ClusterMembers()
	if err != nil {
		return fmt.Errorf("Failed to get %s service cluster members: %w", cloudService.Type(), err)
	}

	err = sh.RunConcurrent(false, false, func(s service.Service) error {
		if s.Type() == types.MicroCloud {
			return nil
		}

		cluster, err := s.ClusterMembers()
		if err != nil {
			return fmt.Errorf("Failed to get %s service cluster members: %w", s.Type(), err)
		}

		if len(cloudCluster) != len(cluster) {
			return fmt.Errorf("%s service cluster does not match %s", s.Type(), cloudService.Type())
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// waitForCluster will loop until the timeout has passed, or all cluster members for all services have reported that
// their join process is complete.
func waitForCluster(sh *service.Handler, secrets map[string]string, peers map[string]types.ServicesPut) error {
	cloud := sh.Services[types.MicroCloud].(*service.CloudService)
	joinedChan := cloud.RequestJoin(context.Background(), secrets, peers)
	timeAfter := time.After(5 * time.Minute)
	for {
		select {
		case <-timeAfter:
			return fmt.Errorf("Timed out waiting for a response from all cluster members")
		case entry, ok := <-joinedChan:
			if !ok {
				logger.Info("Join response channel has closed")

				if len(peers) != 0 {
					return fmt.Errorf("%q members failed to join the cluster", len(peers))
				}

				return nil
			}

			if entry.Error != nil {
				return fmt.Errorf("Peer %q failed to join the cluster: %w", entry.Name, entry.Error)
			}

			_, ok = peers[entry.Name]
			if !ok {
				return fmt.Errorf("Unexpected response from unknown server %q", entry.Name)
			}

			fmt.Printf(" Peer %q has joined the cluster\n", entry.Name)

			delete(peers, entry.Name)
			if len(peers) == 0 {
				close(joinedChan)

				return nil
			}
		}
	}
}

// setupCluster Bootstraps the cluster if necessary, adds all peers to the cluster, and completes any post cluster
// configuration.
func setupCluster(s *service.Handler, systems map[string]InitSystem) error {
	_, bootstrap := systems[s.Name]
	if bootstrap {
		fmt.Println("Initializing a new cluster")
		err := s.RunConcurrent(true, false, func(s service.Service) error {
			err := s.Bootstrap()
			if err != nil {
				return fmt.Errorf("Failed to bootstrap local %s: %w", s.Type(), err)
			}

			fmt.Printf(" Local %s is ready\n", s.Type())

			return nil
		})
		if err != nil {
			return err
		}

		// Only add disks for the local MicroCeph as other systems will add their disks upon joining.
		var c *client.Client
		for _, disk := range systems[s.Name].MicroCephDisks {
			if c == nil {
				c, err = s.Services[types.MicroCeph].(*service.CephService).Client("", "")
				if err != nil {
					return err
				}
			}

			logger.Debug("Adding disk to MicroCeph", logger.Ctx{"peer": s.Name, "disk": disk.Path})
			err = cephClient.AddDisk(context.Background(), c, &disk)
			if err != nil {
				return err
			}
		}
	}

	err := AddPeers(s, systems)
	if err != nil {
		return err
	}

	var ovnConfig string
	if s.Services[types.MicroOVN] != nil {
		ovn := s.Services[types.MicroOVN].(*service.OVNService)
		c, err := ovn.Client()
		if err != nil {
			return err
		}

		services, err := ovnClient.GetServices(context.Background(), c)
		if err != nil {
			return err
		}

		clusterMap := map[string]string{}
		if bootstrap {
			for peer, system := range systems {
				clusterMap[peer] = system.ServerInfo.Address
			}
		} else {
			cloud := s.Services[types.MicroCloud].(*service.CloudService)
			clusterMap, err = cloud.ClusterMembers()
			if err != nil {
				return err
			}
		}

		conns := []string{}
		for _, service := range services {
			if service.Service == "central" {
				addr := s.Address
				if service.Location != s.Name {
					addr = clusterMap[service.Location]
				}

				conns = append(conns, fmt.Sprintf("ssl:%s", util.CanonicalNetworkAddress(addr, 6641)))
			}
		}

		ovnConfig = strings.Join(conns, ",")
	}

	config := map[string]string{"network.ovn.northbound_connection": ovnConfig}
	lxd := s.Services[types.LXD].(*service.LXDService)
	lxdClient, err := lxd.Client("")
	if err != nil {
		return err
	}

	// Update LXD's global config.
	server, _, err := lxdClient.GetServer()
	if err != nil {
		return err
	}

	newServer := server.Writable()
	changed := false
	for k, v := range config {
		if newServer.Config[k] != v {
			changed = true
		}

		newServer.Config[k] = v
	}

	if changed {
		err = lxdClient.UpdateServer(newServer, "")
		if err != nil {
			return err
		}
	}

	// Create preliminary networks & storage pools on each target.
	for name, system := range systems {
		lxdClient, err := lxd.Client(system.ServerInfo.AuthSecret)
		if err != nil {
			return err
		}

		targetClient := lxdClient.UseTarget(name)
		for _, network := range system.TargetNetworks {
			err = targetClient.CreateNetwork(network)
			if err != nil {
				return err
			}
		}

		for _, pool := range system.TargetStoragePools {
			err = targetClient.CreateStoragePool(pool)
			if err != nil {
				return err
			}
		}
	}

	// If bootstrapping, finalize setup of storage pools & networks, and update the default profile accordingly.
	system, bootstrap := systems[s.Name]
	if bootstrap {
		lxd := s.Services[types.LXD].(*service.LXDService)
		lxdClient, err := lxd.Client(system.ServerInfo.AuthSecret)
		if err != nil {
			return err
		}

		profile := lxdAPI.ProfilesPost{ProfilePut: lxdAPI.ProfilePut{Devices: map[string]map[string]string{}}, Name: "default"}

		for _, network := range system.Networks {
			if network.Name == "default" || profile.Devices["eth0"] == nil {
				profile.Devices["eth0"] = map[string]string{"name": "eth0", "network": network.Name, "type": "nic"}
			}

			err = lxdClient.CreateNetwork(network)
			if err != nil {
				return err
			}
		}

		for _, pool := range system.StoragePools {
			if pool.Driver == "ceph" || profile.Devices["root"] == nil {
				profile.Devices["root"] = map[string]string{"path": "/", "pool": pool.Name, "type": "disk"}
			}

			err = lxdClient.CreateStoragePool(pool)
			if err != nil {
				return err
			}
		}

		profiles, err := lxdClient.GetProfileNames()
		if err != nil {
			return err
		}

		if !shared.StringInSlice(profile.Name, profiles) {
			err = lxdClient.CreateProfile(profile)
		} else {
			err = lxdClient.UpdateProfile(profile.Name, profile.ProfilePut, "")
		}

		if err != nil {
			return err
		}
	}

	// With storage pools set up, add some volumes for images & backups.
	for name, system := range systems {
		lxdClient, err := lxd.Client(system.ServerInfo.AuthSecret)
		if err != nil {
			return err
		}

		poolNames := []string{}
		if bootstrap {
			for _, pool := range system.TargetStoragePools {
				poolNames = append(poolNames, pool.Name)
			}
		} else {
			for _, cfg := range system.JoinConfig {
				if cfg.Name == "local" || cfg.Name == "remote" {
					if cfg.Entity == "storage-pool" && cfg.Key == "source" {
						poolNames = append(poolNames, cfg.Name)
					}
				}
			}
		}

		targetClient := lxdClient.UseTarget(name)
		for _, pool := range poolNames {
			if pool == "local" {
				err = targetClient.CreateStoragePoolVolume("local", lxdAPI.StorageVolumesPost{Name: "images", Type: "custom"})
				if err != nil {
					return err
				}

				err = targetClient.CreateStoragePoolVolume("local", lxdAPI.StorageVolumesPost{Name: "backups", Type: "custom"})
				if err != nil {
					return err
				}

				server, _, err := targetClient.GetServer()
				if err != nil {
					return err
				}

				newServer := server.Writable()
				newServer.Config["storage.backups_volume"] = "local/backups"
				newServer.Config["storage.images_volume"] = "local/images"
				err = targetClient.UpdateServer(newServer, "")
				if err != nil {
					return err
				}
			}
		}
	}

	fmt.Println("MicroCloud is ready")

	return nil
}
