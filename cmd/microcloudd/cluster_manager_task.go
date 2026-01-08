package main

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/microcluster/v3/state"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcloud/microcloud/database"
	"github.com/canonical/microcloud/microcloud/service"
)

// SendClusterManagerStatusMessageTask starts a go routine, that sends periodic status messages to cluster manager.
func SendClusterManagerStatusMessageTask(ctx context.Context, sh *service.Handler, s state.State) {
	go func(ctx context.Context, sh *service.Handler, s state.State) {
		ticker := time.NewTicker(database.UpdateIntervalDefaultSeconds * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				newUpdateTime := sendClusterManagerStatusMessage(ctx, sh, s)
				if newUpdateTime > 0 {
					ticker.Reset(newUpdateTime)
				}

			case <-ctx.Done():
				return // exit the loop and close the go routine
			}
		}
	}(ctx, sh, s)
}

func sendClusterManagerStatusMessage(ctx context.Context, sh *service.Handler, s state.State) time.Duration {
	logger.Debug("Starting sendClusterManagerStatusMessage")
	var nextUpdate time.Duration = 0

	cloud := sh.Services[types.MicroCloud].(*service.CloudService)
	isInitialized, err := cloud.IsInitialized(ctx)
	if err != nil {
		logger.Error("Failed to check if MicroCloud is initialized", logger.Ctx{"err": err})
		return nextUpdate
	}

	if !isInitialized {
		logger.Debug("MicroCloud not initialized, skipping status message")
		return nextUpdate
	}

	clusterManager, clusterManagerConfig, err := database.LoadClusterManager(s, ctx, database.ClusterManagerDefaultName)
	if err != nil {
		if err.Error() == "Cluster manager not found" {
			logger.Debug("Cluster manager not configured, skipping status message")
			return nextUpdate
		}

		logger.Error("Failed to load cluster manager config", logger.Ctx{"err": err})
		return nextUpdate
	}

	for _, config := range clusterManagerConfig {
		if config.Key == database.UpdateIntervalSecondsKey {
			interval, err := time.ParseDuration(config.Value + "s")
			if err != nil {
				logger.Error("Failed to parse update interval", logger.Ctx{"err": err})
				return nextUpdate
			}

			nextUpdate = interval
			break
		}
	}

	leaderClient, err := s.Database().Leader(ctx)
	if err != nil {
		logger.Error("Failed to get database leader client", logger.Ctx{"err": err})
		return nextUpdate
	}

	leaderInfo, err := leaderClient.Leader(ctx)
	if err != nil {
		logger.Error("Failed to get database leader info", logger.Ctx{"err": err})
		return nextUpdate
	}

	if leaderInfo.Address != s.Address().URL.Host {
		logger.Debug("Not the leader, skipping status message")
		return nextUpdate
	}

	payload := types.ClusterManagerPostStatus{}

	lxdService := sh.Services[types.LXD].(*service.LXDService)
	lxdClient, err := lxdService.Client(context.Background())
	if err != nil {
		logger.Error("Failed to get LXD client", logger.Ctx{"err": err})
		return nextUpdate
	}

	err = enrichInstanceMetrics(lxdClient, &payload)
	if err != nil {
		logger.Error("Failed to enrich instance metrics", logger.Ctx{"err": err})
		return nextUpdate
	}

	err = enrichServerMetrics(lxdClient, &payload)
	if err != nil {
		logger.Error("Failed to enrich server metrics", logger.Ctx{"err": err})
		return nextUpdate
	}

	err = enrichClusterMemberMetrics(lxdClient, &payload)
	if err != nil {
		logger.Error("Failed to enrich cluster member metrics", logger.Ctx{"err": err})
		return nextUpdate
	}

	clusterCert, err := cloud.ClusterCert()
	if err != nil {
		logger.Error("Failed to get cluster certificate", logger.Ctx{"err": err})
		return nextUpdate
	}

	clusterManagerClient := client.NewClusterManagerClient(clusterManager)
	err = clusterManagerClient.PostStatus(clusterCert, payload)
	if err != nil {
		logger.Error("Failed to send status message to cluster manager", logger.Ctx{"err": err})
		err = database.SetClusterManagerStatusLastError(s, ctx, database.ClusterManagerDefaultName, time.Now(), err.Error())
		if err != nil {
			logger.Error("Failed to set cluster manager status last error", logger.Ctx{"err": err})
		}

		return nextUpdate
	}

	err = database.SetClusterManagerStatusLastSuccess(s, ctx, database.ClusterManagerDefaultName, time.Now())
	if err != nil {
		logger.Error("Failed to set cluster manager status last success", logger.Ctx{"err": err})
	}

	logger.Debug("Finished sendClusterManagerStatusMessage")
	return nextUpdate
}

func enrichInstanceMetrics(lxdClient lxd.InstanceServer, result *types.ClusterManagerPostStatus) error {
	instanceFrequencies := make(map[string]int64)

	instanceList, err := lxdClient.GetInstancesAllProjects(api.InstanceTypeAny)
	for i := range instanceList {
		inst := instanceList[i]
		instanceFrequencies[inst.Status]++
	}

	for status, count := range instanceFrequencies {
		result.InstanceStatuses = append(result.InstanceStatuses, types.StatusDistribution{
			Status: status,
			Count:  count,
		})
	}

	return err
}

