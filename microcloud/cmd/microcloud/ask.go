package main

import (
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/canonical/lxd/shared/api"
	lxdAPI "github.com/canonical/lxd/shared/api"
	cli "github.com/canonical/lxd/shared/cmd"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/lxd/shared/units"
	cephTypes "github.com/canonical/microceph/microceph/api/types"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/mdns"
	"github.com/canonical/microcloud/microcloud/service"
)

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

func askMissingServices(services []types.ServiceType, stateDirs map[types.ServiceType]string, autoSetup bool) ([]types.ServiceType, error) {
	missingServices := []string{}
	for serviceType, stateDir := range stateDirs {
		if service.ServiceExists(serviceType, stateDir) {
			services = append(services, serviceType)
		} else {
			missingServices = append(missingServices, string(serviceType))
		}
	}

	if len(missingServices) > 0 {
		serviceStr := strings.Join(missingServices, ",")
		if !autoSetup {
			skip, err := cli.AskBool(fmt.Sprintf("%s not found. Continue anyway? (yes/no) [default=yes]: ", serviceStr), "yes")
			if err != nil {
				return nil, err
			}

			if !skip {
				return services, nil
			}
		}

		logger.Infof("Skipping %s (could not detect service state directory)", serviceStr)
	}

	return services, nil
}

func askAddress(autoSetup bool, listenAddr string) (string, *net.IPNet, error) {
	info, err := mdns.GetNetworkInfo()
	if err != nil {
		return "", nil, fmt.Errorf("Failed to find network interfaces: %w", err)
	}

	if listenAddr == "" {
		if len(info) == 0 {
			return "", nil, fmt.Errorf("Found no valid network interfaces")
		}

		listenAddr = info[0].Address
		if !autoSetup && len(info) > 1 {
			data := make([][]string, 0, len(info))
			for _, net := range info {
				data = append(data, []string{net.Address, net.Interface})
			}

			table := NewSelectableTable([]string{"ADDRESS", "IFACE"}, data)
			askRetry("Retry selecting an address?", autoSetup, func() error {
				fmt.Println("Select an address for MicroCloud's internal traffic:")
				table.Render(table.rows)
				answers, err := table.GetSelections()
				if err != nil {
					return err
				}

				if len(answers) != 1 {
					return fmt.Errorf("You must select exactly one address")
				}

				listenAddr = table.SelectionValue(answers[0], "ADDRESS")

				fmt.Printf(" Using address %q for MicroCloud\n\n", listenAddr)

				return nil
			})
		} else {
			fmt.Printf("Using address %q for MicroCloud\n", listenAddr)
		}
	}

	var subnet *net.IPNet

	for _, network := range info {
		if network.Subnet.Contains(net.ParseIP(listenAddr)) {
			subnet = network.Subnet

			break
		}
	}

	if subnet == nil {
		return "", nil, fmt.Errorf("Cloud not find valid subnet for address %q", listenAddr)
	}

	if !autoSetup {
		filter, err := cli.AskBool(fmt.Sprintf("Limit search for other MicroCloud servers to %s? (yes/no) [default=yes]: ", subnet.String()), "yes")
		if err != nil {
			return "", nil, err
		}

		if !filter {
			subnet = nil
		}
	}

	return listenAddr, subnet, nil
}

