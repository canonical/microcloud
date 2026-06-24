package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/lxd/shared/version"
	microTypes "github.com/canonical/microcluster/v3/microcluster/types"
	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcloud/microcloud/database"
	"github.com/canonical/microcloud/microcloud/service"
)

const TunnelCheckIntervalSeconds = 10

// ClusterManagerTunnel represents the tunnel connection to the cluster manager.
type ClusterManagerTunnel struct {
	Mu     sync.RWMutex
	WsConn *websocket.Conn
	Wg     sync.WaitGroup
}

// ReconcileClusterManagerTunnel starts a go routine, that ensures the tunnel to cluster manager is in the right state.
func ReconcileClusterManagerTunnel(ctx context.Context, g *errgroup.Group, sh *service.Handler, s microTypes.State) {
	g.Go(func() error {
		// tunnel object to hold the websocket connection and its mutex for safe concurrent access
		tunnel := &ClusterManagerTunnel{
			WsConn: nil, // This will be set when the websocket connection is established
			Mu:     sync.RWMutex{},
			Wg:     sync.WaitGroup{},
		}

		ticker := time.NewTicker(TunnelCheckIntervalSeconds * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ensureTunnel(ctx, sh, s, tunnel)

			case <-ctx.Done():
				ensureTunnelClosed(tunnel)
				return nil // exit the loop and close the go routine
			}
		}
	})
}

func ensureTunnel(ctx context.Context, sh *service.Handler, s microTypes.State, tunnel *ClusterManagerTunnel) {
	logger.Debug("Starting ensureTunnel")

	cloud := sh.Services[types.MicroCloud].(*service.CloudService)
	isInitialized, err := cloud.IsInitialized(ctx)
	if err != nil {
		logger.Error("Failed to check if MicroCloud is initialized", logger.Ctx{"err": err})
		return
	}

	if !isInitialized {
		logger.Debug("MicroCloud not initialized")
		return
	}

	leaderClient, err := s.Database().Leader(ctx)
	if err != nil {
		logger.Error("Failed to get database leader client", logger.Ctx{"err": err})
		return
	}

	leaderInfo, err := leaderClient.Leader(ctx)
	if err != nil {
		logger.Error("Failed to get database leader info", logger.Ctx{"err": err})
		return
	}

	if leaderInfo.Address != s.Address().Host {
		ensureTunnelClosed(tunnel)
		return
	}

	clusterManager, err := database.LoadClusterManager(s, ctx, database.ClusterManagerDefaultName)
	if err != nil {
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			ensureTunnelClosed(tunnel)
			logger.Debug("Cluster manager not configured, skipping tunnel reconciliation")
			return
		}

		logger.Error("Failed to load cluster manager config", logger.Ctx{"err": err})
		return
	}

	needsTunnel, err := database.LoadClusterManagerReverseTunnel(s, ctx, clusterManager.ID)
	if err != nil {
		logger.Error("Failed to load reverse tunnel", logger.Ctx{"err": err})
		return
	}

	clusterManagerClient := client.NewClusterManagerClient(clusterManager)
	clusterCert, err := cloud.ClusterCert()
	if err != nil {
		logger.Error("Failed to get cluster certificate", logger.Ctx{"err": err})
		return
	}

	tunnel.Mu.Lock()
	hasConnection := tunnel.WsConn != nil
	tunnel.Mu.Unlock()

	if needsTunnel && hasConnection {
		logger.Debug("Tunnel already connected")
		return
	}

	if needsTunnel && !hasConnection {
		logger.Debug("Tunnel not connected, opening")
		tunnel.Wg.Go(func() {
			openTunnel(ctx, sh, tunnel, clusterManagerClient, clusterCert)
		})
		return
	}

	if !needsTunnel && hasConnection {
		logger.Debug("Tunnel connected but should be disabled, closing")
		ensureTunnelClosed(tunnel)
		return
	}

	logger.Debug("Tunnel disabled, finished ensure tunnel")
}

