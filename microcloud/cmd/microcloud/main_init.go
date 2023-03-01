package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/canonical/microceph/microceph/api/types"
	cephClient "github.com/canonical/microceph/microceph/client"
	"github.com/canonical/microcluster/microcluster"
	"github.com/lxc/lxd/lxc/utils"
	"github.com/lxc/lxd/lxd/util"
	lxdAPI "github.com/lxc/lxd/shared/api"
	cli "github.com/lxc/lxd/shared/cmd"
	"github.com/lxc/lxd/shared/logger"
	"github.com/lxc/lxd/shared/units"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/mdns"
	"github.com/canonical/microcloud/microcloud/service"
)

type cmdInit struct {
	common *CmdControl

	flagAuto bool
	flagWipe bool
}

func (c *cmdInit) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "init",
		Aliases: []string{"bootstrap"},
		Short:   "Initialize the network endpoint and create or join a new cluster",
		RunE:    c.Run,
	}

	cmd.Flags().BoolVar(&c.flagAuto, "auto", false, "Automatic setup with default configuration")
	cmd.Flags().BoolVar(&c.flagWipe, "wipe", false, "Wipe disks to add to MicroCeph")

	return cmd
}

func (c *cmdInit) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
	}

	addr := util.NetworkInterfaceAddress()
	name, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Failed to retrieve system honame: %w", err)
	}

	if !c.flagAuto {
		addr, err = cli.AskString(fmt.Sprintf("Please choose the address MicroCloud will be listening on [default=%s]: ", addr), addr, nil)
		if err != nil {
			return err
		}

		// FIXME: MicroCeph does not currently support non-hostname cluster names.
		// name, err = cli.AskString(fmt.Sprintf("Please choose a name for this system [default=%s]: ", name), name, nil)
		// if err != nil {
		// 	return err
		// }
	}

	cloud, err := service.NewCloudService(context.Background(), name, addr, c.common.FlagMicroCloudDir, c.common.FlagLogVerbose, c.common.FlagLogDebug)
	if err != nil {
		return err
	}

	lxd, err := service.NewLXDService(context.Background(), name, addr, c.common.FlagMicroCloudDir)
	if err != nil {
		return err
	}

	services := []service.Service{*cloud, *lxd}
	app, err := microcluster.App(context.Background(), microcluster.Args{StateDir: api.MicroCephDir})
	if err != nil {
		return err
	}

	_, err = os.Stat(app.FileSystem.ControlSocket().URL.Host)
	if err == nil {
		ceph, err := service.NewCephService(context.Background(), name, addr, c.common.FlagMicroCloudDir)
		if err != nil {
			return err
		}

		services = append(services, *ceph)
	} else {
		logger.Info("Skipping MicroCeph service, could not detect state directory")
	}

	_, err = microcluster.App(context.Background(), microcluster.Args{StateDir: api.MicroOVNDir})
	if err != nil {
		logger.Info("Skipping MicroOVN service, could not detect state directory")
	} else {
		ovn, err := service.NewOVNService(context.Background(), name, addr, c.common.FlagMicroCloudDir)
		if err != nil {
			return err
		}

		services = append(services, *ovn)
	}

	s := service.NewServiceHandler(name, addr, services...)
	peers, err := lookupPeers(s, c.flagAuto)
	if err != nil {
		return err
	}

	err = Bootstrap(s, peers)
	if err != nil {
		return err
	}

	var localDisks map[string][]lxdAPI.ClusterMemberConfigKey
	wantsDisks := true
	if !c.flagAuto {
		wantsDisks, err = cli.AskBool("Would you like to add a local LXD storage pool? (yes/no) [default=yes]: ", "yes")
		if err != nil {
			return err
		}
	}

	if wantsDisks {
		askRetry("Retry selecting disks?", c.flagAuto, func() error {
			// Add the local member to the list of peers so we can select disks.
			peers[name] = addr
			defer delete(peers, name)
			localDisks, err = askLocalPool(peers, c.flagAuto, c.flagWipe, *lxd)

			return err
		})
	}

	err = AddPeers(s, peers, localDisks)
	if err != nil {
		return err
	}

	if s.Services[service.MicroCeph] != nil {
		ceph, ok := s.Services[service.MicroCeph].(service.CephService)
		if !ok {
			return fmt.Errorf("Invalid MicroCeph service")
		}

		wantsDisks = true
		if !c.flagAuto {
			wantsDisks, err = cli.AskBool("Would you like to add additional local disks to MicroCeph? (yes/no) [default=yes]: ", "yes")
			if err != nil {
				return err
			}
		}

		if wantsDisks {
			askRetry("Retry selecting disks?", c.flagAuto, func() error {

				addedRemote, err = askRemotePool(c.flagAuto, c.flagWipe, ceph, *lxd, localDisks)

				return err
			})
		}
	}

	err = lxd.Configure(len(localDisks) > 0, addedRemote)
	if err != nil {
		return err
	}

	fmt.Println("MicroCloud is ready")

	return nil
}