func askDisks(sh *service.ServiceHandler, peers map[string]mdns.ServerInfo, bootstrap bool, autoSetup bool, wipeAllDisks bool) (map[string][]lxdAPI.ClusterMemberConfigKey, map[string][]cephTypes.DisksPost, error) {
	if bootstrap {
		// Add the local system to the list of peers so we can select disks.
		peers[sh.Name] = mdns.ServerInfo{Name: sh.Name}
		defer delete(peers, sh.Name)
	}

	allResources := make(map[string]*lxdAPI.Resources, len(peers))
	var err error
	for peer, info := range peers {
		allResources[peer], err = sh.Services[types.LXD].(*service.LXDService).GetResources(peer, info.Address, info.AuthSecret)
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to get system resources of peer %q: %w", peer, err)
		}
	}

	validDisks := make(map[string][]lxdAPI.ResourcesStorageDisk, len(allResources))
	for peer, r := range allResources {
		validDisks[peer] = make([]lxdAPI.ResourcesStorageDisk, 0, len(r.Storage.Disks))
		for _, disk := range r.Storage.Disks {
			if len(disk.Partitions) == 0 {
				validDisks[peer] = append(validDisks[peer], disk)
			}
		}
	}

	var diskConfig map[string][]lxdAPI.ClusterMemberConfigKey
	var reservedDisks map[string]string
	wantsDisks := true
	if !autoSetup {
		wantsDisks, err = cli.AskBool("Would you like to set up local storage? (yes/no) [default=yes]: ", "yes")
		if err != nil {
			return nil, nil, err
		}
	}

	lxd := sh.Services[types.LXD].(*service.LXDService)
	if wantsDisks {
		askRetry("Retry selecting disks?", autoSetup, func() error {
			diskConfig, reservedDisks, err = askLocalPool(validDisks, autoSetup, wipeAllDisks, *lxd)

			return err
		})
	}

	for peer, path := range reservedDisks {
		fmt.Printf(" Using %q on %q for local storage pool\n", path, peer)
	}

	if len(reservedDisks) > 0 {
		// Add a space between the CLI and the response.
		fmt.Println("")
	}

	var cephDisks map[string][]cephTypes.DisksPost
	if sh.Services[types.MicroCeph] != nil {
		availableDisks := map[string][]lxdAPI.ResourcesStorageDisk{}
		for peer, disks := range validDisks {
			peerDisks := []lxdAPI.ResourcesStorageDisk{}
			for _, disk := range disks {
				devicePath := fmt.Sprintf("/dev/%s", disk.ID)
				if disk.DeviceID != "" {
					devicePath = fmt.Sprintf("/dev/disk/by-id/%s", disk.DeviceID)
				} else if disk.DevicePath != "" {
					devicePath = fmt.Sprintf("/dev/disk/by-path/%s", disk.DevicePath)
				}

				if reservedDisks[peer] == devicePath {
					continue
				}

				peerDisks = append(peerDisks, disk)
			}

			if len(peerDisks) > 0 {
				availableDisks[peer] = peerDisks
			}
		}

		if len(availableDisks) < 3 {
			fmt.Println("Insufficient number of disks available to set up distributed storage, skipping at this time")
		} else {
			ceph := sh.Services[types.MicroCeph].(*service.CephService)
			wantsDisks = true
			if !autoSetup {
				wantsDisks, err = cli.AskBool("Would you like to set up distributed storage? (yes/no) [default=yes]: ", "yes")
				if err != nil {
					return nil, nil, err
				}

				if len(peers) != len(availableDisks) && wantsDisks {
					wantsDisks, err = cli.AskBool("Unable to find disks on some systems. Continue anyway? (yes/no) [default=yes]: ", "yes")
					if err != nil {
						return nil, nil, err
					}
				}
			}

			if wantsDisks {
				askRetry("Retry selecting disks?", autoSetup, func() error {
					cephDisks, err = askRemotePool(availableDisks, autoSetup, wipeAllDisks, *ceph)

					return err
				})
			} else {
				// Add a space between the CLI and the response.
				fmt.Println("")
			}

			for peer, disks := range cephDisks {
				fmt.Printf(" Using %d disk(s) on %q for remote storage pool\n", len(disks), peer)
			}

			if len(cephDisks) > 0 {
				// Add a space between the CLI and the response.
				fmt.Println("")
			}
		}
	}

	if !bootstrap {
		sourceTemplate := lxdAPI.ClusterMemberConfigKey{
			Entity: "storage-pool",
			Name:   "remote",
			Key:    "source",
			Value:  "lxd_remote",
		}

		for peer := range cephDisks {
			diskConfig[peer] = append(diskConfig[peer], sourceTemplate)
		}
	}

	return diskConfig, cephDisks, nil
}

