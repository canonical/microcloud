package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	cli "github.com/canonical/lxd/shared/cmd"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/lxd/shared/units"
	"github.com/canonical/lxd/shared/validate"
	cephTypes "github.com/canonical/microceph/microceph/api/types"

	"github.com/canonical/microcloud/microcloud/api/types"
	cloudClient "github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcloud/microcloud/mdns"
	"github.com/canonical/microcloud/microcloud/service"
)

// askUpdateProfile asks whether to update the existing profile configuration if it has changed.
func (c *initConfig) askUpdateProfile(profile api.ProfilesPost, profiles []string, lxdClient lxd.InstanceServer) (*api.ProfilePut, error) {
	if !shared.ValueInSlice(profile.Name, profiles) {
		return &profile.ProfilePut, nil
	}

	// Ensure any pre-existing devices and config are carried over to the new profile, unless we are managing them.
	existingProfile, _, err := lxdClient.GetProfile("default")
	if err != nil {
		return nil, err
	}

	askConflictingConfig := []string{}
	askConflictingDevices := []string{}
	for k, v := range profile.Config {
		_, ok := existingProfile.Config[k]
		if !ok {
			existingProfile.Config[k] = v
		} else {
			askConflictingConfig = append(askConflictingConfig, k)
		}
	}

	for k, v := range profile.Devices {
		_, ok := existingProfile.Devices[k]
		if !ok {
			existingProfile.Devices[k] = v
		} else {
			askConflictingDevices = append(askConflictingDevices, k)
		}
	}

	if len(askConflictingConfig) > 0 || len(askConflictingDevices) > 0 {
		replace, err := c.asker.AskBool("Replace existing default profile configuration? (yes/no) [default=no]: ", "no")
		if err != nil {
			return nil, err
		}

		if replace {
			for _, key := range askConflictingConfig {
				existingProfile.Config[key] = profile.Config[key]
			}

			for _, key := range askConflictingDevices {
				existingProfile.Devices[key] = profile.Devices[key]
			}
		}
	}

	newProfile := existingProfile.Writable()

	return &newProfile, nil
}

// askRetry will print all errors and re-attempt the given function on user input.
func (c *initConfig) askRetry(question string, f func() error) error {
	for {
		retry := false
		err := f()
		if err != nil {
			fmt.Println(err)

			retry, err = c.asker.AskBool(fmt.Sprintf("%s (yes/no) [default=yes]: ", question), "yes")
			if err != nil {
				return err
			}
		}

		if !retry {
			return nil
		}
	}
}