// askRetry will print all errors and re-attempt the given function on user input.
func askRetry(question string, auto bool, f func() error) {
	for {
		retry := false
		err := f()
		if err != nil {
			fmt.Println(err)

			if !auto {
				retry, err = cli.AskBool(fmt.Sprintf("%s (yes/no) [default=yes]: ", question), "yes")
				if err != nil {
					fmt.Println(err)
					retry = false
				}
			}
		}

		if !retry {
			break
		}
	}
}

func lookupPeers(s *service.ServiceHandler, auto bool) (map[string]string, error) {
	stdin := bufio.NewReader(os.Stdin)
	totalPeers := map[string]string{}

	fmt.Println("Scanning for eligible servers...")
	if !auto {
		fmt.Println("Press enter to end scanning for servers")
	}

	// Wait for input to stop scanning.
	var doneCh chan error
	if !auto {
		doneCh = make(chan error)
		go func() {
			_, err := stdin.ReadByte()
			if err != nil {
				doneCh <- err
			} else {
				close(doneCh)
			}

			fmt.Println("Ending scan")
		}()
	}

	for {
		select {
		case err := <-doneCh:
			if err != nil {
				return nil, err
			}

			return totalPeers, nil
		default:
			peers, err := mdns.LookupPeers(context.Background(), mdns.ClusterService, s.Name)
			if err != nil {
				return nil, err
			}

			for peer, addr := range peers {
				_, ok := totalPeers[peer]
				if !ok {
					fmt.Printf(" Found %q at %q\n", peer, addr)
					totalPeers[peer] = addr
				}
			}

			if auto {
				return totalPeers, nil
			}

			// Sleep for a few seconds before retrying.
			time.Sleep(5 * time.Second)
		}
	}
}

func Bootstrap(sh *service.ServiceHandler, peers map[string]string) error {
	fmt.Println("Initializing a new cluster")

	// Bootstrap MicroCloud first.
	cloudService, ok := sh.Services[service.MicroCloud]
	if !ok {
		return fmt.Errorf("Missing MicroCloud service")
	}

	err := cloudService.Bootstrap()
	if err != nil {
		return fmt.Errorf("Failed to bootstrap local %s: %w", service.MicroCloud, err)
	}

	fmt.Printf(" Local %s is ready\n", service.MicroCloud)
	return sh.RunAsync(func(s service.Service) error {
		if s.Type() == service.MicroCloud {
			return nil
		}

		err := s.Bootstrap()
		if err != nil {
			return fmt.Errorf("Failed to bootstrap local %s: %w", s.Type(), err)
		}

		fmt.Printf(" Local %s is ready\n", s.Type())

		return nil
	})
}

