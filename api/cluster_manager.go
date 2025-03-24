package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/microcluster/v2/rest"
	"github.com/canonical/microcluster/v2/state"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/database"
	"github.com/canonical/microcloud/microcloud/service"
)

const updateIntervalField = "UpdateInterval"
const updateIntervalDefaultValue = "60"

// ClusterManagerCmd represents the /1.0/cluster-manager API on MicroCloud.
var ClusterManagerCmd = func(sh *service.Handler) rest.Endpoint {
	return rest.Endpoint{
		Path: "cluster-manager",

		Delete: rest.EndpointAction{Handler: authHandlerMTLS(sh, clusterManagerDelete(sh))},
		Get:    rest.EndpointAction{Handler: authHandlerMTLS(sh, clusterManagerGet)},
		Post:   rest.EndpointAction{Handler: authHandlerMTLS(sh, clusterManagerPost(sh))},
		Put:    rest.EndpointAction{Handler: authHandlerMTLS(sh, clusterManagerPut)},
	}
}

// clusterManagerGet returns the cluster manager configuration.
func clusterManagerGet(state state.State, r *http.Request) response.Response {
	clusterManager, updateIntervalConfig, err := loadClusterManagerConfig(state, r.Context())
	if err != nil {
		return response.SmartError(err)
	}

	if clusterManager.Addresses == "" {
		return response.SyncResponse(true, types.ClusterManager{})
	}

	var updateInterval string
	if len(updateIntervalConfig) > 0 {
		updateInterval = updateIntervalConfig[0].Value
	}

	resp := types.ClusterManager{
		Addresses:      []string{clusterManager.Addresses},
		Fingerprint:    &clusterManager.Fingerprint,
		UpdateInterval: &updateInterval,
	}

	return response.SyncResponse(true, resp)
}

// clusterManagerPost creates a new cluster manager configuration from a token.
func clusterManagerPost(sh *service.Handler) func(state state.State, r *http.Request) response.Response {
	return func(state state.State, r *http.Request) response.Response {
		args := types.ClusterManagerPost{}
		err := json.NewDecoder(r.Body).Decode(&args)
		if err != nil {
			return response.BadRequest(err)
		}

		if args.Token == "" {
			return response.BadRequest(errors.New("No token provided"))
		}

		joinToken, err := shared.JoinTokenDecode(args.Token)
		if err != nil {
			return response.BadRequest(err)
		}

		// ensure cluster manager is not already configured
		err = state.Database().Transaction(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
			existingId, err := loadClusterManagerId(state)
			if err != nil {
				return err
			}

			if existingId > 0 {
				return errors.New("Cluster manager already configured.")
			}

			return nil
		})
		if err != nil {
			return response.SmartError(err)
		}

		clusterManager := database.ClusterManager{
			Addresses:   strings.Join(joinToken.Addresses, ","),
			Fingerprint: joinToken.Fingerprint,
		}

		// register in remote cluster manager (also ensures the token is valid)
		clusterManagerClient := NewClusterManagerClient(&clusterManager)
		err = clusterManagerClient.PostJoin(sh, joinToken.ServerName, joinToken.Secret)
		if err != nil {
			return response.SmartError(err)
		}

		updateIntervalConfig := database.ClusterManagerConfig{
			Field: updateIntervalField,
			Value: updateIntervalDefaultValue,
		}

		// store cluster manager configuration in local database
		err = state.Database().Transaction(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
			clusterManagerId, err := database.CreateClusterManager(ctx, tx, clusterManager)
			if err != nil {
				return err
			}

			updateIntervalConfig.ClusterManagerID = clusterManagerId

			_, err = database.CreateClusterManagerConfig(ctx, tx, updateIntervalConfig)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return response.SmartError(err)
		}

		return response.SyncResponse(true, nil)
	}
}

