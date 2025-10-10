package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/microcluster/v2/state"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcloud/microcloud/database"
	"github.com/canonical/microcloud/microcloud/service"
)

// SendClusterManagerStatusMessageTask starts a go routine, that sends periodic status messages to cluster manager.
func SendClusterManagerStatusMessageTask(ctx context.Context, sh *service.Handler, s state.State) {
	go func(ctx context.Context, sh *service.Handler, s state.State) {
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

func sendClusterManagerStatusMessage(ctx context.Context, sh *service.Handler, s state.State, tunnel *types.ClusterManagerTunnel) time.Duration {
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

	if leaderInfo.Address != s.Address().URL.Host {
		logger.Debug("Not the leader, skipping status message")
		return nextUpdate
	}

	ensureTunnel(ctx, sh, s, tunnel, hasReverseTunnel)

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

		for _, poolsState := range memberState.StoragePools {
			result.DiskTotalSize += int64(poolsState.Space.Total)
			result.DiskUsage += int64(poolsState.Space.Used)
		}
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

func ensureTunnel(ctx context.Context, sh *service.Handler, s state.State, tunnel *types.ClusterManagerTunnel, hasTunnel bool) {
	if hasTunnel && tunnel.WsConn != nil {
		logger.Debug("Websocket already connected, skipping reconnection")
		return
	}

	if hasTunnel && tunnel.WsConn == nil {
		logger.Debug("Websocket not connected, establishing connection in new goroutine")
		go openTunnel(ctx, sh, s, tunnel)
		return
	}

	if !hasTunnel && tunnel.WsConn != nil {
		logger.Debug("Websocket connected but reverse tunnel is disabled, closing connection")
		tunnel.Mu.Lock()
		if tunnel.WsConn != nil {
			tunnel.WsConn.Close()
			tunnel.WsConn = nil
		}

		tunnel.Mu.Unlock()
		return
	}

	logger.Debug("Reverse tunnel is disabled, not opening websocket connection")
}

func openTunnel(ctx context.Context, sh *service.Handler, s state.State, tunnel *types.ClusterManagerTunnel) {
	logger.Error("Connecting ws")

	clusterManager, _, err := database.LoadClusterManager(s, ctx, database.ClusterManagerDefaultName)
	if err != nil {
		logger.Error("Failed to load cluster manager config", logger.Ctx{"err": err})
		return
	}

	cloud := sh.Services[types.MicroCloud].(*service.CloudService)
	clusterCert, err := cloud.ClusterCert()
	if err != nil {
		logger.Error("Failed to get cluster certificate", logger.Ctx{"err": err})
		return
	}

	clusterManagerClient := client.NewClusterManagerClient(clusterManager)
	conn, err := clusterManagerClient.ConnectTunnelWebsocket(clusterCert)
	if err != nil {
		logger.Error("Failed to connect to cluster manager websocket", logger.Ctx{"err": err})
		return
	}

	tunnel.Mu.Lock()
	tunnel.WsConn = conn
	tunnel.Mu.Unlock()

	defer conn.Close()

	// Handle CTRL+C to gracefully close
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	done := make(chan struct{})

	logger.Error("Connected to cluster manager websocket")

	defer close(done)
	for {
		var req types.ClusterManagerTunnelRequest
		err = conn.ReadJSON(&req)
		if err != nil {
			logger.Error("Read error:", logger.Ctx{"err": err})
			tunnel.Mu.Lock()
			tunnel.WsConn = nil
			tunnel.Mu.Unlock()
			return
		}

		logger.Error("Received request:", logger.Ctx{"path": req.Path})
		resp := handleTunnelRequest(req, sh)

		// Send back the response
		err = conn.WriteJSON(resp)
		if err != nil {
			logger.Error("Write error:", logger.Ctx{"err": err})
			tunnel.Mu.Lock()
			tunnel.WsConn = nil
			tunnel.Mu.Unlock()
			return
		}
	}
}

func handleTunnelRequest(req types.ClusterManagerTunnelRequest, sh *service.Handler) types.ClusterManagerTunnelResponse {
	// todo use the lxd service instead of raw connection
	reqUrl := "https://localhost:8443" + req.Path
	httpReq, err := http.NewRequest(req.Method, reqUrl, bytes.NewReader(req.Body))
	if err != nil {
		logger.Error("Request build error", logger.Ctx{"err": err, "path": req.Path, "method": req.Method})
		return types.ClusterManagerTunnelResponse{ID: req.ID, Status: http.StatusInternalServerError}
	}

	// Copy headers
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	tlsConfig := shared.InitTLSConfig()
	tlsConfig.InsecureSkipVerify = true // todo For testing purposes, skip verification of the server's certificate

	cloud := sh.Services[types.MicroCloud].(*service.CloudService)
	clusterCert, err := cloud.ClusterCert()
	if err != nil {
		logger.Error("Failed to get cluster certificate", logger.Ctx{"err": err})
		return types.ClusterManagerTunnelResponse{ID: req.ID, Status: http.StatusInternalServerError}
	}

	// todo we are using the cluster certificate to authenticate the request. This must be avoided and replaced with the client being authenticated instead.
	cert := clusterCert.KeyPair()
	tlsConfig.GetClientCertificate = func(info *tls.CertificateRequestInfo) (*tls.Certificate, error) {
		// GetClientCertificate is called if not nil instead of performing the default selection of an appropriate
		// certificate from the `Certificates` list. We only have one-key pair to send, and we always want to send it
		// because this is what uniquely identifies the caller to the server.
		return &cert, nil
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	// Send to internal HTTP handler
	httpClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Return an error to prevent following redirects
			return http.ErrUseLastResponse
		},
	}

	httpClient.Transport = transport
	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		logger.Error("Internal request failed: %v", logger.Ctx{"err": err, "reqUrl": reqUrl, "method": req.Method})
		return types.ClusterManagerTunnelResponse{ID: req.ID, Status: http.StatusBadGateway}
	}

	defer httpResp.Body.Close()

	body, _ := io.ReadAll(httpResp.Body)

	headers := make(map[string]string)
	for k, v := range httpResp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	return types.ClusterManagerTunnelResponse{
		ID:      req.ID,
		Status:  httpResp.StatusCode,
		Headers: headers,
		Body:    body,
	}
}