func askLocalPool(peerDisks map[string][]lxdAPI.ResourcesStorageDisk, autoSetup bool, wipeAllDisks bool, lxd service.LXDService) (map[string][]lxdAPI.ClusterMemberConfigKey, map[string]string, error) {
	data := [][]string{}
	selected := map[string]string{}
	for peer, disks := range peerDisks {
		// If there's no spare disk, then we can't add a remote storage pool, so skip local pool creation.
		if autoSetup && len(disks) < 2 {
			logger.Infof("Skipping local storage pool creation, peer %q has too few disks", peer)

			return nil, nil, nil
		}

		for _, disk := range disks {
			devicePath := fmt.Sprintf("/dev/%s", disk.ID)
			if disk.DeviceID != "" {
				devicePath = fmt.Sprintf("/dev/disk/by-id/%s", disk.DeviceID)
			} else if disk.DevicePath != "" {
				devicePath = fmt.Sprintf("/dev/disk/by-path/%s", disk.DevicePath)
			}

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
	wipeable, err := lxd.HasExtension(lxd.Name(), lxd.Address(), "", "storage_pool_source_wipe")
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to check for source.wipe extension: %w", err)
	}

	if !autoSetup {
		sort.Sort(cli.SortColumnsNaturally(data))
		header := []string{"LOCATION", "MODEL", "CAPACITY", "TYPE", "PATH"}
		table := NewSelectableTable(header, data)
		fmt.Println("Select exactly one disk from each cluster member:")
		table.Render(table.rows)
		selectedRows, err := table.GetSelections()
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to confirm local LXD disk selection: %w", err)
		}

		if len(selectedRows) == 0 {
			return nil, nil, fmt.Errorf("No disks selected")
		}

		for _, entry := range selectedRows {
			target := table.SelectionValue(entry, "LOCATION")
			path := table.SelectionValue(entry, "PATH")

			_, ok := selected[target]
			if ok {
				return nil, nil, fmt.Errorf("Failed to add local storage pool: Selected more than one disk for target peer %q", target)
			}

			selected[target] = path
		}

		if !wipeAllDisks && wipeable {
			fmt.Println("Select which disks to wipe:")
			table.Render(selectedRows)
			wipeRows, err := table.GetSelections()
			if err != nil {
				return nil, nil, fmt.Errorf("Failed to confirm which disks to wipe: %w", err)
			}

			for _, entry := range wipeRows {
				target := table.SelectionValue(entry, "LOCATION")
				path := table.SelectionValue(entry, "PATH")
				toWipe[target] = path
			}
		}
	}

	if len(selected) == 0 {
		return nil, nil, nil
	}

	if len(selected) != len(peerDisks) {
		return nil, nil, fmt.Errorf("Failed to add local storage pool: Some peers don't have an available disk")
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
				return nil, nil, fmt.Errorf("Failed to add pending local storage pool on peer %q: %w", target, err)
			}
		} else {
			sourceTemplate.Value = path
			memberConfig[target] = []lxdAPI.ClusterMemberConfigKey{sourceTemplate}
			if toWipe[target] != "" {
				memberConfig[target] = append(memberConfig[target], wipeDisk)
			}
		}
	}

	return memberConfig, selected, nil
}