// clusterManagerPut updates the cluster manager configuration.
func clusterManagerPut(state state.State, r *http.Request) response.Response {
	args := types.ClusterManager{}
	err := json.NewDecoder(r.Body).Decode(&args)
	if err != nil {
		return response.BadRequest(err)
	}

	clusterManager, updateIntervalConfig, err := loadClusterManagerConfig(state, r.Context())
	if err != nil {
		return response.SmartError(err)
	}

	if len(args.Addresses) > 0 {
		clusterManager.Addresses = strings.Join(args.Addresses, ",")
	}

	if args.Fingerprint != nil {
		clusterManager.Fingerprint = *args.Fingerprint
	}

	err = state.Database().Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		err = database.UpdateClusterManager(ctx, tx, clusterManager.ID, *clusterManager)
		if err != nil {
			return err
		}

		if args.UpdateInterval == nil {
			return nil
		}

		if *args.UpdateInterval == "" && len(updateIntervalConfig) > 0 {
			// clear update interval
			err = database.DeleteClusterManagerConfig(ctx, tx, updateIntervalConfig[0].ID)
			if err != nil {
				return err
			}
		} else if *args.UpdateInterval != "" && len(updateIntervalConfig) == 0 {
			// create update interval
			_, err = database.CreateClusterManagerConfig(ctx, tx, database.ClusterManagerConfig{
				ClusterManagerID: clusterManager.ID,
				Field:            updateIntervalField,
				Value:            *args.UpdateInterval,
			})
			if err != nil {
				return err
			}
		} else if *args.UpdateInterval != "" && len(updateIntervalConfig) > 0 {
			// update update interval
			updateIntervalConfig[0].Value = *args.UpdateInterval
			err = database.UpdateClusterManagerConfig(ctx, tx, updateIntervalConfig[0].ID, updateIntervalConfig[0])
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponse(true, nil)
}

// clusterManagerDelete clears the cluster manager configuration.
func clusterManagerDelete(sh *service.Handler) func(state state.State, r *http.Request) response.Response {
	return func(state state.State, r *http.Request) response.Response {
		clusterManager, _, err := loadClusterManagerConfig(state, r.Context())
		if err != nil {
			return response.SmartError(err)
		}

		if clusterManager.Addresses == "" {
			return response.SyncResponse(true, nil)
		}

		err = state.Database().Transaction(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
			err = database.DeleteClusterManager(ctx, tx, clusterManager.ID)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return response.SmartError(err)
		}

		clusterManagerClient := NewClusterManagerClient(clusterManager)
		err = clusterManagerClient.Delete(sh)
		if err != nil {
			return response.SmartError(err)
		}

		return response.SyncResponse(true, nil)
	}
}

func loadClusterManagerId(state state.State) (int64, error) {
	var maxId int64 = -1

	err := state.Database().Transaction(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		managers, err := database.GetClusterManagers(ctx, tx)
		if err != nil {
			return err
		}

		// get max id from cluster managers
		for _, manager := range managers {
			if manager.ID > maxId {
				maxId = manager.ID
			}
		}

		return nil
	})
	if err != nil {
		return -1, err
	}

	return maxId, nil
}

func loadClusterManagerConfig(state state.State, ctx context.Context) (*database.ClusterManager, []database.ClusterManagerConfig, error) {
	var clusterManager *database.ClusterManager
	var updateIntervalConfig []database.ClusterManagerConfig

	err := state.Database().Transaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
		clusterManagerId, err := loadClusterManagerId(state)
		if err != nil {
			return err
		}

		if clusterManagerId == -1 {
			return errors.New("Cluster manager not configured")
		}

		clusterManager, err = database.GetClusterManager(ctx, tx, clusterManagerId)
		if err != nil {
			return err
		}

		updateIntervalField := updateIntervalField
		updateIntervalConfig, err = database.GetClusterManagerConfig(ctx, tx, database.ClusterManagerConfigFilter{
			Field:            &updateIntervalField,
			ClusterManagerID: &clusterManager.ID,
		})
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return clusterManager, updateIntervalConfig, nil
}

// StatusDistribution represents the status distribution of items.
type StatusDistribution struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// ClusterManagerStatusPost represents the status message sent to cluster manager.
type ClusterManagerStatusPost struct {
	CPUTotalCount     int64                `json:"cpu_total_count"`
	CPULoad1          string               `json:"cpu_load_1"`
	CPULoad5          string               `json:"cpu_load_5"`
	CPULoad15         string               `json:"cpu_load_15"`
	MemoryTotalAmount int64                `json:"memory_total_amount"`
	MemoryUsage       int64                `json:"memory_usage"`
	DiskTotalSize     int64                `json:"disk_total_size"`
	DiskUsage         int64                `json:"disk_usage"`
	MemberStatuses    []StatusDistribution `json:"member_statuses"`
	InstanceStatuses  []StatusDistribution `json:"instance_status"`
	Metrics           string               `json:"metrics"`
	UiUrl             string               `json:"ui_url"`
}

