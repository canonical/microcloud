package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
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

	cloudAPI "github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/api/types"
	cloudClient "github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcloud/microcloud/cmd/tui"
	"github.com/canonical/microcloud/microcloud/multicast"
	"github.com/canonical/microcloud/microcloud/service"
)

func checkInitialized(stateDir string, expectInitialized bool, preseed bool) error {
	cfg := initConfig{autoSetup: true}

	installedServices := []types.ServiceType{types.MicroCloud, types.LXD}

	// MicroCloud will automatically set up previously-uninitialized services,
	// and incorporate already-initialized services in interactive setup,
	// so we can ignore optional services unless using preseed.
	if preseed {
		optionalServices := map[types.ServiceType]string{
			types.MicroCeph: cloudAPI.MicroCephDir,
			types.MicroOVN:  cloudAPI.MicroOVNDir,
		}

		var err error
		installedServices, err = cfg.askMissingServices(installedServices, optionalServices)
		if err != nil {
			return err
		}
	}

	s, err := service.NewHandler("", "", stateDir, installedServices...)
	if err != nil {
		return err
	}

	return s.RunConcurrent("", "", func(s service.Service) error {
		initialized, err := s.IsInitialized(context.Background())
		if err != nil {
			return err
		}

		if expectInitialized && !initialized {
			errMsg := fmt.Sprintf("%s is not initialized", s.Type())
			if s.Type() == types.MicroCloud && !preseed {
				errMsg = errMsg + ". Run 'microcloud init' first"
			}

			return fmt.Errorf("%s", errMsg)
		} else if !expectInitialized && initialized {
			errMsg := fmt.Sprintf("%s is already initialized", s.Type())
			if s.Type() == types.MicroCloud && !preseed {
				errMsg = errMsg + ". Use 'microcloud add' instead"
			}

			return fmt.Errorf("%s", errMsg)
		}

		return nil
	})
}

// askUpdateProfile asks whether to update the existing profile configuration if it has changed.
func (c *initConfig) askUpdateProfile(profile api.ProfilesPost, profiles []string, lxdClient lxd.InstanceServer) (*api.ProfilePut, error) {
	if !slices.Contains(profiles, profile.Name) {
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
		replace, err := c.asker.AskBool("Replace existing default profile configuration?", false)
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
			// If the error is a context error, then the lifetime is over so we shouldn't ask to retry.
			if errors.Is(err, tui.ContextError) {
				return err
			}

			tui.PrintError(err.Error())
			retry, err = c.asker.AskBool(question, true)
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
			warning := serviceStr + " not found"
			question := "Continue anyway?"
			confirm, err := c.asker.AskBoolWarn(warning, question, true)
			if err != nil {
				return nil, err
			}

			if !confirm {
				return nil, errors.New("User aborted")
			}

			return services, nil
		}

		logger.Infof("Skipping %s (could not detect service state directory)", serviceStr)
	}

	return services, nil
}

