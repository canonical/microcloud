package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"sync"
	"time"

	"github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/logger"
	microTypes "github.com/canonical/microcluster/v3/microcluster/types"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcloud/microcloud/database"
	"github.com/canonical/microcloud/microcloud/service"
)

// SendClusterManagerStatusMessageTask starts a go routine, that sends periodic status messages to cluster manager.
func SendClusterManagerStatusMessageTask(ctx context.Context, sh *service.Handler, s microTypes.State) {
	go func(ctx context.Context, sh *service.Handler, s microTypes.State) {
		// tunnel object to hold the websocket connection and its mutex for safe concurrent access
		tunnel := &types.ClusterManagerTunnel{
			WsConn: nil, // This will be set when the websocket connection is established
			Mu:     sync.RWMutex{},
		}

		ticker := time.NewTicker(database.UpdateIntervalDefaultSeconds * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				newUpdateTime := sendClusterManagerStatusMessage(ctx, sh, s, tunnel)
				if newUpdateTime > 0 {
					ticker.Reset(newUpdateTime)
				}

			case <-ctx.Done():
				return // exit the loop and close the go routine
			}
		}
	}(ctx, sh, s)
}

func sendClusterManagerStatusMessage(ctx context.Context, sh *service.Handler, s microTypes.State, tunnel *types.ClusterManagerTunnel) time.Duration {
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

	hasReverseTunnel := false
	for _, config := range clusterManagerConfig {
		if config.Key == database.UpdateIntervalSecondsKey {
			interval, err := time.ParseDuration(config.Value + "s")
			if err != nil {
				logger.Error("Failed to parse update interval", logger.Ctx{"err": err})
				return nextUpdate
			}

			nextUpdate = interval
		}

		if config.Key == database.ReverseTunnelKey {
			hasReverseTunnel = config.Value == "true"
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

	if leaderInfo.Address != s.Address().Host {
		logger.Debug("Not the leader, skipping status message")
		ensureTunnelClosed(tunnel)
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

	lxdMembers, err := lxdClient.GetClusterMembers()
	if err != nil {
		logger.Error("Failed to get LXD cluster members", logger.Ctx{"err": err})
		return nextUpdate
	}

	err = enrichServerMetrics(ctx, lxdService, lxdMembers, &payload)
	if err != nil {
		logger.Error("Failed to enrich server metrics", logger.Ctx{"err": err})
		return nextUpdate
	}

	err = enrichClusterMemberMetrics(lxdClient, lxdMembers, &payload)
	if err != nil {
		logger.Error("Failed to enrich cluster member metrics", logger.Ctx{"err": err})
		return nextUpdate
	}

	err = enrichCephStatuses(ctx, sh, &payload)
	if err != nil {
		logger.Error("Failed to enrich Ceph statuses", logger.Ctx{"err": err})
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

	ensureTunnel(sh, tunnel, hasReverseTunnel, clusterManagerClient, clusterCert)

	logger.Debug("Finished sendClusterManagerStatusMessage")
	return nextUpdate
}

func enrichCephStatuses(ctx context.Context, sh *service.Handler, result *types.ClusterManagerPostStatus) error {
	if sh.Services[types.MicroCeph] == nil {
		return nil
	}

	cephService := sh.Services[types.MicroCeph].(*service.CephService)
	m := cephService.Microcluster()

	cephMembers, err := m.GetClusterMembers(ctx)
	if err != nil {
		return err
	}

	statusFrequencies := make(map[string]int64)
	for _, member := range cephMembers {
		statusFrequencies[string(member.Status)]++
	}

	for status, count := range statusFrequencies {
		result.CephStatuses = append(result.CephStatuses, types.StatusDistribution{
			Status: status,
			Count:  count,
		})
	}

	return nil
}

func enrichInstanceMetrics(lxdClient lxd.InstanceServer, result *types.ClusterManagerPostStatus) error {
	instanceFrequencies := make(map[string]int64)

	instanceList, err := lxdClient.GetInstancesAllProjects(api.InstanceTypeAny)
	for _, instance := range instanceList {
		instanceFrequencies[instance.Status]++
	}

	for status, count := range instanceFrequencies {
		result.InstanceStatuses = append(result.InstanceStatuses, types.StatusDistribution{
			Status: status,
			Count:  count,
		})
	}

	return err
}

func enrichServerMetrics(ctx context.Context, lxdService *service.LXDService, lxdMembers []api.ClusterMember, result *types.ClusterManagerPostStatus) error {
	for _, member := range lxdMembers {
		u, err := url.Parse(member.URL)
		if err != nil {
			// If we can't parse the URL of a member, skip it but continue with others.
			logger.Error("Could not parse URL for cluster member", logger.Ctx{"member": member, "url": member.URL, "err": err})
			continue
		}

		memberAddress := u.Hostname()
		metrics, err := lxdService.Metrics(ctx, memberAddress)
		if err != nil {
			// If we can't get the metrics of a member, skip it but continue with others.
			logger.Error("Could not fetch metrics for cluster member", logger.Ctx{"member": member, "err": err})
			continue
		}

		result.ServerMetrics = append(result.ServerMetrics, types.ServerMetrics{
			Member:  member.ServerName,
			Metrics: metrics,
			Service: types.LXD,
		})
	}

	return nil
}

func enrichClusterMemberMetrics(lxdClient lxd.InstanceServer, lxdMembers []api.ClusterMember, result *types.ClusterManagerPostStatus) error {
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
	for _, member := range lxdMembers {
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

func ensureTunnel(sh *service.Handler, tunnel *types.ClusterManagerTunnel, needsTunnel bool, clusterManagerClient *client.ClusterManagerClient, clusterCert *shared.CertInfo) {
	if needsTunnel && tunnel.WsConn != nil {
		logger.Debug("Websocket already connected, skipping reconnection")
		return
	}

	if needsTunnel && tunnel.WsConn == nil {
		logger.Debug("Websocket not connected, establishing connection in new goroutine")
		go openTunnel(sh, tunnel, clusterManagerClient, clusterCert)
		return
	}

	if !needsTunnel && tunnel.WsConn != nil {
		logger.Debug("Websocket connected but reverse tunnel is disabled, closing connection")
		go closeTunnel(tunnel)
		return
	}

	logger.Debug("Reverse tunnel is disabled, not opening websocket connection")
}

func openTunnel(sh *service.Handler, tunnel *types.ClusterManagerTunnel, clusterManagerClient *client.ClusterManagerClient, clusterCert *shared.CertInfo) {
	conn, err := clusterManagerClient.ConnectTunnelWebsocket(clusterCert)
	if err != nil {
		logger.Error("Failed to connect to cluster manager websocket", logger.Ctx{"err": err})
		return
	}

	defer func() {
		err := conn.Close()
		if err != nil {
			logger.Error("Failed to close cluster manager websocket", logger.Ctx{"err": err})
		}
	}()

	tunnel.Mu.Lock()
	tunnel.WsConn = conn
	tunnel.Mu.Unlock()

	// Handle CTRL+C to gracefully close
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	done := make(chan struct{})

	logger.Debug("Connected to cluster manager websocket")

	defer close(done)
	for {
		var req types.ClusterManagerTunnelRequest
		err = conn.ReadJSON(&req)
		if err != nil {
			logger.Error("Cluster manager websocket read error:", logger.Ctx{"err": err})
			tunnel.Mu.Lock()
			tunnel.WsConn = nil
			tunnel.Mu.Unlock()
			return
		}

		logger.Debug("Cluster manager websocket request received:", logger.Ctx{"path": req.Path})
		resp := handleTunnelRequest(req, sh)

		// Send back the response
		err = conn.WriteJSON(resp)
		if err != nil {
			logger.Error("Cluster manager websocket write error:", logger.Ctx{"err": err})
			tunnel.Mu.Lock()
			tunnel.WsConn = nil
			tunnel.Mu.Unlock()
			return
		}
	}
}

func handleTunnelRequest(req types.ClusterManagerTunnelRequest, sh *service.Handler) types.ClusterManagerTunnelResponse {
	lxdService := sh.Services[types.LXD].(*service.LXDService)
	lxdClient, err := lxdService.Client(context.Background())
	if err != nil {
		logger.Error("Failed to get LXD client", logger.Ctx{"err": err})
		return types.ClusterManagerTunnelResponse{ID: req.ID, Status: http.StatusInternalServerError}
	}

	lxdResponse, _, err := lxdClient.RawQuery(req.Method, req.Path, bytes.NewReader(req.Body), "")
	if err != nil {
		logger.Error("Error from LXD client query", logger.Ctx{"err": err, "path": req.Path, "method": req.Method})
		return types.ClusterManagerTunnelResponse{ID: req.ID, Status: http.StatusInternalServerError}
	}

	responseBody, err := json.Marshal(lxdResponse)
	if err != nil {
		logger.Error("Failed to marshal LXD response", logger.Ctx{"err": err})
		return types.ClusterManagerTunnelResponse{ID: req.ID, Status: http.StatusInternalServerError}
	}

	return types.ClusterManagerTunnelResponse{
		ID:     req.ID,
		Status: lxdResponse.StatusCode,
		Body:   responseBody,
	}
}

func ensureTunnelClosed(tunnel *types.ClusterManagerTunnel) {
	if tunnel.WsConn == nil {
		return
	}

	logger.Debug("Closing cluster manager websocket.")
	closeTunnel(tunnel)
}

func closeTunnel(tunnel *types.ClusterManagerTunnel) {
	tunnel.Mu.Lock()
	defer tunnel.Mu.Unlock()

	err := tunnel.WsConn.Close()
	tunnel.WsConn = nil
	if err != nil {
		logger.Error("Failed to close cluster manager websocket", logger.Ctx{"err": err})
	}
}
