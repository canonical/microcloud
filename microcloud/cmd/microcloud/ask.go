package main

import (
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/canonical/lxd/shared/api"
	cli "github.com/canonical/lxd/shared/cmd"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/lxd/shared/units"
	cephTypes "github.com/canonical/microceph/microceph/api/types"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/mdns"
	"github.com/canonical/microcloud/microcloud/service"
)

// askRetry will print all errors and re-attempt the given function on user input.
func (c *CmdControl) askRetry(question string, autoSetup bool, f func() error) {
	for {
		retry := false
		err := f()
		if err != nil {
			fmt.Println(err)

			if !autoSetup {
				retry, err = c.asker.AskBool(fmt.Sprintf("%s (yes/no) [default=yes]: ", question), "yes")
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

func (c *CmdControl) askMissingServices(services []types.ServiceType, stateDirs map[types.ServiceType]string, autoSetup bool) ([]types.ServiceType, error) {
	missingServices := []string{}
	for serviceType, stateDir := range stateDirs {
		if service.Exists(serviceType, stateDir) {
			services = append(services, serviceType)
		} else {
			missingServices = append(missingServices, string(serviceType))
		}
	}

	if len(missingServices) > 0 {
		serviceStr := strings.Join(missingServices, ", ")
		if !autoSetup {
			confirm, err := c.asker.AskBool(fmt.Sprintf("%s not found. Continue anyway? (yes/no) [default=yes]: ", serviceStr), "yes")
			if err != nil {
				return nil, err
			}

			if !confirm {
				return nil, fmt.Errorf("User aborted")
			}

			return services, nil
		}

		logger.Infof("Skipping %s (could not detect service state directory)", serviceStr)
	}

	return services, nil
}

func (c *CmdControl) askAddress(autoSetup bool, listenAddr string) (string, *net.IPNet, error) {
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
			c.askRetry("Retry selecting an address?", autoSetup, func() error {
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
		filter, err := c.asker.AskBool(fmt.Sprintf("Limit search for other MicroCloud servers to %s? (yes/no) [default=yes]: ", subnet.String()), "yes")
		if err != nil {
			return "", nil, err
		}

		if !filter {
			subnet = nil
		}
	}

	return listenAddr, subnet, nil
}

func (c *CmdControl) askDisks(sh *service.Handler, systems map[string]InitSystem, autoSetup bool, wipeAllDisks bool) error {
	_, bootstrap := systems[sh.Name]
	allResources := make(map[string]*api.Resources, len(systems))
	var err error
	for peer, system := range systems {
		allResources[peer], err = sh.Services[types.LXD].(*service.LXDService).GetResources(peer, system.ServerInfo.Address, system.ServerInfo.AuthSecret)
		if err != nil {
			return fmt.Errorf("Failed to get system resources of peer %q: %w", peer, err)
		}
	}

	foundDisks := false
	for peer, r := range allResources {
		system := systems[peer]
		system.AvailableDisks = make([]api.ResourcesStorageDisk, 0, len(r.Storage.Disks))
		for _, disk := range r.Storage.Disks {
			if len(disk.Partitions) == 0 {
				system.AvailableDisks = append(system.AvailableDisks, disk)
			}
		}

		if len(system.AvailableDisks) > 0 {
			foundDisks = true
		}

		systems[peer] = system
	}

	wantsDisks := true
	if !autoSetup && foundDisks {
		wantsDisks, err = c.asker.AskBool("Would you like to set up local storage? (yes/no) [default=yes]: ", "yes")
		if err != nil {
			return err
		}
	}

	if !foundDisks {
		wantsDisks = false
	}

	lxd := sh.Services[types.LXD].(*service.LXDService)
	if wantsDisks {
		c.askRetry("Retry selecting disks?", autoSetup, func() error {
			return askLocalPool(systems, autoSetup, wipeAllDisks, *lxd)
		})
	}

	if sh.Services[types.MicroCeph] != nil {
		availableDisks := map[string][]api.ResourcesStorageDisk{}
		for peer, system := range systems {
			if len(system.AvailableDisks) > 0 {
				availableDisks[peer] = system.AvailableDisks
			}
		}

		if bootstrap && len(availableDisks) < 3 {
			fmt.Println("Insufficient number of disks available to set up distributed storage, skipping at this time")
		} else {
			wantsDisks = true
			if !autoSetup {
				wantsDisks, err = c.asker.AskBool("Would you like to set up distributed storage? (yes/no) [default=yes]: ", "yes")
				if err != nil {
					return err
				}

				if len(systems) != len(availableDisks) && wantsDisks {
					wantsDisks, err = c.asker.AskBool("Unable to find disks on some systems. Continue anyway? (yes/no) [default=yes]: ", "yes")
					if err != nil {
						return err
					}
				}
			}

			if wantsDisks {
				c.askRetry("Retry selecting disks?", autoSetup, func() error {
					return askRemotePool(systems, autoSetup, wipeAllDisks, sh)
				})
			} else {
				// Add a space between the CLI and the response.
				fmt.Println("")
			}

			usingCeph := false
			for peer, system := range systems {
				if len(system.MicroCephDisks) > 0 {
					usingCeph = true
					fmt.Printf(" Using %d disk(s) on %q for remote storage pool\n", len(system.MicroCephDisks), peer)
				}
			}

			if usingCeph {
				// Add a space between the CLI and the response.
				fmt.Println("")
			}
		}
	}

	if !bootstrap {
		for peer, system := range systems {
			if len(system.MicroCephDisks) > 0 {
				if system.JoinConfig == nil {
					system.JoinConfig = []api.ClusterMemberConfigKey{}
				}

				system.JoinConfig = append(system.JoinConfig, lxd.DefaultCephStoragePoolJoinConfig())

				systems[peer] = system
			}
		}
	}

	return nil
}

func parseDiskPath(disk api.ResourcesStorageDisk) string {
	devicePath := fmt.Sprintf("/dev/%s", disk.ID)
	if disk.DeviceID != "" {
		devicePath = fmt.Sprintf("/dev/disk/by-id/%s", disk.DeviceID)
	} else if disk.DevicePath != "" {
		devicePath = fmt.Sprintf("/dev/disk/by-path/%s", disk.DevicePath)
	}

	return devicePath
}

func askLocalPool(systems map[string]InitSystem, autoSetup bool, wipeAllDisks bool, lxd service.LXDService) error {
	data := [][]string{}
	selected := map[string]string{}
	for peer, system := range systems {
		// If there's no spare disk, then we can't add a remote storage pool, so skip local pool creation.
		if autoSetup && len(system.AvailableDisks) < 2 {
			logger.Infof("Skipping local storage pool creation, peer %q has too few disks", peer)

			return nil
		}

		for _, disk := range system.AvailableDisks {
			devicePath := parseDiskPath(disk)
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
		return fmt.Errorf("Failed to check for source.wipe extension: %w", err)
	}

	if !autoSetup {
		sort.Sort(cli.SortColumnsNaturally(data))
		header := []string{"LOCATION", "MODEL", "CAPACITY", "TYPE", "PATH"}
		table := NewSelectableTable(header, data)
		fmt.Println("Select exactly one disk from each cluster member:")
		table.Render(table.rows)
		selectedRows, err := table.GetSelections()
		if err != nil {
			return fmt.Errorf("Failed to confirm local LXD disk selection: %w", err)
		}

		if len(selectedRows) == 0 {
			return fmt.Errorf("No disks selected")
		}

		for _, entry := range selectedRows {
			target := table.SelectionValue(entry, "LOCATION")
			path := table.SelectionValue(entry, "PATH")

			_, ok := selected[target]
			if ok {
				return fmt.Errorf("Failed to add local storage pool: Selected more than one disk for target peer %q", target)
			}

			selected[target] = path
		}

		if !wipeAllDisks && wipeable {
			fmt.Println("Select which disks to wipe:")
			table.Render(selectedRows)
			wipeRows, err := table.GetSelections()
			if err != nil {
				return fmt.Errorf("Failed to confirm which disks to wipe: %w", err)
			}

			for _, entry := range wipeRows {
				target := table.SelectionValue(entry, "LOCATION")
				path := table.SelectionValue(entry, "PATH")
				toWipe[target] = path
			}
		}
	}

	if len(selected) == 0 {
		return nil
	}

	if len(selected) != len(systems) {
		return fmt.Errorf("Failed to add local storage pool: Some peers don't have an available disk")
	}

	if wipeAllDisks && wipeable {
		toWipe = selected
	}

	_, bootstrap := systems[lxd.Name()]
	for target, path := range selected {
		system := systems[target]
		if bootstrap {
			system.TargetStoragePools = []api.StoragePoolsPost{lxd.DefaultPendingZFSStoragePool(wipeable && toWipe[target] != "", path)}
			if target == lxd.Name() {
				system.StoragePools = []api.StoragePoolsPost{lxd.DefaultZFSStoragePool()}
			}
		} else {
			system.JoinConfig = lxd.DefaultZFSStoragePoolJoinConfig(wipeable && toWipe[target] != "", path)
		}

		// Remove the disks that we selected.
		remainingDisks := make([]api.ResourcesStorageDisk, 0, len(system.AvailableDisks)-1)
		for _, disk := range system.AvailableDisks {
			if parseDiskPath(disk) != path {
				remainingDisks = append(remainingDisks, disk)
			}
		}

		system.AvailableDisks = remainingDisks

		systems[target] = system

		fmt.Printf(" Using %q on %q for local storage pool\n", path, target)
	}

	if len(selected) > 0 {
		// Add a space between the CLI and the response.
		fmt.Println("")
	}

	return nil
}

func askRemotePool(systems map[string]InitSystem, autoSetup bool, wipeAllDisks bool, sh *service.Handler) error {
	header := []string{"LOCATION", "MODEL", "CAPACITY", "TYPE", "PATH"}
	data := [][]string{}
	for peer, system := range systems {
		for _, disk := range system.AvailableDisks {
			// Skip any disks that have been reserved for the local storage pool.
			devicePath := parseDiskPath(disk)
			data = append(data, []string{peer, disk.Model, units.GetByteSizeStringIEC(int64(disk.Size), 2), disk.Type, devicePath})
		}
	}

	if len(data) == 0 {
		return fmt.Errorf("Found no available disks")
	}

	sort.Sort(cli.SortColumnsNaturally(data))
	table := NewSelectableTable(header, data)
	selected := table.rows
	var toWipe []string
	if wipeAllDisks {
		toWipe = selected
	}

	if len(table.rows) == 0 {
		return nil
	}

	if !autoSetup {
		fmt.Println("Select from the available unpartitioned disks:")
		var err error
		table.Render(table.rows)
		selected, err = table.GetSelections()
		if err != nil {
			return fmt.Errorf("Failed to confirm disk selection: %w", err)
		}

		if len(selected) > 0 && !wipeAllDisks {
			fmt.Println("Select which disks to wipe:")
			table.Render(selected)
			toWipe, err = table.GetSelections()
			if err != nil {
				return fmt.Errorf("Failed to confirm disk wipe selection: %w", err)
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

	diskCount := 0
	lxd := sh.Services[types.LXD].(*service.LXDService)
	for _, entry := range selected {
		target := table.SelectionValue(entry, "LOCATION")
		path := table.SelectionValue(entry, "PATH")
		system := systems[target]

		if system.MicroCephDisks == nil {
			diskCount++
			system.MicroCephDisks = []cephTypes.DisksPost{}
		}

		system.MicroCephDisks = append(system.MicroCephDisks, cephTypes.DisksPost{Path: path, Wipe: wipeMap[entry]})

		systems[target] = system
	}

	if diskCount > 0 {
		for target, system := range systems {
			if system.TargetStoragePools == nil {
				system.TargetStoragePools = []api.StoragePoolsPost{}
			}

			_, bootstrap := systems[sh.Name]
			if bootstrap {
				system.TargetStoragePools = append(system.TargetStoragePools, lxd.DefaultPendingCephStoragePool())
				if target == sh.Name {
					if system.StoragePools == nil {
						system.StoragePools = []api.StoragePoolsPost{}
					}

					system.StoragePools = append(system.StoragePools, lxd.DefaultCephStoragePool())
				}
			}

			systems[target] = system
		}
	}

	_, checkMinSize := systems[sh.Name]
	if !checkMinSize || diskCount >= 3 {
		return nil
	}

	return fmt.Errorf("Unable to add remote storage pool: At least 3 peers must have allocated disks")
}

func (c *CmdControl) askNetwork(sh *service.Handler, systems map[string]InitSystem, autoSetup bool) error {
	_, bootstrap := systems[sh.Name]
	lxd := sh.Services[types.LXD].(*service.LXDService)
	for peer, system := range systems {
		if bootstrap {
			system.TargetNetworks = []api.NetworksPost{lxd.DefaultPendingFanNetwork()}
			if peer == sh.Name {
				network, err := lxd.DefaultFanNetwork()
				if err != nil {
					return err
				}

				system.Networks = []api.NetworksPost{network}
			}
		}

		systems[peer] = system
	}

	// Automatic setup gets a basic fan setup.
	if autoSetup {
		return nil
	}

	// Environments without OVN get a basic fan setup.
	if sh.Services[types.MicroOVN] == nil {
		return nil
	}

	// Get the list of networks from all peers.
	infos := []mdns.ServerInfo{}
	for _, system := range systems {
		infos = append(infos, system.ServerInfo)
	}

	networks, err := sh.Services[types.LXD].(*service.LXDService).GetUplinkInterfaces(bootstrap, infos)
	if err != nil {
		return err
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
		return nil
	}

	// Ask the user if they want OVN.
	wantsOVN, err := c.asker.AskBool("Configure distributed networking? (yes/no) [default=yes]: ", "yes")
	if err != nil {
		return err
	}

	if !wantsOVN {
		return nil
	}

	missingSystems := len(systems) != len(networks)
	for _, nets := range networks {
		if len(nets) == 0 {
			missingSystems = true
			break
		}
	}

	if missingSystems {
		wantsSkip, err := c.asker.AskBool("Some systems are ineligible for distributed networking. Continue anyway? (yes/no) [default=yes]: ", "yes")
		if err != nil {
			return err
		}

		if !wantsSkip {
			return nil
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
	c.askRetry("Retry selecting uplink interfaces?", autoSetup, func() error {
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
			gateway, err := c.asker.AskString(msg, "", validator)
			if err != nil {
				return err
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
					rangeStart, err := c.asker.AskString(fmt.Sprintf("Specify the first %s address in the range to use with LXD: ", ip), "", validator)
					if err != nil {
						return err
					}

					rangeEnd, err := c.asker.AskString(fmt.Sprintf("Specify the last %s address in the range to use with LXD: ", ip), "", validator)
					if err != nil {
						return err
					}

					config[gateway] = fmt.Sprintf("%s-%s", rangeStart, rangeEnd)
				} else {
					config[gateway] = ""
				}
			}
		}
	}

	// If interfaces were selected for OVN, remove the FAN config.
	if len(selected) > 0 {
		for peer, system := range systems {
			system.TargetNetworks = []api.NetworksPost{}
			system.Networks = []api.NetworksPost{}

			systems[peer] = system
		}
	}

	// If we are adding a new member, a MemberConfig entry should suffice to create the network on the node.
	for peer, parent := range selected {
		system := systems[peer]
		if !bootstrap {
			if system.JoinConfig == nil {
				system.JoinConfig = []api.ClusterMemberConfigKey{}
			}

			system.JoinConfig = append(system.JoinConfig, lxd.DefaultOVNNetworkJoinConfig(parent))
		} else {
			system.TargetNetworks = append(system.TargetNetworks, lxd.DefaultPendingOVNNetwork(parent))
		}

		systems[peer] = system
	}

	if bootstrap {
		bootstrapSystem := systems[sh.Name]

		var ipv4Gateway string
		var ipv4Ranges string
		var ipv6Gateway string
		for gateway, ipRange := range config {
			ip, _, err := net.ParseCIDR(gateway)
			if err != nil {
				return err
			}

			if ip.To4() != nil {
				ipv4Gateway = gateway
				ipv4Ranges = ipRange
			} else {
				ipv6Gateway = gateway
			}
		}

		uplink, ovn := lxd.DefaultOVNNetwork(ipv4Gateway, ipv4Ranges, ipv6Gateway)
		bootstrapSystem.Networks = []api.NetworksPost{uplink, ovn}
		systems[sh.Name] = bootstrapSystem
	}

	return nil
}