func (c *initConfig) askMissingServices(services []types.ServiceType, stateDirs map[types.ServiceType]string) ([]types.ServiceType, error) {
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

		// Ignore missing services in case of preseed.
		if !c.autoSetup {
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

func (c *initConfig) askAddress(filterAddress string) error {
	info, err := mdns.GetNetworkInfo()
	if err != nil {
		return fmt.Errorf("Failed to find network interfaces: %w", err)
	}

	listenAddr := c.address
	if listenAddr == "" {
		if len(info) == 0 {
			return fmt.Errorf("Found no valid network interfaces")
		}

		filterIp := net.ParseIP(filterAddress)
		if filterAddress != "" && filterIp == nil {
			return fmt.Errorf("Invalid filter address %q", filterAddress)
		}

		listenAddr = info[0].Address
		if !c.autoSetup && len(info) > 1 {
			data := make([][]string, 0, len(info))
			for _, network := range info {
				// Filter out addresses which are not in the same network as the filter address.
				if filterAddress != "" && !network.Subnet.Contains(filterIp) {
					continue
				}

				data = append(data, []string{network.Address, network.Interface.Name})
			}

			table := NewSelectableTable([]string{"ADDRESS", "IFACE"}, data)
			err := c.askRetry("Retry selecting an address?", func() error {
				fmt.Println("Select an address for MicroCloud's internal traffic:")
				err := table.Render(table.rows)
				if err != nil {
					return err
				}

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
			if err != nil {
				return err
			}
		} else {
			fmt.Printf("Using address %q for MicroCloud\n", listenAddr)
		}
	}

	var subnet *net.IPNet
	var iface *net.Interface
	for _, network := range info {
		if network.Subnet.Contains(net.ParseIP(listenAddr)) {
			subnet = network.Subnet
			iface = &network.Interface
			break
		}
	}

	if subnet == nil {
		return fmt.Errorf("Cloud not find valid subnet for address %q", listenAddr)
	}

	c.address = listenAddr
	c.lookupIface = iface
	c.lookupSubnet = subnet

	return nil
}

func (c *initConfig) askDisks(sh *service.Handler) error {
	err := c.askLocalPool(sh)
	if err != nil {
		return err
	}

	err = c.askRemotePool(sh)
	if err != nil {
		return err
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

func (c *initConfig) askLocalPool(sh *service.Handler) error {
	useJoinConfig := false
	askSystems := map[string]bool{}
	for _, info := range c.state {
		hasPool, supportsPool := info.SupportsLocalPool()
		if !supportsPool {
			logger.Warn("Skipping local storage pool setup, some systems don't support it")
			return nil
		}

		if hasPool {
			useJoinConfig = true
		} else {
			askSystems[info.ClusterName] = true
		}
	}

	availableDisks := map[string]map[string]api.ResourcesStorageDisk{}
	for name, state := range c.state {
		if len(state.AvailableDisks) == 0 {
			logger.Infof("Skipping local storage pool creation, peer %q has too few disks", name)

			return nil
		}

		if askSystems[name] {
			availableDisks[name] = state.AvailableDisks
		}
	}

	// Local storage is already set up on every system.
	if len(askSystems) == 0 {
		return nil
	}

	// We can't setup a local pool if not every system has a disk.
	if len(availableDisks) != len(askSystems) {
		return nil
	}

	data := [][]string{}
	selectedDisks := map[string]string{}
	for peer, disks := range availableDisks {
		sortedDisks := []api.ResourcesStorageDisk{}
		for _, disk := range disks {
			sortedDisks = append(sortedDisks, disk)
		}

		sort.Slice(sortedDisks, func(i, j int) bool {
			return parseDiskPath(sortedDisks[i]) < parseDiskPath(sortedDisks[j])
		})

		for _, disk := range sortedDisks {
			devicePath := parseDiskPath(disk)
			data = append(data, []string{peer, disk.Model, units.GetByteSizeStringIEC(int64(disk.Size), 2), disk.Type, devicePath})
		}
	}

	wantsDisks, err := c.asker.AskBool("Would you like to set up local storage? (yes/no) [default=yes]: ", "yes")
	if err != nil {
		return err
	}

	if !wantsDisks {
		return nil
	}

	lxd := sh.Services[types.LXD].(*service.LXDService)
	toWipe := map[string]string{}
	wipeable, err := lxd.HasExtension(context.Background(), lxd.Name(), lxd.Address(), nil, "storage_pool_source_wipe")
	if err != nil {
		return fmt.Errorf("Failed to check for source.wipe extension: %w", err)
	}

	err = c.askRetry("Retry selecting disks?", func() error {
		selected := map[string]string{}
		sort.Sort(cli.SortColumnsNaturally(data))
		header := []string{"LOCATION", "MODEL", "CAPACITY", "TYPE", "PATH"}
		table := NewSelectableTable(header, data)
		fmt.Println("Select exactly one disk from each cluster member:")
		err := table.Render(table.rows)
		if err != nil {
			return err
		}

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

		if len(selected) != len(askSystems) {
			return fmt.Errorf("Failed to add local storage pool: Some peers don't have an available disk")
		}

		if !c.wipeAllDisks && wipeable {
			fmt.Println("Select which disks to wipe:")
			err := table.Render(selectedRows)
			if err != nil {
				return err
			}

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

		selectedDisks = selected

		return nil
	})
	if err != nil {
		return err
	}

	if len(selectedDisks) == 0 {
		return nil
	}

	if c.wipeAllDisks && wipeable {
		toWipe = selectedDisks
	}

	var joinConfigs map[string][]api.ClusterMemberConfigKey
	var finalConfigs []api.StoragePoolsPost
	var targetConfigs map[string][]api.StoragePoolsPost
	if useJoinConfig {
		joinConfigs = map[string][]api.ClusterMemberConfigKey{}
		for target, path := range selectedDisks {
			joinConfigs[target] = lxd.DefaultZFSStoragePoolJoinConfig(wipeable && toWipe[target] != "", path)
		}
	} else {
		targetConfigs = map[string][]api.StoragePoolsPost{}
		for target, path := range selectedDisks {
			targetConfigs[target] = []api.StoragePoolsPost{lxd.DefaultPendingZFSStoragePool(wipeable && toWipe[target] != "", path)}
		}

		if len(targetConfigs) > 0 {
			finalConfigs = []api.StoragePoolsPost{lxd.DefaultZFSStoragePool()}
		}
	}

	for target, path := range selectedDisks {
		fmt.Printf(" Using %q on %q for local storage pool\n", path, target)
	}

	if len(selectedDisks) > 0 {
		// Add a space between the CLI and the response.
		fmt.Println("")
	}

	newAvailableDisks := map[string]map[string]api.ResourcesStorageDisk{}
	for target, path := range selectedDisks {
		newAvailableDisks[target] = map[string]api.ResourcesStorageDisk{}
		for id, disk := range availableDisks[target] {
			if parseDiskPath(disk) != path {
				newAvailableDisks[target][id] = disk
			}
		}
	}

	for peer, system := range c.systems {
		if !askSystems[peer] {
			continue
		}

		if system.JoinConfig == nil {
			system.JoinConfig = []api.ClusterMemberConfigKey{}
		}

		if system.TargetStoragePools == nil {
			system.TargetStoragePools = []api.StoragePoolsPost{}
		}

		if system.StoragePools == nil {
			system.StoragePools = []api.StoragePoolsPost{}
		}

		if joinConfigs[peer] != nil {
			system.JoinConfig = append(system.JoinConfig, joinConfigs[peer]...)
		}

		if targetConfigs[peer] != nil {
			system.TargetStoragePools = append(system.TargetStoragePools, targetConfigs[peer]...)
		}

		if peer == sh.Name && finalConfigs != nil {
			system.StoragePools = append(system.StoragePools, finalConfigs...)
		}

		c.systems[peer] = system
	}

	for peer, state := range c.state {
		if askSystems[peer] {
			state.AvailableDisks = newAvailableDisks[peer]
			c.state[peer] = state
		}
	}

	return nil
}

func validateCephInterfacesForSubnet(lxdService *service.LXDService, systems map[string]InitSystem, availableCephNetworkInterfaces map[string]map[string]service.DedicatedInterface, askedCephSubnet string) error {
	validatedCephInterfacesData, err := lxdService.ValidateCephInterfaces(askedCephSubnet, availableCephNetworkInterfaces)
	if err != nil {
		return err
	}

	// List the detected network interfaces
	for _, interfaces := range validatedCephInterfacesData {
		for _, iface := range interfaces {
			fmt.Printf("Interface %q (%q) detected on cluster member %q\n", iface[1], iface[2], iface[0])
		}
	}

	// Even though not all the cluster members might have OSDs,
	// we check that all the machines have at least one interface to sustain the Ceph network
	for systemName := range systems {
		if len(validatedCephInterfacesData[systemName]) == 0 {
			return fmt.Errorf("Not enough network interfaces found with an IP within the given CIDR subnet on %q.\nYou need at least one interface per cluster member.", systemName)
		}
	}

	return nil
}

// getTargetCephNetworks fetches the Ceph network configuration from the existing Ceph cluster.
// If the system passed as an argument is nil, we will fetch the local Ceph network configuration.
func getTargetCephNetworks(sh *service.Handler, s *InitSystem) (internalCephNetwork *net.IPNet, err error) {
	microCephService := sh.Services[types.MicroCeph].(*service.CephService)
	if microCephService == nil {
		return nil, fmt.Errorf("Failed to get MicroCeph service")
	}

	var cephAddr string
	var cephCert *x509.Certificate
	if s != nil && s.ServerInfo.Name != sh.Name {
		cephAddr = s.ServerInfo.Address
		cephCert = s.ServerInfo.Certificate
	}

	remoteCephConfigs, err := microCephService.ClusterConfig(context.Background(), cephAddr, cephCert)
	if err != nil {
		return nil, err
	}

	for key, value := range remoteCephConfigs {
		if key == "cluster_network" && value != "" {
			// Sometimes, the default cluster_network value in the Ceph configuration
			// is not a network range but a regular IP address. We need to extract the network range.
			_, valueNet, err := net.ParseCIDR(value)
			if err != nil {
				return nil, fmt.Errorf("Failed to parse the Ceph cluster network configuration from the existing Ceph cluster: %v", err)
			}

			internalCephNetwork = valueNet
		}
	}

	return internalCephNetwork, nil
}

func (c *initConfig) askRemotePool(sh *service.Handler) error {
	// If MicroCeph is not installed, skip this block entirely.
	if sh.Services[types.MicroCeph] == nil {
		return nil
	}

	// Check if we need to use JoinConfig because the storage pools and networks are already set up.
	// Select the systems that don't have the corresponding storage pools, and only ask questions for those systems.
	useJoinConfigRemote := false
	useJoinConfigRemoteFS := false
	askSystemsRemote := map[string]bool{}
	askSystemsRemoteFS := map[string]bool{}
	for _, info := range c.state {
		hasPool, supportsPool := info.SupportsRemotePool()
		if !supportsPool {
			logger.Warn("Skipping remote storage pool setup, some systems don't support it")
			return nil
		}

		hasFSPool, supportsFSPool := info.SupportsRemoteFSPool()
		if !supportsFSPool {
			logger.Warn("Skipping remote-fs storage pool setup, some systems don't support it")
			return nil
		}

		if !hasPool && hasFSPool {
			return fmt.Errorf("Unsupported configuration, remote-fs pool already exists")
		}

		if hasPool {
			useJoinConfigRemote = true
		} else {
			askSystemsRemote[info.ClusterName] = true
		}

		if hasFSPool {
			useJoinConfigRemoteFS = true
		} else {
			askSystemsRemoteFS[info.ClusterName] = true
		}
	}
	var selectedDisks map[string][]string
	var wipeDisks map[string]map[string]bool
	availableDiskCount := 0
	if len(askSystemsRemote) != 0 {
		availableDisks := map[string]map[string]api.ResourcesStorageDisk{}
		for name, state := range c.state {
			if askSystemsRemote[name] {
				availableDisks[name] = state.AvailableDisks

				if len(state.AvailableDisks) > 0 {
					availableDiskCount++
				}
			}
		}

		if availableDiskCount == 0 {
			fmt.Println("No disks available for distributed storage. Skipping configuration")

			return nil
		}

		wantsDisks, err := c.asker.AskBool("Would you like to set up distributed storage? (yes/no) [default=yes]: ", "yes")
		if err != nil {
			return err
		}

		// Ask if the user is okay with fully remote ceph on some systems.
		if len(askSystemsRemote) != availableDiskCount && wantsDisks {
			wantsDisks, err = c.asker.AskBool("Unable to find disks on some systems. Continue anyway? (yes/no) [default=yes]: ", "yes")
			if err != nil {
				return err
			}
		}

		if !wantsDisks {
			return nil
		}

		err = c.askRetry("Change disk selection?", func() error {
			selectedDisks = map[string][]string{}
			wipeDisks = map[string]map[string]bool{}
			header := []string{"LOCATION", "MODEL", "CAPACITY", "TYPE", "PATH"}
			data := [][]string{}
			for peer, disks := range availableDisks {
				sortedDisks := []api.ResourcesStorageDisk{}
				for _, disk := range disks {
					sortedDisks = append(sortedDisks, disk)
				}

				// Ensure the list of disks is sorted by name.
				sort.Slice(sortedDisks, func(i, j int) bool {
					return parseDiskPath(sortedDisks[i]) < parseDiskPath(sortedDisks[j])
				})

				for _, disk := range sortedDisks {
					// Skip any disks that have been reserved for the local storage pool.
					devicePath := parseDiskPath(disk)
					data = append(data, []string{peer, disk.Model, units.GetByteSizeStringIEC(int64(disk.Size), 2), disk.Type, devicePath})
				}
			}

			if len(data) == 0 {
				return fmt.Errorf("Invalid disk configuration. Found no available disks")
			}

			sort.Sort(cli.SortColumnsNaturally(data))
			table := NewSelectableTable(header, data)
			selected := table.rows
			var toWipe []string
			if c.wipeAllDisks {
				toWipe = selected
			}

			if len(table.rows) == 0 {
				return nil
			}

			fmt.Println("Select from the available unpartitioned disks:")
			err := table.Render(table.rows)
			if err != nil {
				return err
			}

			selected, err = table.GetSelections()
			if err != nil {
				return fmt.Errorf("Invalid disk configuration: %w", err)
			}

			if len(selected) > 0 && !c.wipeAllDisks {
				fmt.Println("Select which disks to wipe:")
				err := table.Render(selected)
				if err != nil {
					return err
				}

				toWipe, err = table.GetSelections()
				if err != nil {
					return fmt.Errorf("Invalid disk configuration: %w", err)
				}
			}

			targetDisks := map[string][]string{}
			for _, entry := range selected {
				target := table.SelectionValue(entry, "LOCATION")
				path := table.SelectionValue(entry, "PATH")
				if targetDisks[target] == nil {
					targetDisks[target] = []string{}
				}

				targetDisks[target] = append(targetDisks[target], path)
			}

			wipeDisks = map[string]map[string]bool{}
			for _, entry := range toWipe {
				target := table.SelectionValue(entry, "LOCATION")
				path := table.SelectionValue(entry, "PATH")
				if wipeDisks[target] == nil {
					wipeDisks[target] = map[string]bool{}
				}

				wipeDisks[target][path] = true
			}

			selectedDisks = targetDisks

			if len(targetDisks) == 0 {
				return fmt.Errorf("No disks were selected")
			}

			insufficientDisks := !useJoinConfigRemote && len(targetDisks) < RecommendedOSDHosts

			if insufficientDisks {
				// This error will be printed to STDOUT as a normal message, so it includes a new-line for readability.
				return fmt.Errorf("Disk configuration does not meet recommendations for fault tolerance. At least %d systems must supply disks.\nContinuing with this configuration will leave MicroCloud susceptible to data loss", RecommendedOSDHosts)
			}

			return nil
		})
		if err != nil {
			return err
		}

		if len(selectedDisks) == 0 {
			return nil
		}

		for target, disks := range selectedDisks {
			if len(disks) > 0 {
				fmt.Printf(" Using %d disk(s) on %q for remote storage pool\n", len(disks), target)
			}
		}

		if len(selectedDisks) > 0 {
			fmt.Println()
		}
	}

	encryptDisks := c.encryptAllDisks
	if !c.encryptAllDisks && len(selectedDisks) > 0 {
		var err error
		encryptDisks, err = c.asker.AskBool("Do you want to encrypt the selected disks? (yes/no) [default=no]: ", "no")
		if err != nil {
			return err
		}
	}

	// If a cephfs pool has already been set up, we will extend it automatically, so no need to ask the question.
	setupCephFS := useJoinConfigRemoteFS
	if !useJoinConfigRemoteFS {
		lxd := sh.Services[types.LXD].(*service.LXDService)
		ext := "storage_cephfs_create_missing"
		hasCephFS, err := lxd.HasExtension(context.Background(), lxd.Name(), lxd.Address(), nil, ext)
		if err != nil {
			return fmt.Errorf("Failed to check for the %q LXD API extension: %w", ext, err)
		}

		if hasCephFS {
			setupCephFS, err = c.asker.AskBool("Would you like to set up CephFS remote storage? (yes/no) [default=yes]: ", "yes")
			if err != nil {
				return err
			}
		}
	}

	// Ask ceph networking questions last.
	err := c.askCephNetwork(sh)
	if err != nil {
		return err
	}

	osds := map[string][]cephTypes.DisksPost{}
	for target, disks := range selectedDisks {
		for _, disk := range disks {
			if osds[target] == nil {
				osds[target] = []cephTypes.DisksPost{}
			}

			osds[target] = append(osds[target], cephTypes.DisksPost{Path: []string{disk}, Wipe: wipeDisks[target][disk], Encrypt: encryptDisks})
		}
	}

	joinConfigs := map[string][]api.ClusterMemberConfigKey{}
	finalConfigs := []api.StoragePoolsPost{}
	targetConfigs := map[string][]api.StoragePoolsPost{}
	lxd := sh.Services[types.LXD].(*service.LXDService)
	if useJoinConfigRemote {
		for target := range askSystemsRemote {
			if joinConfigs[target] == nil {
				joinConfigs[target] = []api.ClusterMemberConfigKey{}
			}

			joinConfigs[target] = append(joinConfigs[target], lxd.DefaultCephStoragePoolJoinConfig())
		}
	} else {
		for target := range askSystemsRemote {
			if targetConfigs[target] == nil {
				targetConfigs[target] = []api.StoragePoolsPost{}
			}

			targetConfigs[target] = append(targetConfigs[target], lxd.DefaultPendingCephStoragePool())
		}

		if len(targetConfigs) > 0 {
			finalConfigs = append(finalConfigs, lxd.DefaultCephStoragePool())
		}
	}

	if useJoinConfigRemoteFS {
		for target := range askSystemsRemoteFS {
			if joinConfigs[target] == nil {
				joinConfigs[target] = []api.ClusterMemberConfigKey{}
			}

			joinConfigs[target] = append(joinConfigs[target], lxd.DefaultCephFSStoragePoolJoinConfig())
		}
	} else if setupCephFS {
		for target := range askSystemsRemoteFS {
			if targetConfigs[target] == nil {
				targetConfigs[target] = []api.StoragePoolsPost{}
			}

			targetConfigs[target] = append(targetConfigs[target], lxd.DefaultPendingCephFSStoragePool())
		}

		if len(targetConfigs) > 0 {
			finalConfigs = append(finalConfigs, lxd.DefaultCephFSStoragePool())
		}
	}

	for peer, system := range c.systems {
		if system.JoinConfig == nil {
			system.JoinConfig = []api.ClusterMemberConfigKey{}
		}

		if system.TargetStoragePools == nil {
			system.TargetStoragePools = []api.StoragePoolsPost{}
		}

		if system.StoragePools == nil {
			system.StoragePools = []api.StoragePoolsPost{}
		}

		if system.MicroCephDisks == nil {
			system.MicroCephDisks = []cephTypes.DisksPost{}
		}

		if joinConfigs[peer] != nil {
			system.JoinConfig = append(system.JoinConfig, joinConfigs[peer]...)
		}

		if targetConfigs[peer] != nil {
			system.TargetStoragePools = append(system.TargetStoragePools, targetConfigs[peer]...)
		}

		if osds[peer] != nil {
			system.MicroCephDisks = append(system.MicroCephDisks, osds[peer]...)
		}

		if peer == sh.Name && finalConfigs != nil {
			system.StoragePools = append(system.StoragePools, finalConfigs...)
		}

		c.systems[peer] = system
	}

	return nil
}

func (c *initConfig) askOVNNetwork(sh *service.Handler) error {
	if sh.Services[types.MicroOVN] == nil {
		return nil
	}

	useOVNJoinConfig := false
	askSystems := map[string]bool{}
	for _, state := range c.state {
		hasOVN, supportsOVN := state.SupportsOVNNetwork()
		if !supportsOVN || len(state.AvailableUplinkInterfaces) == 0 {
			logger.Warn("Skipping OVN network setup, some systems don't support it")
			return nil
		}

		if hasOVN {
			useOVNJoinConfig = true
		} else {
			askSystems[state.ClusterName] = true
		}
	}

	if len(askSystems) == 0 {
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

	for name, state := range c.state {
		if !askSystems[name] {
			continue
		}

		if len(state.AvailableUplinkInterfaces) == 0 {
			wantsContinue, err := c.asker.AskBool("Some systems are ineligible for distributed networking, which requires either an interface with no IPs assigned or a bridge. Continue anyway? (yes/no) [default=yes]: ", "yes")
			if err != nil {
				return err
			}

			if wantsContinue {
				return nil
			}

			return fmt.Errorf("User aborted")
		}
	}

	// Uplink selection table.
	header := []string{"LOCATION", "IFACE", "TYPE"}
	fmt.Println("Select an available interface per system to provide external connectivity for distributed network(s):")
	data := [][]string{}
	for peer, state := range c.state {
		if !askSystems[peer] {
			continue
		}

		for _, net := range state.AvailableUplinkInterfaces {
			data = append(data, []string{peer, net.Name, net.Type})
		}
	}

	table := NewSelectableTable(header, data)
	var selectedIfaces map[string]string
	err = c.askRetry("Retry selecting uplink interfaces?", func() error {
		err := table.Render(table.rows)
		if err != nil {
			return err
		}

		answers, err := table.GetSelections()
		if err != nil {
			return err
		}

		selected := map[string]string{}
		for _, answer := range answers {
			target := table.SelectionValue(answer, "LOCATION")
			iface := table.SelectionValue(answer, "IFACE")

			if selected[target] != "" {
				return fmt.Errorf("Failed to add OVN uplink network: Selected more than one interface for target %q", target)
			}

			selected[target] = iface
		}

		if len(selected) != len(askSystems) {
			return fmt.Errorf("Failed to add OVN uplink network: Some peers don't have a selected interface")
		}

		selectedIfaces = selected

		return nil
	})
	if err != nil {
		return err
	}

	for peer, iface := range selectedIfaces {
		fmt.Printf(" Using %q on %q for OVN uplink\n", iface, peer)
	}

	// If we didn't select anything, then abort network setup.
	if len(selectedIfaces) == 0 {
		return nil
	}

	// Add a space between the CLI and the response.
	fmt.Println("")

	// Prepare the configuration.
	var dnsAddresses string
	ipConfig := map[string]string{}
	if !useOVNJoinConfig {
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
				if ip == "IPv4" {
					rangeStart, err := c.asker.AskString(fmt.Sprintf("Specify the first %s address in the range to use on the uplink network: ", ip), "", validate.Required(validate.IsNetworkAddressV4))
					if err != nil {
						return err
					}

					rangeEnd, err := c.asker.AskString(fmt.Sprintf("Specify the last %s address in the range to use on the uplink network: ", ip), "", validate.Required(validate.IsNetworkAddressV4))
					if err != nil {
						return err
					}

					ipConfig[gateway] = fmt.Sprintf("%s-%s", rangeStart, rangeEnd)
				} else {
					ipConfig[gateway] = ""
				}
			}
		}

		if len(ipConfig) > 0 {
			gateways := []string{}
			for gateway := range ipConfig {
				gatewayAddr, _, err := net.ParseCIDR(gateway)
				if err != nil {
					return err
				}

				gateways = append(gateways, gatewayAddr.String())
			}

			gatewayAddrs := strings.Join(gateways, ",")
			dnsAddresses, err = c.asker.AskString(fmt.Sprintf("Specify the DNS addresses (comma-separated IPv4 / IPv6 addresses) for the distributed network (default: %s): ", gatewayAddrs), gatewayAddrs, validate.Optional(validate.IsListOf(validate.IsNetworkAddress)))
			if err != nil {
				return err
			}
		}
	}

	lxd := sh.Services[types.LXD].(*service.LXDService)
	joinConfigs := map[string]api.ClusterMemberConfigKey{}
	targetConfigs := map[string]api.NetworksPost{}
	finalConfigs := []api.NetworksPost{}
	if useOVNJoinConfig {
		for target, parent := range selectedIfaces {
			joinConfigs[target] = lxd.DefaultOVNNetworkJoinConfig(parent)
		}
	} else {
		for target, parent := range selectedIfaces {
			targetConfigs[target] = lxd.DefaultPendingOVNNetwork(parent)
		}

		if len(targetConfigs) > 0 {
			var ipv4Gateway string
			var ipv4Ranges string
			var ipv6Gateway string
			for gateway, ipRange := range ipConfig {
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

			uplink, ovn := lxd.DefaultOVNNetwork(ipv4Gateway, ipv4Ranges, ipv6Gateway, dnsAddresses)
			finalConfigs = append(finalConfigs, uplink, ovn)
		}
	}

	canOVNUnderlay := true
	for peer, system := range c.systems {
		if len(c.state[system.ServerInfo.Name].AvailableOVNInterfaces) == 0 {
			fmt.Printf("Not enough interfaces available on %s to create an underlay network, skipping\n", peer)
			canOVNUnderlay = false
			break
		}
	}

	var ovnUnderlaySelectedIPs map[string]string
	ovnUnderlayData := [][]string{}
	for peer, system := range c.systems {
		// skip any systems that have already been clustered, but are available for other configuration.
		state, ok := c.state[c.name]
		if ok {
			if state.ExistingServices[types.MicroOVN][peer] != "" {
				continue
			}
		}

		for _, net := range c.state[system.ServerInfo.Name].AvailableOVNInterfaces {
			for _, addr := range net.Addresses {
				ovnUnderlayData = append(ovnUnderlayData, []string{peer, net.Network.Name, net.Network.Type, addr})
			}
		}
	}

	if len(ovnUnderlayData) != 0 && canOVNUnderlay {
		wantsDedicatedUnderlay, err := c.asker.AskBool("Configure dedicated underlay networking? (yes/no) [default=no]: ", "no")
		if err != nil {
			return err
		}

		if wantsDedicatedUnderlay {
			header = []string{"LOCATION", "IFACE", "TYPE", "IP ADDRESS (CIDR)"}
			fmt.Println("Select exactly one network interface from each cluster member:")

			table = NewSelectableTable(header, ovnUnderlayData)
			ovnUnderlaySelectedIPs = map[string]string{}
			err = c.askRetry("Retry selecting underlay network interfaces?", func() error {
				err = table.Render(table.rows)
				if err != nil {
					return err
				}

				answers, err := table.GetSelections()
				if err != nil {
					return err
				}

				ovnUnderlaySelectedIPs = map[string]string{}
				for _, answer := range answers {
					target := table.SelectionValue(answer, "LOCATION")
					ipAddr := table.SelectionValue(answer, "IP ADDRESS (CIDR)")

					if ovnUnderlaySelectedIPs[target] != "" {
						return fmt.Errorf("Failed to configure OVN underlay traffic: Selected more than one interface for target %q", target)
					}

					ovnUnderlaySelectedIPs[target] = ipAddr
				}

				return nil
			})
			if err != nil {
				return err
			}
		}
	}

	for peer, system := range c.systems {
		if !askSystems[peer] {
			continue
		}

		if system.JoinConfig == nil {
			system.JoinConfig = []api.ClusterMemberConfigKey{}
		}

		if system.TargetNetworks == nil {
			system.TargetNetworks = []api.NetworksPost{}
		}

		if system.Networks == nil {
			system.Networks = []api.NetworksPost{}
		}

		if joinConfigs[peer] != (api.ClusterMemberConfigKey{}) {
			system.JoinConfig = append(system.JoinConfig, joinConfigs[peer])
		}

		if targetConfigs[peer].Name != "" {
			system.TargetNetworks = append(system.TargetNetworks, targetConfigs[peer])
		}

		if peer == sh.Name {
			system.Networks = append(system.Networks, finalConfigs...)
		}

		if ovnUnderlaySelectedIPs != nil {
			ovnUnderlayIpAddr, ok := ovnUnderlaySelectedIPs[peer]
			if ok {
				ip, _, err := net.ParseCIDR(ovnUnderlayIpAddr)
				if err != nil {
					return err
				}

				fmt.Printf("Using %q for OVN underlay traffic on %q\n", ip.String(), peer)
				system.OVNGeneveAddr = ip.String()
			}
		}

		c.systems[peer] = system
	}

	return nil
}

func (c *initConfig) askNetwork(sh *service.Handler) error {
	err := c.askOVNNetwork(sh)
	if err != nil {
		return err
	}

	for _, system := range c.systems {
		if len(system.TargetNetworks) > 0 || len(system.Networks) > 0 {
			return nil
		}

		for _, cfg := range system.JoinConfig {
			if cfg.Name == service.DefaultOVNNetwork || cfg.Name == service.DefaultUplinkNetwork {
				return nil
			}
		}
	}

	useFANJoinConfig := false
	for _, state := range c.state {
		hasFAN, supportsFAN, err := state.SupportsFANNetwork(c.name == state.ClusterName)
		if err != nil {
			return err
		}

		if !supportsFAN {
			proceedWithNoOverlayNetworking, err := c.asker.AskBool("FAN networking is not usable. Do you want to proceed with setting up an inoperable cluster? (yes/no) [default=no]: ", "no")
			if err != nil {
				return err
			}

			if !proceedWithNoOverlayNetworking {
				return fmt.Errorf("Cluster bootstrapping aborted due to lack of usable networking")
			}
		}

		if hasFAN {
			useFANJoinConfig = true
		}
	}

	if !useFANJoinConfig {
		lxd := sh.Services[types.LXD].(*service.LXDService)
		fan, err := lxd.DefaultFanNetwork()
		if err != nil {
			return err
		}

		pendingFan := lxd.DefaultPendingFanNetwork()
		for peer, system := range c.systems {
			if system.TargetNetworks == nil {
				system.TargetNetworks = []api.NetworksPost{}
			}

			system.TargetNetworks = append(system.TargetNetworks, pendingFan)

			if peer == sh.Name {
				if system.Networks == nil {
					system.Networks = []api.NetworksPost{}
				}

				system.Networks = append(system.Networks, fan)
			}

			c.systems[peer] = system
		}
	}

	return nil
}

func (c *initConfig) askCephNetwork(sh *service.Handler) error {
	availableCephNetworkInterfaces := map[string]map[string]service.DedicatedInterface{}
	for name, state := range c.state {
		if len(state.AvailableCephInterfaces) == 0 {
			fmt.Printf("No network interfaces found with IPs on %q to set a dedicated Ceph network, skipping Ceph network setup\n", name)

			return nil
		}

		ifaces := make(map[string]service.DedicatedInterface, len(state.AvailableCephInterfaces))
		for name, iface := range state.AvailableCephInterfaces {
			ifaces[name] = iface
		}

		availableCephNetworkInterfaces[name] = ifaces
	}

	var defaultCephNetwork *net.IPNet
	for _, state := range c.state {
		if state.CephConfig != nil {
			value, ok := state.CephConfig["cluster_network"]
			if !ok || value == "" {
				continue
			}

			// Sometimes, the default cluster_network value in the Ceph configuration
			// is not a network range but a regular IP address. We need to extract the network range.
			_, valueNet, err := net.ParseCIDR(value)
			if err != nil {
				return fmt.Errorf("Failed to parse the Ceph cluster network configuration from the existing Ceph cluster: %v", err)
			}

			defaultCephNetwork = valueNet
			break
		}
	}

	lxd := sh.Services[types.LXD].(*service.LXDService)
	if defaultCephNetwork != nil {
		if defaultCephNetwork.String() != "" && defaultCephNetwork.String() != c.lookupSubnet.String() {
			err := validateCephInterfacesForSubnet(lxd, c.systems, availableCephNetworkInterfaces, defaultCephNetwork.String())
			if err != nil {
				return err
			}
		}

		return nil
	}

	// MicroCeph is uninitialized, so ask the user for the network configuration.
	microCloudInternalNetworkAddr := c.lookupSubnet.IP.Mask(c.lookupSubnet.Mask)
	ones, _ := c.lookupSubnet.Mask.Size()
	microCloudInternalNetworkAddrCIDR := fmt.Sprintf("%s/%d", microCloudInternalNetworkAddr.String(), ones)
	internalCephSubnet, err := c.asker.AskString(fmt.Sprintf("What subnet (either IPv4 or IPv6 CIDR notation) would you like your Ceph internal traffic on? [default: %s] ", microCloudInternalNetworkAddrCIDR), microCloudInternalNetworkAddrCIDR, validate.IsNetwork)
	if err != nil {
		return err
	}

	if internalCephSubnet != microCloudInternalNetworkAddrCIDR {
		err = validateCephInterfacesForSubnet(lxd, c.systems, availableCephNetworkInterfaces, internalCephSubnet)
		if err != nil {
			return err
		}

		bootstrapSystem := c.systems[sh.Name]
		bootstrapSystem.MicroCephInternalNetworkSubnet = internalCephSubnet
		c.systems[sh.Name] = bootstrapSystem
	}

	return nil
}

// askClustered checks whether any of the selected systems have already initialized any expected services.
// If a service is already initialized on some systems, we will offer to add the remaining systems, or skip that service.
// In auto setup, we will expect no initialized services so that we can be opinionated about how we configure the cluster without user input.
// This works by deleting the record for the service from the `service.Handler`, thus ignoring it for the remainder of the setup.
func (c *initConfig) askClustered(s *service.Handler, expectedServices []types.ServiceType) error {
	if !c.setupMany {
		return nil
	}

	for _, serviceType := range expectedServices {
		for name, info := range c.state {
			_, newSystem := c.systems[name]
			if !newSystem {
				continue
			}

			if info.ServiceClustered(serviceType) {
				question := fmt.Sprintf("%q is already part of a %s cluster. Do you want to add this cluster to Microcloud? (add/skip) [default=add]", info.ClusterName, serviceType)
				validator := func(s string) error {
					if !shared.ValueInSlice(s, []string{"add", "skip"}) {
						return fmt.Errorf("Invalid input, expected one of (add,skip) but got %q", s)
					}

					return nil
				}

				addOrSkip, err := c.asker.AskString(question, "add", validator)
				if err != nil {
					return err
				}

				if addOrSkip != "add" {
					delete(s.Services, serviceType)
				}

				break
			}
		}
	}

	return nil
}

func (c *initConfig) shortFingerprint(fingerprint string) (string, error) {
	if len(fingerprint) < 12 {
		return "", fmt.Errorf("Fingerprint is not long enough")
	}

	return fingerprint[0:12], nil
}

func (c *initConfig) askPassphrase(s *service.Handler) (string, error) {
	validator := func(password string) error {
		if password == "" {
			return fmt.Errorf("Passphrase cannot be empty")
		}

		passwordSplit := strings.Split(password, " ")
		if len(passwordSplit) != 4 {
			return fmt.Errorf("Passphrase has to contain exactly four elements")
		}

		return nil
	}

	cloud := s.Services[types.MicroCloud].(*service.CloudService)
	cert, err := cloud.ServerCert()
	if err != nil {
		return "", err
	}

	fingerprint, err := c.shortFingerprint(cert.Fingerprint())
	if err != nil {
		return "", fmt.Errorf("Failed to shorten fingerprint: %w", err)
	}

	fmt.Printf("Verify the fingerprint %q is displayed on the other system.\n", fingerprint)

	msg := "Specify the passphrase for joining the system: "
	password, err := c.asker.AskString(msg, "", validator)
	if err != nil {
		return "", err
	}

	return password, nil
}

func (c *initConfig) askJoinIntents(gw *cloudClient.WebsocketGateway, expectedSystems []string) ([]types.SessionJoinPost, error) {
	header := []string{"NAME", "ADDRESS", "FINGERPRINT"}
	var table *SelectableTable

	rendered := make(chan error)
	joinIntents := make(map[string]types.SessionJoinPost)

	renderCtx, renderCancel := context.WithCancel(gw.Context())
	defer renderCancel()

	renderIntentsInteractive := func() {
		for {
			select {
			case bytes := <-gw.Receive():
				session := types.Session{}
				err := json.Unmarshal(bytes, &session)
				if err != nil {
					logger.Error("Failed to read join intent", logger.Ctx{"err": err})
					break
				}

				joinIntents[session.Intent.Name] = session.Intent

				remoteCert, err := shared.ParseCert([]byte(session.Intent.Certificate))
				if err != nil {
					logger.Error("Failed to parse certificate", logger.Ctx{"err": err})
				}

				fingerprint, err := c.shortFingerprint(shared.CertFingerprint(remoteCert))
				if err != nil {
					logger.Error("Failed to shorten fingerprint", logger.Ctx{"err": err})
				}

				if table == nil {
					table = NewSelectableTable(header, [][]string{{session.Intent.Name, session.Intent.Address, fingerprint}})
					err := table.Render(table.rows)
					if err != nil {
						logger.Error("Failed to render table", logger.Ctx{"err": err})
					}

					rendered <- nil
				} else {
					table.Update([]string{session.Intent.Name, session.Intent.Address, fingerprint})
				}

			case <-renderCtx.Done():
				return
			}
		}
	}

	renderIntents := func() {
		for {
			select {
			case bytes := <-gw.Receive():
				session := types.Session{}
				err := json.Unmarshal(bytes, &session)
				if err != nil {
					logger.Error("Failed to read join intent", logger.Ctx{"err": err})
					break
				}

				// Skip systems which aren't listed in the preseed.
				if !shared.ValueInSlice(session.Intent.Name, expectedSystems) {
					continue
				}

				joinIntents[session.Intent.Name] = session.Intent
				if len(joinIntents) == len(expectedSystems) {
					renderCancel()
				}

			case <-renderCtx.Done():
				return
			}
		}
	}

	var systems []types.SessionJoinPost
	if !c.autoSetup {
		go renderIntentsInteractive()

		// Wait until the table got rendered.
		// This is important otherwise the table might not be selectable
		// as it's being built in a go routine.
		select {
		case <-rendered:
		case <-gw.Context().Done():
			return nil, fmt.Errorf("Failed to render join intents: %w", context.Cause(gw.Context()))
		}

		var answers []string
		retry := false
		err := c.askRetry("Retry selecting systems?", func() error {
			defer func() {
				retry = true
			}()

			fmt.Println("Select which systems you want to join:")

			if retry {
				err := table.Render(table.rows)
				if err != nil {
					return fmt.Errorf("Failed to render table: %w", err)
				}
			}

			var err error
			answers, err = table.GetSelections()
			if err != nil {
				return fmt.Errorf("Failed to get join intent selections: %w", err)
			}

			if len(answers) == 0 {
				return fmt.Errorf("No system selected")
			}

			return nil
		})
		if err != nil {
			return nil, err
		}

		for _, answer := range answers {
			name := table.SelectionValue(answer, "NAME")
			for intentName, intent := range joinIntents {
				if intentName == name {
					systems = append(systems, intent)
				}
			}
		}
	} else {
		go renderIntents()

		select {
		case <-time.After(c.lookupTimeout):
		case <-renderCtx.Done():
		}

		for _, name := range expectedSystems {
			_, ok := joinIntents[name]
			if !ok {
				return nil, fmt.Errorf("System %q hasn't reached out", name)
			}
		}

		for _, intent := range joinIntents {
			systems = append(systems, intent)
		}
	}

	return systems, nil
}

func (c *initConfig) askJoinConfirmation(gw *cloudClient.WebsocketGateway, services []types.ServiceType) error {
	session := types.Session{}
	err := gw.ReceiveWithContext(gw.Context(), &session)
	if err != nil {
		return fmt.Errorf("Failed to read join confirmation: %w", err)
	}

	if !c.autoSetup {
		fmt.Printf("\n Received confirmation from system %q\n\n", session.Intent.Name)
		fmt.Println("Do not exit out to keep the session alive.")
		fmt.Printf("Complete the remaining configuration on %q ...\n", session.Intent.Name)
	}

	err = gw.ReceiveWithContext(gw.Context(), &session)
	if err != nil {
		return fmt.Errorf("Failed waiting during join: %w", err)
	}

	if session.Error != "" {
		return fmt.Errorf("Failed to join system: %s", session.Error)
	}

	fmt.Println("Successfully joined the MicroCloud cluster and closing the session.")

	// Filter out MicroCloud.
	services = slices.DeleteFunc(services, func(t types.ServiceType) bool {
		return t == types.MicroCloud
	})

	if len(services) > 0 {
		var servicesStr []string
		for _, service := range services {
			servicesStr = append(servicesStr, string(service))
		}

		fmt.Printf("Commencing cluster join of the remaining services (%s)\n", strings.Join(servicesStr, ", "))
	}

	return nil
}