func askRemotePool(peerDisks map[string][]lxdAPI.ResourcesStorageDisk, autoSetup bool, wipeAllDisks bool, ceph service.CephService) (map[string][]cephTypes.DisksPost, error) {
	header := []string{"LOCATION", "MODEL", "CAPACITY", "TYPE", "PATH"}
	data := [][]string{}
	for peer, disks := range peerDisks {
		for _, disk := range disks {
			// Skip any disks that have been reserved for the local storage pool.
			devicePath := fmt.Sprintf("/dev/%s", disk.ID)
			if disk.DeviceID != "" {
				devicePath = fmt.Sprintf("/dev/disk/by-id/%s", disk.DeviceID)
			} else if disk.DevicePath != "" {
				devicePath = fmt.Sprintf("/dev/disk/by-path/%s", disk.DevicePath)
			}

			data = append(data, []string{peer, disk.Model, units.GetByteSizeStringIEC(int64(disk.Size), 2), disk.Type, devicePath})
		}
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("Found no available disks")
	}

	sort.Sort(cli.SortColumnsNaturally(data))
	table := NewSelectableTable(header, data)
	selected := table.rows
	var toWipe []string
	if wipeAllDisks {
		toWipe = selected
	}

	if len(table.rows) == 0 {
		return nil, nil
	}

	if !autoSetup {
		fmt.Println("Select from the available unpartitioned disks:")
		var err error
		table.Render(table.rows)
		selected, err = table.GetSelections()
		if err != nil {
			return nil, fmt.Errorf("Failed to confirm disk selection: %w", err)
		}

		if len(selected) > 0 && !wipeAllDisks {
			fmt.Println("Select which disks to wipe:")
			table.Render(selected)
			toWipe, err = table.GetSelections()
			if err != nil {
				return nil, fmt.Errorf("Failed to confirm disk wipe selection: %w", err)
			}
		}
	}

	wipeMap := make(map[string]bool, len(toWipe))
	for _, entry := range toWipe {
		_, ok := table.data[entry]
		if ok {
			wipeMap[entry] = true
		}
	}

	diskMap := map[string][]cephTypes.DisksPost{}
	for _, entry := range selected {
		target := table.SelectionValue(entry, "LOCATION")
		path := table.SelectionValue(entry, "PATH")

		_, ok := diskMap[target]
		if !ok {
			diskMap[target] = []cephTypes.DisksPost{}
		}

		diskMap[target] = append(diskMap[target], cephTypes.DisksPost{Path: path, Wipe: wipeMap[entry]})
	}

	_, checkMinSize := peerDisks[ceph.Name()]
	if !checkMinSize || len(diskMap) >= 3 {
		return diskMap, nil
	}

	return nil, fmt.Errorf("Unable to add remote storage pool: Each peer (minimum 3) must have allocated disks")
}