func enrichServerMetrics(lxdClient lxd.InstanceServer, result *types.ClusterManagerPostStatus) error {
	metrics, err := lxdClient.GetMetrics()
	if err != nil {
		return fmt.Errorf("Failed to get LXD metrics: %w", err)
	}

	result.Metrics = metrics

	return nil
}

func enrichClusterMemberMetrics(lxdClient lxd.InstanceServer, result *types.ClusterManagerPostStatus) error {
	lxdMembers, err := lxdClient.GetClusterMembers()
	if err != nil {
		return fmt.Errorf("Failed to get LXD cluster members: %w", err)
	}

	if len(lxdMembers) > 0 {
		result.UIURL = lxdMembers[0].URL
	}

	localPools, err := getLocalPools(lxdClient)
	if err != nil {
		return fmt.Errorf("Failed to get local LXD storage pools: %w", err)
	}

	var cpuLoad1 float64
	var cpuLoad5 float64
	var cpuLoad15 float64
	statusFrequencies := make(map[string]int64)
	for i := range lxdMembers {
		member := lxdMembers[i]

		statusFrequencies[member.Status]++
		memberState, _, err := lxdClient.GetClusterMemberState(member.ServerName)
		if err != nil {
			// If we can't get the state of a member, skip it but continue with others.
			logger.Warn("Failed to get LXD cluster member state", logger.Ctx{"member": member.ServerName, "err": err})
			continue
		}

		result.MemoryTotalAmount += int64(memberState.SysInfo.TotalRAM)
		result.MemoryUsage += int64(memberState.SysInfo.TotalRAM - memberState.SysInfo.FreeRAM - memberState.SysInfo.BufferRAM)

		cpuLoad1 += memberState.SysInfo.LoadAverages[0]
		cpuLoad5 += memberState.SysInfo.LoadAverages[1]
		cpuLoad15 += memberState.SysInfo.LoadAverages[2]

		enrichStoragePoolMetrics(member, memberState, localPools, result)
	}

	for status, count := range statusFrequencies {
		result.MemberStatuses = append(result.MemberStatuses, types.StatusDistribution{
			Status: status,
			Count:  count,
		})
	}

	if result.CPUTotalCount > 0 {
		result.CPULoad1 = fmt.Sprintf("%.2f", cpuLoad1/float64(result.CPUTotalCount))
		result.CPULoad5 = fmt.Sprintf("%.2f", cpuLoad5/float64(result.CPUTotalCount))
		result.CPULoad15 = fmt.Sprintf("%.2f", cpuLoad15/float64(result.CPUTotalCount))
	} else {
		result.CPULoad1 = fmt.Sprintf("%.2f", cpuLoad1)
		result.CPULoad5 = fmt.Sprintf("%.2f", cpuLoad5)
		result.CPULoad15 = fmt.Sprintf("%.2f", cpuLoad15)
	}

	return nil
}

func enrichStoragePoolMetrics(member api.ClusterMember, memberState *api.ClusterMemberState, localPools []string, result *types.ClusterManagerPostStatus) {
	for name, poolState := range memberState.StoragePools {
		if poolState.Space.Total == 0 || poolState.Space.Used == 0 {
			// Error state or no available info on this pool.
			logger.Info("Missing usage information from LXD storage pool", logger.Ctx{"member": member.ServerName, "pool": name})
			continue
		}

		isLocalPool := slices.Contains(localPools, name)
		if isLocalPool {
			result.StoragePoolUsages = append(result.StoragePoolUsages, types.StoragePoolUsage{
				Name:   name,
				Member: member.ServerName,
				Total:  poolState.Space.Total,
				Usage:  poolState.Space.Used,
			})

			continue
		}

		if hasStoragePool(result.StoragePoolUsages, name) {
			// We have already recorded this remote pool from another member.
			continue
		}

		result.StoragePoolUsages = append(result.StoragePoolUsages, types.StoragePoolUsage{
			Name:  name,
			Total: poolState.Space.Total,
			Usage: poolState.Space.Used,
		})
	}
}

func hasStoragePool(usages []types.StoragePoolUsage, name string) bool {
	for _, u := range usages {
		if u.Name == name {
			return true
		}
	}
	return false
}

func getLocalPools(lxdClient lxd.InstanceServer) ([]string, error) {
	server, _, err := lxdClient.GetServer()
	if err != nil {
		return nil, fmt.Errorf("Failed to get LXD server info: %w", err)
	}

	var localDrivers []string
	for _, d := range server.Environment.StorageSupportedDrivers {
		if !d.Remote {
			localDrivers = append(localDrivers, d.Name)
		}
	}

	storagePools, err := lxdClient.GetStoragePools()
	if err != nil {
		return nil, fmt.Errorf("Failed to get LXD storage pools: %w", err)
	}

	var localPools []string
	for _, pool := range storagePools {
		poolDriver := pool.Driver
		if slices.Contains(localDrivers, poolDriver) {
			localPools = append(localPools, pool.Name)
		}
	}

	return localPools, nil
}