func openTunnel(ctx context.Context, sh *service.Handler, tunnel *ClusterManagerTunnel, clusterManagerClient *client.ClusterManagerClient, clusterCert *shared.CertInfo) {
	tunnel.Mu.Lock()
	locked := true
	defer func() {
		if locked {
			tunnel.Mu.Unlock()
		}
	}()
	if tunnel.WsConn != nil {
		// active tunnel, do not open tunnel
		return
	}

	conn, err := clusterManagerClient.ConnectTunnel(ctx, clusterCert)
	if err != nil {
		logger.Error("Failed to connect cluster manager tunnel", logger.Ctx{"err": err})
		return
	}

	// Mark the tunnel as active immediately to avoid concurrent open attempts.
	tunnel.WsConn = conn
	tunnel.Mu.Unlock()
	locked = false

	defer func() {
		err := conn.Close()
		if err != nil {
			logger.Error("Failed to close cluster manager tunnel", logger.Ctx{"err": err})
		}
	}()

	// Get the server certificate
	lxdService := sh.Services[types.LXD].(*service.LXDService)
	lxdClient, err := lxdService.Client(ctx)
	if err != nil {
		logger.Error("Failed to connect to LXD service", logger.Ctx{"err": err})
		return
	}

	server, _, err := lxdClient.GetServer()
	if err != nil {
		logger.Error("Failed to get LXD server info", logger.Ctx{"err": err})
		return
	}

	lxdServerCert := server.Environment.Certificate
	lxdHttpsAddress := fmt.Sprint(server.Config["core.https_address"])
	lxdPort := strconv.FormatInt(service.LXDPort, 10)
	if lxdHttpsAddress == "[::]:"+lxdPort || lxdHttpsAddress == ":"+lxdPort {
		cloud := sh.Services[types.MicroCloud].(*service.CloudService)
		lxdHttpsAddress = cloud.Address() + ":" + lxdPort
	}

	logger.Debug("Tunnel with cluster manager connected")

	for {
		var req types.ClusterManagerTunnelRequest
		err = conn.ReadJSON(&req)
		if err != nil {
			logger.Error("Cluster manager tunnel read error:", logger.Ctx{"err": err})
			ensureTunnelClosed(tunnel)
			return
		}

		logger.Debug("Cluster manager tunnel request received:", logger.Ctx{"path": req.Path, "body": req.Body})
		resp := handleTunnelRequest(req, lxdServerCert, lxdHttpsAddress)

		// Send back the response
		err = conn.WriteJSON(resp)
		if err != nil {
			logger.Error("Cluster manager tunnel write error:", logger.Ctx{"err": err})
			ensureTunnelClosed(tunnel)
			return
		}
	}
}

func handleTunnelRequest(req types.ClusterManagerTunnelRequest, lxdServerCert string, lxdHttpsAddress string) types.ClusterManagerTunnelResponse {
	if req.Path != "/1.0" && !strings.HasPrefix(req.Path, "/1.0/") {
		logger.Warn("Received tunnel request with invalid path prefix", logger.Ctx{"path": req.Path})
		return types.ClusterManagerTunnelResponse{UUID: req.UUID, Status: http.StatusBadRequest}
	}

	allowedMethods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
	}

	if !slices.Contains(allowedMethods, req.Method) {
		logger.Warn("Received tunnel request with unsupported HTTP method", logger.Ctx{"method": req.Method})
		return types.ClusterManagerTunnelResponse{UUID: req.UUID, Status: http.StatusMethodNotAllowed}
	}

	targetURL := "https://" + lxdHttpsAddress + req.Path
	newReq, err := http.NewRequest(req.Method, targetURL, bytes.NewReader(req.Body))
	if err != nil {
		logger.Error("Failed to create new HTTP request for tunnel", logger.Ctx{"err": err})
		return types.ClusterManagerTunnelResponse{UUID: req.UUID, Status: http.StatusInternalServerError}
	}

	newReq.Header.Set("Cookie", req.Headers.Get("Cookie"))
	newReq.Header.Set("Authorization", req.Headers.Get("Authorization"))
	newReq.Header.Set("User-Agent", version.UserAgent+" (cookiejar)")

	tlsConfig := shared.InitTLSConfig()
	tlsConfig.RootCAs = x509.NewCertPool()
	ok := tlsConfig.RootCAs.AppendCertsFromPEM([]byte(lxdServerCert))
	if !ok {
		logger.Error("Failed to parse LXD server certificate")
		return types.ClusterManagerTunnelResponse{UUID: req.UUID, Status: http.StatusUnauthorized}
	}

	lxdHttpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: 30 * time.Second,
	}

	lxdResponse, err := lxdHttpClient.Do(newReq)
	if err != nil {
		logger.Error("Error from LXD client query", logger.Ctx{"err": err, "path": req.Path, "method": req.Method})
		return types.ClusterManagerTunnelResponse{UUID: req.UUID, Status: http.StatusInternalServerError}
	}

	defer func() {
		err = lxdResponse.Body.Close()
		if err != nil {
			logger.Error("Failed to close LXD response body", logger.Ctx{"err": err})
		}
	}()

	responseBody, err := io.ReadAll(lxdResponse.Body)
	if err != nil {
		logger.Error("Failed to marshal LXD response", logger.Ctx{"err": err})
		return types.ClusterManagerTunnelResponse{UUID: req.UUID, Status: http.StatusInternalServerError}
	}

	return types.ClusterManagerTunnelResponse{
		UUID:    req.UUID,
		Status:  lxdResponse.StatusCode,
		Body:    responseBody,
		Cookies: lxdResponse.Cookies(),
		Headers: lxdResponse.Header,
	}
}

func ensureTunnelClosed(tunnel *ClusterManagerTunnel) {
	tunnel.Mu.Lock()
	locked := true
	defer func() {
		if locked {
			tunnel.Mu.Unlock()
		}
	}()

	if tunnel.WsConn == nil {
		return
	}

	logger.Debug("Closing cluster manager tunnel")
	err := tunnel.WsConn.Close()
	tunnel.WsConn = nil
	if err != nil {
		logger.Error("Failed to close cluster manager tunnel", logger.Ctx{"err": err})
	}

	tunnel.Mu.Unlock()
	locked = false
	tunnel.Wg.Wait() // wait for dedicated tunnel routine to finish
}
