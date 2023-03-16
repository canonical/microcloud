package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	cephTypes "github.com/canonical/microceph/microceph/api/types"
	cephClient "github.com/canonical/microceph/microceph/client"
	"github.com/lxc/lxd/lxc/utils"
	"github.com/lxc/lxd/lxd/util"
	lxdAPI "github.com/lxc/lxd/shared/api"
	cli "github.com/lxc/lxd/shared/cmd"
	"github.com/lxc/lxd/shared/logger"
	"github.com/lxc/lxd/shared/units"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/mdns"
	"github.com/canonical/microcloud/microcloud/service"
)

type cmdInit struct {
	common *CmdControl

	flagAutoSetup    bool
	flagWipeAllDisks bool
}

func (c *cmdInit) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "init",
		Aliases: []string{"bootstrap"},
		Short:   "Initialize the network endpoint and create or join a new cluster",
		RunE:    c.Run,
	}

	cmd.Flags().BoolVar(&c.flagAutoSetup, "auto", false, "Automatic setup with default configuration")
	cmd.Flags().BoolVar(&c.flagWipeAllDisks, "wipe", false, "Wipe disks to add to MicroCeph")

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

	if !c.flagAutoSetup {
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

	services := []types.ServiceType{types.MicroCloud, types.LXD}
	optionalServices := map[types.ServiceType]string{
		types.MicroCeph: api.MicroCephDir,
		types.MicroOVN:  api.MicroOVNDir,
	}

	missingServices := []string{}
	for serviceType, stateDir := range optionalServices {
		if service.ServiceExists(serviceType, stateDir) {
			services = append(services, serviceType)
		} else {
			missingServices = append(missingServices, string(serviceType))
		}
	}

	if len(missingServices) > 0 {
		serviceStr := strings.Join(missingServices, ",")
		if !c.flagAutoSetup {
			skip, err := cli.AskBool(fmt.Sprintf("%s not found. Continue anyway? (yes/no) [default=yes]: ", serviceStr), "yes")
			if err != nil {
				return err
			}

			if !skip {
				return nil
			}
		}

		logger.Infof("Skipping %s (could not detect service state directory)", serviceStr)
	}

	s, err := service.NewServiceHandler(name, addr, c.common.FlagMicroCloudDir, c.common.FlagLogDebug, c.common.FlagLogVerbose, services...)
	if err != nil {
		return err
	}

	peers, err := lookupPeers(s, c.flagAutoSetup)
	if err != nil {
		return err
	}

	fmt.Println("Initializing a new cluster")
	err = s.RunConcurrent(true, func(s service.Service) error {
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

	var localDisks map[string][]lxdAPI.ClusterMemberConfigKey
	wantsDisks := true
	if !c.flagAutoSetup {
		wantsDisks, err = cli.AskBool("Would you like to add a local LXD storage pool? (yes/no) [default=yes]: ", "yes")
		if err != nil {
			return err
		}
	}

	lxd := s.Services[types.LXD].(*service.LXDService)
	if wantsDisks {
		askRetry("Retry selecting disks?", c.flagAutoSetup, func() error {
			// Add the local member to the list of peers so we can select disks.
			peers[name] = mdns.ServerInfo{Name: name, Address: addr}
			defer delete(peers, name)
			localDisks, err = askLocalPool(peers, c.flagAutoSetup, c.flagWipeAllDisks, *lxd)

			return err
		})
	}

	err = AddPeers(s, peers, localDisks)
	if err != nil {
		return err
	}

	var remotePoolTargets map[string]string
	if s.Services[types.MicroCeph] != nil {
		ceph, ok := s.Services[types.MicroCeph].(*service.CephService)
		if !ok {
			return fmt.Errorf("Invalid MicroCeph service")
		}

		wantsDisks = true
		if !c.flagAutoSetup {
			wantsDisks, err = cli.AskBool("Would you like to add additional local disks to MicroCeph? (yes/no) [default=yes]: ", "yes")
			if err != nil {
				return err
			}
		}

		if wantsDisks {
			askRetry("Retry selecting disks?", c.flagAutoSetup, func() error {
				peers[name] = mdns.ServerInfo{Name: name, Address: addr}
				defer delete(peers, name)

				reservedDisks := map[string]string{}
				for peer, config := range localDisks {
					for _, entry := range config {
						if entry.Key == "source" {
							reservedDisks[peer] = entry.Value
							break
						}
					}
				}

				remotePoolTargets, err = askRemotePool(peers, c.flagAutoSetup, c.flagWipeAllDisks, *ceph, *lxd, reservedDisks, true)
				if err != nil {
					return err
				}

				return lxd.AddRemotePools(remotePoolTargets)
			})
		}
	}

	err = lxd.Configure(len(localDisks) > 0, len(remotePoolTargets) > 0)
	if err != nil {
		return err
	}

	fmt.Println("MicroCloud is ready")

	return nil
}

// askRetry will print all errors and re-attempt the given function on user input.
func askRetry(question string, autoSetup bool, f func() error) {
	for {
		retry := false
		err := f()
		if err != nil {
			fmt.Println(err)

			if !autoSetup {
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

func lookupPeers(s *service.ServiceHandler, autoSetup bool) (map[string]mdns.ServerInfo, error) {
	stdin := bufio.NewReader(os.Stdin)
	totalPeers := map[string]mdns.ServerInfo{}
	skipPeers := map[string]bool{}

	fmt.Println("Scanning for eligible servers...")
	if !autoSetup {
		fmt.Println("Press enter to end scanning for servers")
	}

	// Wait for input to stop scanning.
	var doneCh chan error
	if !autoSetup {
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

	done := false
	for !done {
		select {
		case err := <-doneCh:
			if err != nil {
				return nil, err
			}

			done = true
		default:
			peers, err := mdns.LookupPeers(context.Background(), mdns.Version, s.Name)
			if err != nil {
				return nil, err
			}

			for peer, info := range peers {
				if skipPeers[peer] {
					continue
				}

				_, ok := totalPeers[peer]
				if !ok {
					serviceMap := make(map[types.ServiceType]bool, len(info.Services))
					for _, service := range info.Services {
						serviceMap[service] = true
					}

					for service := range s.Services {
						if !serviceMap[service] {
							skipPeers[peer] = true
							logger.Infof("Skipping peer %q due to missing services (%s)", peer, string(service))
							break
						}
					}

					if !skipPeers[peer] {
						fmt.Printf(" Found %q at %q\n", peer, info.Address)
						totalPeers[peer] = info
					}
				}
			}

			if autoSetup {
				done = true
				break
			}

			// Sleep for a few seconds before retrying.
			time.Sleep(5 * time.Second)
		}
	}

	if len(totalPeers) == 0 {
		return nil, fmt.Errorf("Found no available systems")
	}

	return totalPeers, nil
}

func AddPeers(sh *service.ServiceHandler, peers map[string]mdns.ServerInfo, localDisks map[string][]lxdAPI.ClusterMemberConfigKey) error {
	joinConfig := make(map[string]types.ServicesPut, len(peers))
	secrets := make(map[string]string, len(peers))
	for peer, info := range peers {
		joinConfig[peer] = types.ServicesPut{
			Tokens:    []types.ServiceToken{},
			Address:   info.Address,
			LXDConfig: localDisks[peer],
		}

		secrets[peer] = info.AuthSecret
	}

	mut := sync.Mutex{}
	err := sh.RunConcurrent(false, func(s service.Service) error {
		for peer := range peers {
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

	fmt.Println("Awaiting cluster formation...")
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

	err = sh.RunConcurrent(false, func(s service.Service) error {
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
func waitForCluster(sh *service.ServiceHandler, secrets map[string]string, peers map[string]types.ServicesPut) error {
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

func askLocalPool(peers map[string]mdns.ServerInfo, autoSetup bool, wipeAllDisks bool, lxd service.LXDService) (map[string][]lxdAPI.ClusterMemberConfigKey, error) {
	data := [][]string{}
	selected := map[string]string{}
	for peer, info := range peers {
		resources, err := lxd.GetResources(info.Name != lxd.Name(), peer, info.Address, info.AuthSecret)
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
		if autoSetup && len(validDisks) < 2 {
			logger.Infof("Skipping local storage pool creation, peer %q has too few disks", peer)

			return nil, nil
		}

		for _, disk := range validDisks {
			devicePath := fmt.Sprintf("/dev/disk/by-id/%s", disk.DeviceID)
			data = append(data, []string{peer, disk.Model, units.GetByteSizeStringIEC(int64(disk.Size), 2), disk.Type, devicePath})

			// Add the first disk for each peer.
			if autoSetup {
				_, ok := selected[peer]
				if !ok {
					selected[peer] = devicePath
				}
			}
		}
	}

	toWipe := map[string]string{}
	wipeable, err := lxd.HasExtension(false, lxd.Name(), lxd.Address(), "", "storage_pool_source_wipe")
	if err != nil {
		return nil, fmt.Errorf("Failed to check for source.wipe extension: %w", err)
	}

	if !autoSetup {
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

		if !wipeAllDisks && wipeable {
			fmt.Println("Select which disks to wipe:")
			wipeRows, err := table.Render(selectedRows)
			if err != nil {
				return nil, fmt.Errorf("Failed to confirm which disks to wipe: %w", err)
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

	if wipeAllDisks && wipeable {
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
			err := lxd.AddLocalPool(path, wipeable && toWipe[target] != "")
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

func askRemotePool(peers map[string]mdns.ServerInfo, autoSetup bool, wipeAllDisks bool, ceph service.CephService, lxd service.LXDService, localDisks map[string]string, checkMinSize bool) (map[string]string, error) {
	header := []string{"LOCATION", "MODEL", "CAPACITY", "TYPE", "PATH"}
	data := [][]string{}
	for peer, info := range peers {
		c, err := ceph.Client(peer, info.AuthSecret)
		if err != nil {
			return nil, err
		}

		disks, err := cephClient.GetDisks(context.Background(), c)
		if err != nil {
			return nil, err
		}

		// List physical disks.
		resources, err := cephClient.GetResources(context.Background(), c)
		if err != nil {
			return nil, err
		}

		for _, disk := range resources.Disks {
			if len(disk.Partitions) > 0 {
				continue
			}

			devicePath := fmt.Sprintf("/dev/disk/by-id/%s", disk.DeviceID)
			found := false
			for _, entry := range disks {
				if entry.Location != peer {
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
			if localDisks != nil && localDisks[peer] == devicePath {
				continue
			}

			data = append(data, []string{peer, disk.Model, units.GetByteSizeStringIEC(int64(disk.Size), 2), disk.Type, devicePath})
		}
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("Found no available disks")
	}

	sort.Sort(utils.ByName(data))
	table := NewSelectableTable(header, data)
	selected := table.rows
	var toWipe []string
	if wipeAllDisks {
		toWipe = selected
	}

	// map the rows (as strings) to the associated row.
	rowMap := make(map[string][]string, len(data))
	for i, r := range table.rows {
		rowMap[r] = data[i]
	}

	if len(table.rows) == 0 {
		return nil, nil
	}

	if !autoSetup {
		fmt.Println("Select from the available unpartitioned disks:")
		var err error
		selected, err = table.Render(table.rows)
		if err != nil {
			return nil, fmt.Errorf("Failed to confirm disk selection: %w", err)
		}

		if len(selected) > 0 && !wipeAllDisks {
			fmt.Println("Select which disks to wipe:")
			toWipe, err = table.Render(selected)
			if err != nil {
				return nil, fmt.Errorf("Failed to confirm disk wipe selection: %w", err)
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

	diskMap := map[string][]cephTypes.DisksPost{}
	for _, entry := range selected {
		target := rowMap[entry][0]
		path := rowMap[entry][4]

		_, ok := diskMap[target]
		if !ok {
			diskMap[target] = []cephTypes.DisksPost{}
		}

		diskMap[target] = append(diskMap[target], cephTypes.DisksPost{Path: path, Wipe: wipeMap[entry]})
	}

	if len(diskMap) == len(peers) {
		if !checkMinSize || len(peers) >= 3 {
			fmt.Printf("Adding %d disks to MicroCeph\n", len(selected))
			targets := make(map[string]string, len(peers))
			for target, reqs := range diskMap {
				c, err := ceph.Client(target, peers[target].AuthSecret)
				if err != nil {
					return nil, err
				}

				for _, req := range reqs {
					err = cephClient.AddDisk(context.Background(), c, &req)
					if err != nil {
						return nil, err
					}
				}

				targets[target] = peers[target].AuthSecret
			}

			return targets, nil
		}
	}

	return nil, fmt.Errorf("Unable to add remote storage pool: Each peer (minimum 3) must have allocated disks")
}