func AddPeers(sh *service.ServiceHandler, peers map[string]string, localDisks map[string][]lxdAPI.ClusterMemberConfigKey) error {
	joinConfig := map[service.ServiceType][]mdns.JoinConfig{}
	for serviceType := range sh.Services {
		joinConfig[serviceType] = make([]mdns.JoinConfig, len(peers))
	}

	err := sh.RunAsync(func(s service.Service) error {
		i := 0
		for peer, memberConfig := range localDisks {
			token, err := s.IssueToken(peer)
			if err != nil {
				return fmt.Errorf("Failed to issue %s token for peer %q: %w", s.Type(), peer, err)
			}

			if peer != s.Name() {
				joinConfig[s.Type()][i] = mdns.JoinConfig{Token: token, LXDConfig: memberConfig}
				i++
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Initially add just 2 peers for each dqlite service to handle issues with role reshuffling while another
	// node is joining the cluster.
	initialSize := 2
	var initialCfg map[string]map[string]mdns.JoinConfig
	var cfgByName map[string]map[string]mdns.JoinConfig
	if len(peers) > initialSize {
		initialCfg = make(map[string]map[string]mdns.JoinConfig, initialSize)
		for peer := range peers {
			if len(initialCfg) < initialSize {
				initialCfg[peer] = make(map[string]mdns.JoinConfig, len(sh.Services))
			}
		}
		cfgByName = make(map[string]map[string]mdns.JoinConfig, len(peers)-initialSize)
	} else {
		cfgByName = make(map[string]map[string]mdns.JoinConfig, len(peers))
	}

	for serviceType, s := range sh.Services {
		i := 0
		for peer := range peers {
			cfg := joinConfig[serviceType][i]
			_, ok := initialCfg[peer]
			if ok {
				initialCfg[peer][string(s.Type())] = cfg
			} else {
				_, ok := cfgByName[peer]
				if !ok {
					cfgByName[peer] = make(map[string]mdns.JoinConfig, len(sh.Services))
				}

				cfgByName[peer][string(s.Type())] = cfg
			}

			i++
		}
	}

	fmt.Println("Awaiting cluster formation...")
	if initialSize > 0 {
		_, err = waitForCluster(sh, initialCfg)
		if err != nil {
			return err
		}

		// Sleep 3 seconds to give the cluster roles time to reshuffle before adding more members.
		time.Sleep(3 * time.Second)
	}

	timedOut, err := waitForCluster(sh, cfgByName)
	if err != nil {
		return err
	}

	if !timedOut {
		fmt.Println("Cluster initialization is complete")
	}

	cloudService, ok := sh.Services[service.MicroCloud]
	if !ok {
		return fmt.Errorf("Missing MicroCloud service")
	}

	cloudCluster, err := cloudService.ClusterMembers()
	if err != nil {
		return fmt.Errorf("Failed to get %s service cluster members: %w", cloudService.Type(), err)
	}

	err = sh.RunAsync(func(s service.Service) error {
		if s.Type() == service.MicroCloud {
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
func waitForCluster(sh *service.ServiceHandler, cfgByName map[string]map[string]mdns.JoinConfig) (bool, error) {
	bytes, err := json.Marshal(cfgByName)
	if err != nil {
		return false, fmt.Errorf("Failed to marshal list of tokens: %w", err)
	}

	server, err := mdns.NewBroadcast(mdns.TokenService, sh.Name, sh.Address, sh.Port, bytes)
	if err != nil {
		return false, fmt.Errorf("Failed to begin join token broadcast: %w", err)
	}
	timeAfter := time.After(5 * time.Minute)
	bootstrapDoneCh := make(chan struct{})
	var bootstrapDone bool
	for {
		select {
		case <-bootstrapDoneCh:
			logger.Info("Shutting down broadcast")
			err := server.Shutdown()
			if err != nil {
				return false, fmt.Errorf("Failed to shutdown mDNS server after timeout: %w", err)
			}

			bootstrapDone = true
		case <-timeAfter:
			fmt.Println("Timed out waiting for a response from all cluster members")
			logger.Info("Shutting down broadcast")
			err := server.Shutdown()
			if err != nil {
				return true, fmt.Errorf("Failed to shutdown mDNS server after timeout: %w", err)
			}

			bootstrapDone = true
		default:
			// Sleep a bit so the loop doesn't push the CPU as hard.
			time.Sleep(100 * time.Millisecond)

			peers, err := mdns.LookupPeers(context.Background(), mdns.JoinedService, sh.Name)
			if err != nil {
				return false, fmt.Errorf("Failed to lookup records from new cluster members: %w", err)
			}

			for peer := range peers {
				_, ok := cfgByName[peer]
				if ok {
					fmt.Printf(" Peer %q has joined the cluster\n", peer)
				}

				delete(cfgByName, peer)
			}

			if len(cfgByName) == 0 {
				close(bootstrapDoneCh)
			}
		}

		if bootstrapDone {
			break
		}
	}

	return false, nil
}
func askLocalPool(peers map[string]string, auto bool, wipe bool, lxd service.LXDService) (map[string][]lxdAPI.ClusterMemberConfigKey, error) {
	data := [][]string{}
	selected := map[string]string{}
	for peer, addr := range peers {
		resources, err := lxd.GetResources(true, peer, addr)
		if err != nil {
			return nil, fmt.Errorf("Failed to get system resources of LXD peer %q: %w", peer, err)
		}

		validDisks := make([]lxdAPI.ResourcesStorageDisk, 0, len(resources.Storage.Disks))
		for _, disk := range resources.Storage.Disks {
			if len(disk.Partitions) == 0 {
				validDisks = append(validDisks, disk)
			}
		}

		// If there's no spare disk, then we can't add a remote storage pool, so skip local pool creation.
		if auto && len(validDisks) < 2 {
			logger.Infof("Skipping local storage pool creation, peer %q has too few disks", peer)

			return nil, nil
		}

		for _, disk := range validDisks {
			devicePath := fmt.Sprintf("/dev/disk/by-id/%s", disk.DeviceID)
			data = append(data, []string{peer, disk.Model, units.GetByteSizeStringIEC(int64(disk.Size), 2), disk.Type, devicePath})

			// Add the first disk for each peer.
			if auto {
				_, ok := selected[peer]
				if !ok {
					selected[peer] = devicePath
				}
			}
		}
	}

	toWipe := map[string]string{}
	wipeable, err := lxd.HasExtension(false, lxd.Name(), lxd.Address(), "storage_pool_source_wipe")
	if err != nil {
		return nil, fmt.Errorf("Failed to check for source.wipe extension: %w", err)
	}

	if !auto {
		sort.Sort(utils.ByName(data))
		header := []string{"LOCATION", "MODEL", "CAPACITY", "TYPE", "PATH"}
		table := NewSelectableTable(header, data)

		// map the rows (as strings) to the associated row.
		rowMap := make(map[string][]string, len(data))
		for i, r := range table.rows {
			rowMap[r] = data[i]
		}

		fmt.Println("Select exactly one disk from each cluster member:")
		selectedRows, err := table.Render(table.rows)
		if err != nil {
			return nil, fmt.Errorf("Failed to confirm local LXD disk selection: %w", err)
		}

		for _, entry := range selectedRows {
			target := rowMap[entry][0]
			path := rowMap[entry][4]

			_, ok := selected[target]
			if ok {
				return nil, fmt.Errorf("Failed to add local storage pool: Selected more than one disk for target peer %q", target)
			}

			selected[target] = path
		}

		if !wipe && wipeable {
			fmt.Println("Select which disks to wipe:")
			wipeRows, err := table.Render(selectedRows)
			if err != nil {
				return nil, fmt.Errorf("Failed to confirm disk wipe selection: %w", err)
			}

			for _, entry := range wipeRows {
				target := rowMap[entry][0]
				path := rowMap[entry][4]
				toWipe[target] = path
			}
		}
	}

	if len(selected) == 0 {
		return nil, nil
	}

	if len(selected) != len(peers) {
		return nil, fmt.Errorf("Failed to add local storage pool: Some peers don't have an available disk")
	}

	if wipe && wipeable {
		toWipe = selected
	}

	wipeDisk := lxdAPI.ClusterMemberConfigKey{
		Entity: "storage-pool",
		Name:   "local",
		Key:    "source.wipe",
		Value:  "true",
	}

	sourceTemplate := lxdAPI.ClusterMemberConfigKey{
		Entity: "storage-pool",
		Name:   "local",
		Key:    "source",
	}

	memberConfig := make(map[string][]lxdAPI.ClusterMemberConfigKey, len(selected))
	for target, path := range selected {
		if target == lxd.Name() {
			if toWipe[target] != "" {
				err := lxd.WipeDisk(target, filepath.Base(path))
				if err != nil {
					return nil, fmt.Errorf("Failed to add wipe disk %q on peer %q: %w", path, target, err)
				}
			}

			err := lxd.AddLocalPool("", path)
			if err != nil {
				return nil, fmt.Errorf("Failed to add pending local storage pool on peer %q: %w", target, err)
			}
		} else {
			sourceTemplate.Value = path
			memberConfig[target] = []lxdAPI.ClusterMemberConfigKey{sourceTemplate}
			if toWipe[target] != "" {
				memberConfig[target] = append(memberConfig[target], wipeDisk)
			}
		}

	}

	return memberConfig, nil
}

func askRemotePool(auto bool, wipe bool, ceph service.CephService, lxd service.LXDService, localDisks map[string]string) (bool, error) {
	localCeph, err := ceph.Client()
	if err != nil {
		return false, err
	}

	peers, err := localCeph.GetClusterMembers(context.Background())
	if err != nil {
		return false, fmt.Errorf("Failed to get list of current peers: %w", err)
	}

	header := []string{"LOCATION", "MODEL", "CAPACITY", "TYPE", "PATH"}
	data := [][]string{}
	for _, peer := range peers {
		// List configured disks.
		disks, err := cephClient.GetDisks(context.Background(), localCeph.UseTarget(peer.Name))
		if err != nil {
			return false, err
		}

		// List physical disks.
		resources, err := cephClient.GetResources(context.Background(), localCeph.UseTarget(peer.Name))
		if err != nil {
			return false, err
		}

		for _, disk := range resources.Disks {
			if len(disk.Partitions) > 0 {
				continue
			}

			devicePath := fmt.Sprintf("/dev/disk/by-id/%s", disk.DeviceID)
			found := false
			for _, entry := range disks {
				if entry.Location != peer.Name {
					continue
				}

				if entry.Path == devicePath {
					found = true
					break
				}
			}

			if found {
				continue
			}

			// Skip any disks that have been reserved for the local storage pool.
			if localDisks != nil && localDisks[peer.Name] == devicePath {
				continue
			}

			data = append(data, []string{peer.Name, disk.Model, units.GetByteSizeStringIEC(int64(disk.Size), 2), disk.Type, devicePath})
		}
	}

	if len(data) == 0 {
		return false, fmt.Errorf("Found no available disks")
	}

	sort.Sort(utils.ByName(data))
	table := NewSelectableTable(header, data)
	selected := table.rows
	var toWipe []string
	if wipe {
		toWipe = selected
	}

	// map the rows (as strings) to the associated row.
	rowMap := make(map[string][]string, len(data))
	for i, r := range table.rows {
		rowMap[r] = data[i]
	}

	if len(table.rows) == 0 {
		return false, nil
	}

	if !auto {
		fmt.Println("Select from the available unpartitioned disks:")
		selected, err = table.Render(table.rows)
		if err != nil {
			return false, fmt.Errorf("Failed to confirm disk selection: %w", err)
		}

		if len(selected) > 0 && !wipe {
			fmt.Println("Select which disks to wipe:")
			toWipe, err = table.Render(selected)
			if err != nil {
				return false, fmt.Errorf("Failed to confirm disk wipe selection: %w", err)
			}
		}
	}

	wipeMap := make(map[string]bool, len(toWipe))
	for _, entry := range toWipe {
		_, ok := rowMap[entry]
		if ok {
			wipeMap[entry] = true
		}
	}

	diskMap := map[string][]types.DisksPost{}
	for _, entry := range selected {
		target := rowMap[entry][0]
		path := rowMap[entry][4]

		_, ok := diskMap[target]
		if !ok {
			diskMap[target] = []types.DisksPost{}
		}

		diskMap[target] = append(diskMap[target], types.DisksPost{Path: path, Wipe: wipeMap[entry]})
	}

	if len(diskMap) == len(peers) && len(peers) >= 3 {
		fmt.Printf("Adding %d disks to MicroCeph\n", len(selected))
		targets := make([]string, 0, len(peers))
		for target, reqs := range diskMap {
			for _, req := range reqs {
				err = cephClient.AddDisk(context.Background(), localCeph.UseTarget(target), &req)
				if err != nil {
					return false, err
				}
			}

			targets = append(targets, target)
		}

		err = lxd.AddRemotePools(targets)
		if err != nil {
			return false, err
		}

		return true, nil
	}

	return false, fmt.Errorf("Unable to add remote storage pool: Each peer (minimum 3) must have allocated disks")
}