func (c *initConfig) askAddress(filterAddress string) error {
	info, err := multicast.GetNetworkInfo()
	if err != nil {
		return fmt.Errorf("Failed to find network interfaces: %w", err)
	}

	listenAddr := c.address
	if listenAddr == "" {
		if len(info) == 0 {
			return errors.New("Found no valid network interfaces")
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

			err := c.askRetry("Retry selecting an address?", func() error {
				table := tui.NewSelectableTable([]string{"ADDRESS", "IFACE"}, data)
				answers, err := table.Render(context.Background(), c.asker, "Select an address for MicroCloud's internal traffic:")
				if err != nil {
					return err
				}

				if len(answers) != 1 {
					return errors.New("You must select exactly one address")
				}

				listenAddr = answers[0]["ADDRESS"]
				fmt.Printf("\n%s\n\n", tui.SummarizeResult("Using address %s for MicroCloud", listenAddr))

				return nil
			})
			if err != nil {
				return err
			}
		} else {
			fmt.Println(tui.SummarizeResult("Using address %s for MicroCloud", listenAddr))
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

func (c *initConfig) askLocalPool(sh *service.Handler) error {
	useJoinConfig := false
	askSystems := map[string]bool{}
	for _, info := range c.state {
		hasPool, supportsPool := info.SupportsLocalPool()
		if !supportsPool {
			tui.PrintWarning("Skipping local storage pool setup. Some systems don't support it")

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
		// Skip this system if it already has the local storage pool configured
		// and isn't marked for disk selection.
		// This ensures that when adding new systems to the cluster the existing ones
		// don't have to show any disk because they are probably already used for either local or remote storage.
		if !askSystems[name] {
			continue
		}

		if len(state.AvailableDisks) == 0 {
			continue
		}

		availableDisks[name] = state.AvailableDisks
	}

	// Local storage is already set up on every system, or if not every system has a disk.
	if len(askSystems) == 0 || len(availableDisks) != len(askSystems) {
		tui.PrintWarning("No disks available for local storage. Skipping configuration")

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
			return service.FormatDiskPath(sortedDisks[i]) < service.FormatDiskPath(sortedDisks[j])
		})

		for _, disk := range sortedDisks {
			devicePath := service.FormatDiskPath(disk)
			data = append(data, []string{peer, disk.Model, units.GetByteSizeStringIEC(int64(disk.Size), 2), disk.Type, devicePath})
		}
	}

	wantsDisks, err := c.asker.AskBool("Would you like to set up local storage?", true)
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
		table := tui.NewSelectableTable(header, data)
		answers, err := table.Render(context.Background(), c.asker, "Select exactly one disk from each cluster member:")
		if err != nil {
			return err
		}

		if len(answers) == 0 {
			return errors.New("No disks selected")
		}

		for _, entry := range answers {
			target := entry["LOCATION"]
			path := entry["PATH"]

			_, ok := selected[target]
			if ok {
				return fmt.Errorf("Failed to add local storage pool: Selected more than one disk for target peer %q", target)
			}

			selected[target] = path
		}

		if len(selected) != len(askSystems) {
			return errors.New("Failed to add local storage pool: Some peers don't have an available disk")
		}

		if wipeable {
			newRows := make([][]string, len(answers))
			for row := range answers {
				newRows[row] = make([]string, len(header))
				for j, h := range header {
					newRows[row][j] = answers[row][h]
				}
			}

			answers, err := table.Render(context.Background(), c.asker, "Select which disks to wipe:", newRows...)
			if err != nil {
				return fmt.Errorf("Failed to confirm which disks to wipe: %w", err)
			}

			for _, entry := range answers {
				target := entry["LOCATION"]
				path := entry["PATH"]
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

	if wipeable {
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

	if len(selectedDisks) > 0 {
		// Add a space between the CLI and the response.
		fmt.Println("")
	}

	for target, path := range selectedDisks {
		fmt.Println(tui.SummarizeResult("Using %s on %s for local storage pool", path, target))
	}

	if len(selectedDisks) > 0 {
		// Add a space between the CLI and the response.
		fmt.Println("")
	}

	newAvailableDisks := map[string]map[string]api.ResourcesStorageDisk{}
	for target, path := range selectedDisks {
		newAvailableDisks[target] = map[string]api.ResourcesStorageDisk{}
		for id, disk := range availableDisks[target] {
			if service.FormatDiskPath(disk) != path {
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

func (c *initConfig) validateCephInterfacesForSubnet(lxdService *service.LXDService, availableCephNetworkInterfaces map[string]map[string]service.DedicatedInterface, askedCephSubnet string) error {
	validatedCephInterfacesData, err := lxdService.ValidateCephInterfaces(askedCephSubnet, availableCephNetworkInterfaces)
	if err != nil {
		return err
	}

	// List the detected network interfaces
	if !c.autoSetup {
		for _, interfaces := range validatedCephInterfacesData {
			for _, iface := range interfaces {
				fmt.Printf("Interface %q (%q) detected on cluster member %q\n", iface[1], iface[2], iface[0])
			}
		}
	}

	// Even though not all the cluster members might have OSDs,
	// we check that all the machines have at least one interface to sustain the Ceph network
	for systemName := range c.systems {
		if len(validatedCephInterfacesData[systemName]) == 0 {
			return fmt.Errorf("Not enough network interfaces found with an IP within the given CIDR subnet on %q.\nYou need at least one interface per cluster member.", systemName)
		}
	}

	return nil
}

// getTargetCephNetworks fetches the Ceph network configuration from the existing Ceph cluster.
// If the system passed as an argument is nil, we will fetch the local Ceph network configuration.
func getTargetCephNetworks(sh *service.Handler, s *InitSystem) (publicCephNetwork *net.IPNet, internalCephNetwork *net.IPNet, err error) {
	microCephService := sh.Services[types.MicroCeph].(*service.CephService)
	if microCephService == nil {
		return nil, nil, errors.New("Failed to get MicroCeph service")
	}

	var cephAddr string
	var cephCert *x509.Certificate
	if s != nil && s.ServerInfo.Name != sh.Name {
		cephAddr = s.ServerInfo.Address
		cephCert = s.ServerInfo.Certificate
	}

	remoteCephConfigs, err := microCephService.ClusterConfig(context.Background(), cephAddr, cephCert)
	if err != nil {
		return nil, nil, err
	}

	for key, value := range remoteCephConfigs {
		if key == "cluster_network" && value != "" {
			// Sometimes, the default cluster_network value in the Ceph configuration
			// is not a network range but a regular IP address. We need to extract the network range.
			_, valueNet, err := net.ParseCIDR(value)
			if err != nil {
				return nil, nil, fmt.Errorf("Failed to parse the Ceph cluster network configuration from the existing Ceph cluster: %v", err)
			}

			internalCephNetwork = valueNet
		}

		if key == "public_network" && value != "" {
			_, valueNet, err := net.ParseCIDR(value)
			if err != nil {
				return nil, nil, fmt.Errorf("Failed to parse the Ceph public network configuration from the existing Ceph cluster: %v", err)
			}

			publicCephNetwork = valueNet
		}
	}

	return publicCephNetwork, internalCephNetwork, nil
}

func (c *initConfig) askRemotePool(sh *service.Handler) error {
	// If MicroCeph is not installed or an existing Ceph cluster should not be added, skip this block entirely.
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
			return errors.New("Unsupported configuration, remote-fs pool already exists")
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

	if len(askSystemsRemote) != 0 {
		// existingClusterDisks contains a slice of disks configured on each of the MicroCeph cluster members.
		// That allows checking whether or not the configured disks meet the recommendations.
		existingClusterDisks := map[string][]string{}

		// availableDiskCount is the total amount of unconfigured disks across all of the remote systems.
		availableDiskCount := 0

		// availableDisks contains a map of unconfigured disks on each of the remote systems.
		// Those disks aren't yet used for neither local nor remote storage.
		availableDisks := map[string]map[string]api.ResourcesStorageDisk{}

		// Set to true after checking one of the existing MicroCeph cluster members for existing disks.
		existingClusterDisksChecked := false

		for name, state := range c.state {
			// Collect list of already existing remote storage disks.
			// This allows understanding which disks on which cluster members are already configured for remote storage.
			// The information can then be yielded to the user and allows skipping the selection of additional disks
			// in case the user only wants to configure distributed storage without adding additional disks to MicroCeph.
			// That scenario is important when adding an existing MicroCeph cluster to MicroCloud.
			if state.ServiceClustered(types.MicroCeph) && !existingClusterDisksChecked {
				cephService := sh.Services[types.MicroCeph].(*service.CephService)
				system, ok := c.systems[name]
				if !ok {
					return fmt.Errorf("Failed to find system %q", name)
				}

				var cert *x509.Certificate
				var address string

				// When asking for remote pool configuration the MicroCloud cluster isn't yet formed.
				// But we have already established temporary trust with all of the remote systems.
				// Use the temporary trust store certificate only in case we are not trying to request
				// the disks from the local system itself.
				if name != sh.Name {
					cert = system.ServerInfo.Certificate
					address = state.ClusterAddress
				}

				disks, err := cephService.GetDisks(context.TODO(), address, cert)
				if err != nil {
					return fmt.Errorf("Failed to get disks of existing %s cluster on %q: %w", types.MicroCeph, name, err)
				}

				// Only initialize the slice if there are existing disks on this MicroCeph cluster member.
				// We consolidate the length of the map later to indicate whether or not there are already existing disks.
				if len(disks) > 0 {
					existingClusterDisks[name] = []string{}
				}

				// Fetching the disks from one of the existing MicroCeph cluster members is sufficient.
				// As the disks are known cluster wide, each member should respond with the same number.
				for _, disk := range disks {
					existingClusterDisks[disk.Location] = append(existingClusterDisks[disk.Location], disk.Path)
				}

				// Skip checking every other MicroCeph cluster member as we have already collected the existing disks.
				existingClusterDisksChecked = true
			}

			if askSystemsRemote[name] {
				availableDisks[name] = state.AvailableDisks

				if len(state.AvailableDisks) > 0 {
					availableDiskCount++
				}
			}
		}

		if availableDiskCount == 0 && len(existingClusterDisks) == 0 {
			tui.PrintWarning("No disks available for distributed storage. Skipping configuration")

			return nil
		}

		wantsDisks, err := c.asker.AskBool("Would you like to set up distributed storage?", true)
		if err != nil {
			return err
		}

		if len(existingClusterDisks) > 0 && wantsDisks {
			fmt.Println()

			for target, disks := range existingClusterDisks {
				if len(disks) > 0 {
					fmt.Println(tui.SummarizeResult("Using %d disk(s) already setup on %s for remote storage pool", len(disks), target))
				}
			}

			fmt.Println()
		}

		var insufficientDisks bool

		if availableDiskCount > 0 && wantsDisks {
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
						return service.FormatDiskPath(sortedDisks[i]) < service.FormatDiskPath(sortedDisks[j])
					})

					for _, disk := range sortedDisks {
						// Skip any disks that have been reserved for the local storage pool.
						devicePath := service.FormatDiskPath(disk)
						data = append(data, []string{peer, disk.Model, units.GetByteSizeStringIEC(int64(disk.Size), 2), disk.Type, devicePath})
					}
				}

				if len(data) == 0 {
					return errors.New("Invalid disk configuration. Found no available disks")
				}

				sort.Sort(cli.SortColumnsNaturally(data))
				var toWipe []map[string]string
				table := tui.NewSelectableTable(header, data)
				selected, err := table.Render(context.Background(), c.asker, "Select from the available unpartitioned disks:")
				if err != nil {
					return err
				}

				if len(selected) > 0 {
					newRows := make([][]string, len(selected))
					for row := range selected {
						newRows[row] = make([]string, len(header))
						for j, h := range header {
							newRows[row][j] = selected[row][h]
						}
					}

					toWipe, err = table.Render(context.Background(), c.asker, "Select which disks to wipe:", newRows...)
					if err != nil {
						return err
					}
				}

				targetDisks := map[string][]string{}
				for _, entry := range selected {
					target := entry["LOCATION"]
					path := entry["PATH"]
					if targetDisks[target] == nil {
						targetDisks[target] = []string{}
					}

					targetDisks[target] = append(targetDisks[target], path)
				}

				wipeDisks = map[string]map[string]bool{}
				for _, entry := range toWipe {
					target := entry["LOCATION"]
					path := entry["PATH"]
					if wipeDisks[target] == nil {
						wipeDisks[target] = map[string]bool{}
					}

					wipeDisks[target][path] = true
				}

				selectedDisks = targetDisks

				// Error in case no disks were selected or there isn't an existing Ceph cluster with disks configured.
				if len(targetDisks) == 0 && len(existingClusterDisks) == 0 {
					return errors.New("No disks were selected")
				}

				mergedDisks := map[string][]string{}

				// Merge both the already existing disks and the new selected ones.
				// This allows identifying if the selection follows the recommendations.
				for clusterMember, disks := range existingClusterDisks {
					_, ok := mergedDisks[clusterMember]
					if !ok {
						mergedDisks[clusterMember] = []string{}
					}

					mergedDisks[clusterMember] = append(mergedDisks[clusterMember], disks...)
				}

				for clusterMember, disks := range selectedDisks {
					_, ok := mergedDisks[clusterMember]
					if !ok {
						mergedDisks[clusterMember] = []string{}
					}

					mergedDisks[clusterMember] = append(mergedDisks[clusterMember], disks...)
				}

				insufficientDisks = !useJoinConfigRemote && len(mergedDisks) < RecommendedOSDHosts

				if insufficientDisks {
					return fmt.Errorf("Disk configuration does not meet recommendations for fault tolerance. At least %d systems must supply disks. Continuing with this configuration will inhibit MicroCloud's ability to retain data on system failure", RecommendedOSDHosts)
				}

				return nil
			})
			if err != nil {
				return err
			}
		}

		if len(selectedDisks) == 0 && len(existingClusterDisks) == 0 {
			// Skip distributed storage if there are neither disks selected nor is there an existing cluster with disks configured.
			return nil
		} else if len(selectedDisks) > 0 {
			// Print the newline only in case we haven't printed the notification about
			// already existing disks for the remote storage pool.
			// If we are reusing disks and also adding new ones, the two sections
			// should only be separated by a single new line.
			if len(existingClusterDisks) == 0 {
				fmt.Println()
			}

			for target, disks := range selectedDisks {
				if len(disks) > 0 {
					fmt.Println(tui.SummarizeResult("Using %d disk(s) on %s for remote storage pool", len(disks), target))
				}
			}

			fmt.Println()
		}
	}

	encryptDisks := false
	if len(selectedDisks) > 0 {
		var err error
		encryptDisks, err = c.asker.AskBool("Do you want to encrypt the selected disks?", false)
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
			setupCephFS, err = c.asker.AskBool("Would you like to set up CephFS remote storage?", true)
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

// NetworkInterfaceInfo stores a network's interface name, IP address, and its subnet.
type NetworkInterfaceInfo struct {
	// Name of the network interface.
	Interface net.Interface

	// IP address of the network.
	IP net.IP

	// Subnet of the network.
	Subnet *net.IPNet
}

func (c *initConfig) askOVNNetwork(sh *service.Handler) error {
	if sh.Services[types.MicroOVN] == nil {
		return nil
	}

	useOVNJoinConfig := false
	askSystems := map[string]bool{}
	allSystemsEligible := true
	for _, state := range c.state {
		hasOVN, supportsOVN := state.SupportsOVNNetwork()
		if !supportsOVN || len(state.AvailableUplinkInterfaces) == 0 {
			allSystemsEligible = false

			continue
		}

		if hasOVN {
			useOVNJoinConfig = true
		} else {
			askSystems[state.ClusterName] = true
		}
	}

	if len(askSystems) == 0 || !allSystemsEligible {
		warning := "Some systems are ineligible for distributed networking. At least one interface in state UP with no IPs assigned or a bridge is required"
		question := "Continue anyway?"
		wantsContinue, err := c.asker.AskBoolWarn(warning, question, true)
		if err != nil {
			return err
		}

		if wantsContinue {
			return nil
		}

		return errors.New("User aborted")
	}

	// Ask the user if they want OVN.
	wantsOVN, err := c.asker.AskBool("Configure distributed networking?", true)
	if err != nil {
		return err
	}

	if !wantsOVN {
		return nil
	}

	// Uplink selection table.
	header := []string{"LOCATION", "IFACE", "TYPE"}
	data := [][]string{}
	for peer, state := range c.state {
		if !askSystems[peer] {
			continue
		}

		for _, net := range state.AvailableUplinkInterfaces {
			data = append(data, []string{peer, net.Name, net.Type})
		}
	}

	var selectedIfaces map[string]string
	err = c.askRetry("Retry selecting uplink interfaces?", func() error {
		table := tui.NewSelectableTable(header, data)
		answers, err := table.Render(context.Background(), c.asker, "Select an available interface per system to provide external connectivity for distributed network(s):")
		if err != nil {
			return err
		}

		selected := map[string]string{}
		for _, answer := range answers {
			target := answer["LOCATION"]
			iface := answer["IFACE"]
			if selected[target] != "" {
				return fmt.Errorf("Failed to add OVN uplink network: Selected more than one interface for target %q", target)
			}

			selected[target] = iface
		}

		if len(selected) != len(askSystems) {
			return errors.New("Failed to add OVN uplink network: Some peers don't have a selected interface")
		}

		selectedIfaces = selected

		return nil
	})
	if err != nil {
		return err
	}

	if len(selectedIfaces) >= 0 {
		// Add a space between the CLI and the response.
		fmt.Println("")
	}

	for peer, iface := range selectedIfaces {
		fmt.Println(tui.SummarizeResult("Using %s on %s for OVN uplink", iface, peer))
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
					return errors.New("Not a valid IPv4")
				}

				if addr.To4() != nil && ip == "IPv6" {
					return errors.New("Not a valid IPv6")
				}

				return nil
			}

			msg := fmt.Sprintf("Specify the %s gateway (CIDR) on the uplink network", ip)
			gateway, err := c.asker.AskString(msg, "", validator)
			if err != nil {
				return err
			}

			if gateway == "" {
				continue
			}

			if ip == "IPv4" {
				rangeStart, err := c.asker.AskString(fmt.Sprintf("Specify the first %s address in the range to use on the uplink network", ip), "", validate.Required(validate.IsNetworkAddressV4))
				if err != nil {
					return err
				}

				rangeEnd, err := c.asker.AskString(fmt.Sprintf("Specify the last %s address in the range to use on the uplink network", ip), "", validate.Required(validate.IsNetworkAddressV4))
				if err != nil {
					return err
				}

				ipConfig[gateway] = fmt.Sprintf("%s-%s", rangeStart, rangeEnd)
			} else {
				ipConfig[gateway] = ""
			}
		}

		if len(ipConfig) == 0 {
			return errors.New("Either the IPv4 or IPv6 gateway has to be set on the uplink network")
		}

		gateways := []string{}
		for gateway := range ipConfig {
			gatewayAddr, _, err := net.ParseCIDR(gateway)
			if err != nil {
				return err
			}

			gateways = append(gateways, gatewayAddr.String())
		}

		gatewayAddrs := strings.Join(gateways, ",")
		dnsAddresses, err = c.asker.AskString("Specify the DNS addresses (comma-separated IPv4 / IPv6 addresses) for the distributed network", gatewayAddrs, validate.Optional(validate.IsListOf(validate.IsNetworkAddress)))
		if err != nil {
			return err
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
			tui.PrintWarning(fmt.Sprintf("Not enough interfaces available on %s to create an underlay network. Skipping configuration", peer))
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
		wantsDedicatedUnderlay, err := c.asker.AskBool("Configure dedicated OVN underlay networking?", false)
		if err != nil {
			return err
		}

		if wantsDedicatedUnderlay {
			header = []string{"LOCATION", "IFACE", "TYPE", "IP ADDRESS (CIDR)"}
			ovnUnderlaySelectedIPs = map[string]string{}
			err = c.askRetry("Retry selecting underlay network interfaces?", func() error {
				table := tui.NewSelectableTable(header, ovnUnderlayData)
				answers, err := table.Render(context.Background(), c.asker, "Select exactly one network interface from each cluster member:")
				if err != nil {
					return err
				}

				ovnUnderlaySelectedIPs = map[string]string{}
				for _, answer := range answers {
					target := answer["LOCATION"]
					ipAddr := answer["IP ADDRESS (CIDR)"]
					if ovnUnderlaySelectedIPs[target] != "" {
						return fmt.Errorf("Failed to configure OVN underlay traffic: Selected more than one interface for target %q", target)
					}

					ip, _, err := net.ParseCIDR(ipAddr)
					if err != nil {
						return err
					}

					ovnUnderlaySelectedIPs[target] = ip.String()
				}

				return nil
			})
			if err != nil {
				return err
			}
		}
	}

	if len(ovnUnderlaySelectedIPs) > 0 {
		// Add a space between the CLI and the response.
		fmt.Println("")

		for peer := range askSystems {
			underlayIP, ok := ovnUnderlaySelectedIPs[peer]
			if ok {
				fmt.Println(tui.SummarizeResult("Using %s on %s for OVN underlay traffic", underlayIP, peer))
			}
		}

		// Add a space between the result summary and the next question.
		fmt.Println()
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
				system.OVNGeneveAddr = ovnUnderlayIpAddr
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
			warning := "Fan networking is not usable"
			question := "Do you want to proceed with setting up an inoperable cluster?"
			proceedWithNoOverlayNetworking, err := c.asker.AskBoolWarn(warning, question, false)
			if err != nil {
				return err
			}

			if !proceedWithNoOverlayNetworking {
				return errors.New("Cluster bootstrapping aborted due to lack of usable networking")
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
			tui.PrintWarning(fmt.Sprintf("No network interfaces found with IPs on %q to set up a dedicated Ceph network. Skipping Ceph network setup", name))

			return nil
		}

		ifaces := make(map[string]service.DedicatedInterface, len(state.AvailableCephInterfaces))
		for name, iface := range state.AvailableCephInterfaces {
			ifaces[name] = iface
		}

		availableCephNetworkInterfaces[name] = ifaces
	}

	var internalCephNetwork *net.IPNet
	var publicCephNetwork *net.IPNet
	for _, state := range c.state {
		if state.CephConfig != nil {
			value, ok := state.CephConfig["cluster_network"]
			if ok && value != "" {
				// Sometimes, the default cluster_network value in the Ceph configuration
				// is not a network range but a regular IP address. We need to extract the network range.
				_, valueNet, err := net.ParseCIDR(value)
				if err != nil {
					return fmt.Errorf("Failed to parse the Ceph cluster network configuration from the existing Ceph cluster: %v", err)
				}

				internalCephNetwork = valueNet
			}

			value, ok = state.CephConfig["public_network"]
			if ok && value != "" {
				_, valueNet, err := net.ParseCIDR(value)
				if err != nil {
					return fmt.Errorf("Failed to parse the Ceph public network configuration from the existing Ceph cluster: %v", err)
				}

				publicCephNetwork = valueNet
			}
		}
	}

	lxd := sh.Services[types.LXD].(*service.LXDService)
	if internalCephNetwork != nil {
		if internalCephNetwork.String() != "" && internalCephNetwork.String() != c.lookupSubnet.String() {
			err := c.validateCephInterfacesForSubnet(lxd, availableCephNetworkInterfaces, internalCephNetwork.String())
			if err != nil {
				return err
			}
		}

		return nil
	}

	if publicCephNetwork != nil {
		if publicCephNetwork.String() != "" && publicCephNetwork.String() != c.lookupSubnet.String() {
			err := c.validateCephInterfacesForSubnet(lxd, availableCephNetworkInterfaces, publicCephNetwork.String())
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
	internalCephSubnet, err := c.asker.AskString("What subnet (IPv4/IPv6 CIDR) would you like your Ceph internal traffic on?", microCloudInternalNetworkAddrCIDR, validate.IsNetwork)
	if err != nil {
		return err
	}

	if internalCephSubnet != microCloudInternalNetworkAddrCIDR {
		err = c.validateCephInterfacesForSubnet(lxd, availableCephNetworkInterfaces, internalCephSubnet)
		if err != nil {
			return err
		}

		bootstrapSystem := c.systems[sh.Name]
		bootstrapSystem.MicroCephInternalNetworkSubnet = internalCephSubnet
		c.systems[sh.Name] = bootstrapSystem
	}

	publicCephSubnet, err := c.asker.AskString("What subnet (either IPv4 or IPv6 CIDR notation) would you like your Ceph public traffic on?", internalCephSubnet, validate.IsNetwork)
	if err != nil {
		return err
	}

	if publicCephSubnet != internalCephSubnet {
		err = c.validateCephInterfacesForSubnet(lxd, availableCephNetworkInterfaces, publicCephSubnet)
		if err != nil {
			return err
		}
	}

	if publicCephSubnet != microCloudInternalNetworkAddrCIDR {
		bootstrapSystem := c.systems[sh.Name]
		bootstrapSystem.MicroCephPublicNetworkSubnet = publicCephSubnet
		c.systems[sh.Name] = bootstrapSystem

		// This is to avoid the situation where the internal network for Ceph has been skipped, but the public network has been set.
		// Ceph will automatically set the internal network to the public Ceph network if the internal network is not set, which is not what we want.
		// Instead, we still want to keep the internal Ceph network to use the MicroCloud internal network as a default.
		if internalCephSubnet == microCloudInternalNetworkAddrCIDR {
			bootstrapSystem.MicroCephInternalNetworkSubnet = microCloudInternalNetworkAddrCIDR
			c.systems[sh.Name] = bootstrapSystem
		}
	}

	return nil
}

// askClustered checks whether any of the selected systems have already initialized any expected services.
// If a service is already initialized on some systems, we will offer to add the remaining systems, or skip that service.
// In auto setup, we will expect no initialized services so that we can be opinionated about how we configure the cluster without user input.
// This works by deleting the record for the service from the `service.Handler`, thus ignoring it for the remainder of the setup.
func (c *initConfig) askClustered(s *service.Handler, expectedServices map[types.ServiceType]string) error {
	if !c.setupMany {
		return nil
	}

	for serviceType := range expectedServices {
		for name, info := range c.state {
			_, newSystem := c.systems[name]
			if !newSystem {
				continue
			}

			if info.ServiceClustered(serviceType) {
				warning := fmt.Sprintf("%q is already part of a %s cluster", info.ClusterName, serviceType)
				question := "Do you want to add this cluster to MicroCloud?"
				addOrSkip, err := c.asker.AskBoolWarn(warning, question, true)
				if err != nil {
					return err
				}

				if !addOrSkip {
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
		return "", errors.New("Fingerprint is not long enough")
	}

	return fingerprint[0:12], nil
}

func (c *initConfig) askPassphrase(s *service.Handler) (string, error) {
	format := func(password string) (string, error) {
		passwordSplit := strings.Split(password, " ")

		passwordClean := slices.DeleteFunc(passwordSplit, func(element string) bool {
			return element == ""
		})

		if len(passwordClean) != 4 {
			return "", errors.New("Passphrase has to contain exactly four elements")
		}

		return strings.Join(passwordClean, " "), nil
	}

	validator := func(password string) error {
		if password == "" {
			return errors.New("Passphrase cannot be empty")
		}

		_, err := format(password)
		return err
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

	fmt.Println(tui.Printf(tui.Fmt{Arg: "Verify the fingerprint %s is displayed on the other system."}, tui.Fmt{Arg: fingerprint, Color: tui.Green, Bold: true}))
	msg := "Specify the passphrase for joining the system"
	password, err := c.asker.AskString(msg, "", validator)
	if err != nil {
		return "", err
	}

	return format(password)
}

func (c *initConfig) askJoinIntents(gw *cloudClient.WebsocketGateway, expectedSystems []string) ([]types.SessionJoinPost, error) {
	header := []string{"NAME", "ADDRESS", "FINGERPRINT"}
	rows := [][]string{}
	table := tui.NewSelectableTable(header, rows)

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

				table.SendUpdate(tui.InsertMsg{session.Intent.Name, session.Intent.Address, fingerprint})

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
				if !slices.Contains(expectedSystems, session.Intent.Name) {
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
		var answers []map[string]string
		err := c.askRetry("Retry selecting systems?", func() error {
			var err error
			answers, err = table.Render(gw.Context(), c.asker, "Systems will appear in the table as they are detected. Select those that should join the cluster:")
			if err != nil {
				return fmt.Errorf("Failed to render table: %w", err)
			}

			if len(answers) == 0 {
				return errors.New("No system selected")
			}

			return nil
		})
		if err != nil {
			return nil, err
		}

		for _, answer := range answers {
			name := answer["NAME"]
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

func (c *initConfig) askJoinConfirmation(gw *cloudClient.WebsocketGateway, services map[types.ServiceType]string) error {
	session := types.Session{}
	err := gw.ReceiveWithContext(gw.Context(), &session)
	if err != nil {
		return fmt.Errorf("Failed to read join confirmation: %w", err)
	}

	if !c.autoSetup {
		fmt.Println("")
		fmt.Println(tui.SummarizeResult("Received confirmation from system %s", session.Intent.Name))
		fmt.Println("")
		fmt.Println(tui.Note(tui.Yellow, tui.WarningSymbol()+tui.SetColor(tui.Bright, " Do not exit out to keep the session alive", true)) + "\n")
		fmt.Println(tui.Printf(tui.Fmt{Arg: "Complete the remaining configuration on %s ..."}, tui.Fmt{Arg: session.Intent.Name, Bold: true}))
	}

	err = gw.ReceiveWithContext(gw.Context(), &session)
	if err != nil {
		return fmt.Errorf("Failed waiting during join: %w", err)
	}

	if session.Error != "" {
		return fmt.Errorf("Failed to join system: %s", session.Error)
	}

	fmt.Println(tui.SuccessColor("Successfully joined the MicroCloud cluster and closing the session.", true))

	servicesStr := make([]string, 0, len(services))
	for serviceType := range services {
		if serviceType == types.MicroCloud {
			continue
		}

		servicesStr = append(servicesStr, string(serviceType))
	}

	if len(servicesStr) > 0 {
		slices.Sort(servicesStr)

		fmt.Println(tui.Printf(tui.Fmt{Arg: "Commencing cluster join of the remaining services (%s)"}, tui.Fmt{Arg: strings.Join(servicesStr, ","), Bold: true}))
	}

	return nil
}