func sendClusterManagerStatusMessage(ctx context.Context, sh *service.Handler, s state.State) time.Duration {
	logger.Debug("Running sendClusterManagerStatusMessage")
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

	clusterManagerId, err := loadClusterManagerId(s)
	if err != nil {
		logger.Error("Failed to get cluster manager id", logger.Ctx{"err": err})
		return nextUpdate
	}

	if clusterManagerId == -1 {
		logger.Debug("Cluster manager not configured")
		return nextUpdate
	}

	clusterManager, updateIntervalConfig, err := loadClusterManagerConfig(s, ctx)
	if err != nil {
		logger.Error("Failed to load cluster manager config", logger.Ctx{"err": err})
		return nextUpdate
	}

	if len(updateIntervalConfig) > 0 {
		interval, err := time.ParseDuration(updateIntervalConfig[0].Value + "s")
		if err != nil {
			logger.Error("Failed to parse update interval", logger.Ctx{"err": err})
			return nextUpdate
		}

		nextUpdate = interval
	}

	payload := ClusterManagerStatusPost{}

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
		logger.Error("Failed to enrich cluster member metrics", logger.Ctx{"err": err})
		return nextUpdate
	}

	err = enrichClusterMemberMetrics(lxdClient, &payload)
	if err != nil {
		logger.Error("Failed to enrich cluster member metrics", logger.Ctx{"err": err})
		return nextUpdate
	}

	logger.Debug("Sending status message to cluster manager")

	clusterManagerClient := NewClusterManagerClient(clusterManager)
	err = clusterManagerClient.PostStatus(sh, payload)
	if err != nil {
		logger.Error("Failed to send status message to cluster manager", logger.Ctx{"err": err})
		return nextUpdate
	}

	logger.Debug("Done sending status message to cluster manager")
	return nextUpdate
}

func enrichInstanceMetrics(lxdClient lxd.InstanceServer, result *ClusterManagerStatusPost) error {
	instanceFrequencies := make(map[string]int64)

	instanceList, err := lxdClient.GetInstancesAllProjects(api.InstanceTypeAny)
	for i := range instanceList {
		inst := instanceList[i]
		instanceFrequencies[inst.Status]++
	}

	for status, count := range instanceFrequencies {
		result.InstanceStatuses = append(result.InstanceStatuses, StatusDistribution{
			Status: status,
			Count:  count,
		})
	}

	return err
}

func enrichServerMetrics(lxdClient lxd.InstanceServer, result *ClusterManagerStatusPost) error {
	metrics, err := lxdClient.GetMetrics()
	if err != nil {
		return fmt.Errorf("Failed to get LXD metrics: %w", err)
	}

	result.Metrics = metrics

	return nil
}

func enrichClusterMemberMetrics(lxdClient lxd.InstanceServer, result *ClusterManagerStatusPost) error {
	lxdMembers, err := lxdClient.GetClusterMembers()
	if err != nil {
		return fmt.Errorf("Failed to get LXD cluster members: %w", err)
	}

	if len(lxdMembers) > 0 {
		result.UiUrl = lxdMembers[0].URL
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
			return err
		}

		result.MemoryTotalAmount += int64(memberState.SysInfo.TotalRAM)
		result.MemoryUsage += int64(memberState.SysInfo.TotalRAM - memberState.SysInfo.FreeRAM)

		cpuLoad1 += memberState.SysInfo.LoadAverages[0]
		cpuLoad5 += memberState.SysInfo.LoadAverages[1]
		cpuLoad15 += memberState.SysInfo.LoadAverages[2]

		for _, poolsState := range memberState.StoragePools {
			result.DiskTotalSize += int64(poolsState.Space.Total)
			result.DiskUsage += int64(poolsState.Space.Used)
		}
	}

	for status, count := range statusFrequencies {
		result.MemberStatuses = append(result.MemberStatuses, StatusDistribution{
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

// SendClusterManagerStatusMessageTask starts a dedicated go routine, that sends the cluster manager status message.
func SendClusterManagerStatusMessageTask(ctx context.Context, sh *service.Handler, s state.State) {
	go func(ctx context.Context, sh *service.Handler, s state.State) {
		updateTime := 5 * time.Second
		for {
			time.Sleep(updateTime)
			newUpdateTime := sendClusterManagerStatusMessage(ctx, sh, s)
			if newUpdateTime > 0 {
				updateTime = newUpdateTime
			}
		}
	}(ctx, sh, s)
}