func askNetwork(sh *service.ServiceHandler, peers map[string]mdns.ServerInfo, lxdConfig map[string][]api.ClusterMemberConfigKey, bootstrap bool, autoSetup bool) (map[string]string, map[string]string, error) {
	// Automatic setup gets a basic fan setup.
	if autoSetup {
		return nil, nil, nil
	}

	// Environments without OVN get a basic fan setup.
	if sh.Services[types.MicroOVN] == nil {
		return nil, nil, nil
	}

	// Get the list of networks from all peers.
	networks, err := sh.Services[types.LXD].(*service.LXDService).GetUplinkInterfaces(bootstrap, peers)
	if err != nil {
		return nil, nil, err
	}

	// Check if OVN is possible in the environment.
	canOVN := len(networks) > 0
	for _, nets := range networks {
		if len(nets) == 0 {
			canOVN = false
			break
		}
	}

	if !canOVN {
		fmt.Println("No dedicated uplink interfaces detected, skipping distributed networking")
		return nil, nil, nil
	}

	// Ask the user if they want OVN.
	wantsOVN, err := cli.AskBool("Configure distributed networking? (yes/no) [default=yes]: ", "yes")
	if err != nil {
		return nil, nil, err
	}

	if !wantsOVN {
		return nil, nil, nil
	}

	missingSystems := len(peers) != len(networks)
	if bootstrap {
		missingSystems = len(peers) != len(networks)-1
	}

	for _, nets := range networks {
		if len(nets) == 0 {
			missingSystems = true
			break
		}
	}

	if missingSystems {
		wantsSkip, err := cli.AskBool("Some systems are ineligible for distributed networking. Continue anyway? (yes/no) [default=yes]: ", "yes")
		if err != nil {
			return nil, nil, err
		}

		if !wantsSkip {
			return nil, nil, nil
		}
	}

	// Uplink selection table.
	header := []string{"LOCATION", "IFACE", "TYPE"}
	fmt.Println("Select exactly one network interface from each cluster member:")
	data := [][]string{}
	for peer, nets := range networks {
		for _, net := range nets {
			data = append(data, []string{peer, net.Name, net.Type})
		}
	}

	table := NewSelectableTable(header, data)
	var selected map[string]string
	askRetry("Retry selecting uplink interfaces?", autoSetup, func() error {
		table.Render(table.rows)
		answers, err := table.GetSelections()
		if err != nil {
			return err
		}

		selected = map[string]string{}
		for _, answer := range answers {
			target := table.SelectionValue(answer, "LOCATION")
			iface := table.SelectionValue(answer, "IFACE")

			if selected[target] != "" {
				return fmt.Errorf("Failed to add OVN uplink network: Selected more than one interface for target %q", target)
			}

			selected[target] = iface
		}

		if len(selected) != len(networks) {
			return fmt.Errorf("Failed to add OVN uplink network: Some peers don't have a selected interface")
		}

		return nil
	})

	for peer, iface := range selected {
		fmt.Printf(" Using %q on %q for OVN uplink\n", iface, peer)
	}

	if len(selected) > 0 {
		// Add a space between the CLI and the response.
		fmt.Println("")
	}

	// Prepare the configuration.
	config := map[string]string{}
	if bootstrap {
		for _, ip := range []string{"IPv4", "IPv6"} {
			validator := func(s string) error {
				if s == "" {
					return nil
				}

				addr, _, err := net.ParseCIDR(s)
				if err != nil {
					return err
				}

				if addr.To4() == nil && ip == "IPv4" {
					return fmt.Errorf("Not a valid IPv4")
				}

				if addr.To4() != nil && ip == "IPv6" {
					return fmt.Errorf("Not a valid IPv6")
				}

				return nil
			}

			msg := fmt.Sprintf("Specify the %s gateway (CIDR) on the uplink network (empty to skip %s): ", ip, ip)
			gateway, err := cli.AskString(msg, "", validator)
			if err != nil {
				return nil, nil, err
			}

			if gateway != "" {
				validator := func(s string) error {
					addr := net.ParseIP(s)
					if addr == nil {
						return fmt.Errorf("Invalid IP address %q", s)
					}

					if addr.To4() == nil && ip == "IPv4" {
						return fmt.Errorf("Not a valid IPv4")
					}

					if addr.To4() != nil && ip == "IPv6" {
						return fmt.Errorf("Not a valid IPv6")
					}

					return nil
				}

				if ip == "IPv4" {
					rangeStart, err := cli.AskString(fmt.Sprintf("Specify the first %s address in the range to use with LXD: ", ip), "", validator)
					if err != nil {
						return nil, nil, err
					}

					rangeEnd, err := cli.AskString(fmt.Sprintf("Specify the last %s address in the range to use with LXD: ", ip), "", validator)
					if err != nil {
						return nil, nil, err
					}

					config[gateway] = fmt.Sprintf("%s-%s", rangeStart, rangeEnd)
				} else {
					config[gateway] = ""
				}
			}
		}
	}

	if !bootstrap {
		if lxdConfig == nil {
			lxdConfig = map[string][]lxdAPI.ClusterMemberConfigKey{}
		}

		if len(selected) != 0 {
			for peer, parent := range selected {
				config, ok := lxdConfig[peer]
				if !ok {
					config = []api.ClusterMemberConfigKey{}
				}

				config = append(config, api.ClusterMemberConfigKey{
					Entity: "network",
					Name:   "UPLINK",
					Key:    "parent",
					Value:  parent,
				})

				lxdConfig[peer] = config
			}
		}
	}

	return selected, config, nil
}
