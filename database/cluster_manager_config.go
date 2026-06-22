package database

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/microcluster/v3/microcluster/types"
)

// ClusterManagerDefaultName is the default name for the cluster manager.
const ClusterManagerDefaultName = "default"

// UpdateIntervalSecondsKey is the key for the update interval configuration.
const UpdateIntervalSecondsKey = "update_interval_seconds"

// ReverseTunnelKey is the key for enabling or disabling the websocket in configuration.
const ReverseTunnelKey = "reverse_tunnel"

// UpdateIntervalDefaultSeconds is the interval for the status update task if none is defined in the database.
const UpdateIntervalDefaultSeconds = 60

// LoadClusterManager loads the cluster manager configuration from the database.
func LoadClusterManager(state types.State, ctx context.Context, name string) (*ClusterManager, error) {
	clusterManager, err := loadClusterManagerFromDb(ctx, state, name)
	if err != nil {
		return nil, err
	}

	return clusterManager, nil
}

// LoadClusterManagerConfigs loads all cluster manager configurations from the database.
func LoadClusterManagerConfigs(state types.State, ctx context.Context, clusterManagerId int64) ([]ClusterManagerConfig, error) {
	var clusterManagerConfig []ClusterManagerConfig
	var err error
	err = state.Database().Transaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
		clusterManagerConfig, err = GetClusterManagerConfig(ctx, tx, ClusterManagerConfigFilter{
			ClusterManagerID: &clusterManagerId,
		})

		return err
	})
	if err != nil {
		return nil, err
	}

	return clusterManagerConfig, nil
}

// LoadClusterManagerSingleConfig loads a single cluster manager configuration by key from the database.
func LoadClusterManagerSingleConfig(state types.State, ctx context.Context, clusterManagerId int64, configKey string) (*ClusterManagerConfig, error) {
	var clusterManagerConfig []ClusterManagerConfig
	var err error
	err = state.Database().Transaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
		clusterManagerConfig, err = GetClusterManagerConfig(ctx, tx, ClusterManagerConfigFilter{
			ClusterManagerID: &clusterManagerId,
			Key:              &configKey,
		})

		return err
	})
	if err != nil {
		return nil, err
	}

	if len(clusterManagerConfig) == 0 {
		return nil, nil
	}

	if len(clusterManagerConfig) > 1 {
		return nil, errors.New("Cannot load cluster manager config: multiple rows found for the same key")
	}

	return &clusterManagerConfig[0], nil
}

// LoadClusterManagerUpdateIntervalSeconds loads the cluster manager update interval configuration from the database.
func LoadClusterManagerUpdateIntervalSeconds(state types.State, ctx context.Context, clusterManagerId int64) (*time.Duration, error) {
	updateIntervalConfig, err := LoadClusterManagerSingleConfig(state, ctx, clusterManagerId, UpdateIntervalSecondsKey)
	if err != nil {
		return nil, err
	}

	if updateIntervalConfig == nil {
		return nil, api.StatusErrorf(http.StatusNotFound, "update interval not found")
	}

	updateInterval, err := time.ParseDuration(updateIntervalConfig.Value + "s")
	if err != nil {
		return nil, err
	}

	return &updateInterval, nil
}

// LoadClusterManagerReverseTunnel loads the cluster manager reverse tunnel configuration from the database.
func LoadClusterManagerReverseTunnel(state types.State, ctx context.Context, clusterManagerId int64) (bool, error) {
	reverseTunnelConfig, err := LoadClusterManagerSingleConfig(state, ctx, clusterManagerId, ReverseTunnelKey)
	if err != nil {
		return false, err
	}

	if reverseTunnelConfig == nil {
		return false, nil
	}

	reverseTunnel, err := strconv.ParseBool(reverseTunnelConfig.Value)
	if err != nil {
		return false, err
	}

	return reverseTunnel, nil
}

// StoreClusterManager stores the cluster manager configuration in the database.
func StoreClusterManager(state types.State, ctx context.Context, clusterManager ClusterManager) error {
	err := state.Database().Transaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
		err := UpdateClusterManager(ctx, tx, clusterManager.ID, clusterManager)
		return err
	})
	return err
}

// RemoveClusterManager removes the cluster manager configuration from the database.
func RemoveClusterManager(state types.State, ctx context.Context, clusterManager ClusterManager) error {
	err := state.Database().Transaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
		err := DeleteClusterManager(ctx, tx, clusterManager.ID)
		return err
	})
	return err
}

// StoreClusterManagerConfig stores the cluster manager configuration in the database.
func StoreClusterManagerConfig(state types.State, ctx context.Context, name string, key string, value string) error {
	clusterManager, err := LoadClusterManager(state, ctx, name)
	if err != nil {
		return err
	}

	clusterManagerConfig, err := LoadClusterManagerConfigs(state, ctx, clusterManager.ID)
	if err != nil {
		return err
	}

	var existingConfig *ClusterManagerConfig = nil
	for _, config := range clusterManagerConfig {
		if config.Key == key {
			existingConfig = &config
		}
	}

	err = state.Database().Transaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if value == "" && existingConfig != nil {
			// clear
			err = DeleteClusterManagerConfig(ctx, tx, existingConfig.ID)
		} else if value != "" && existingConfig == nil {
			// create
			_, err = CreateClusterManagerConfig(ctx, tx, ClusterManagerConfig{
				ClusterManagerID: clusterManager.ID,
				Key:              key,
				Value:            value,
			})
		} else if value != "" && existingConfig != nil {
			// update
			existingConfig.Value = value
			err = UpdateClusterManagerConfig(ctx, tx, existingConfig.ID, *existingConfig)
		}

		return err
	})
	return err
}

// SetClusterManagerStatusLastSuccess sets the last successful status time in the database.
func SetClusterManagerStatusLastSuccess(state types.State, ctx context.Context, name string, successTime time.Time) error {
	clusterManager, err := loadClusterManagerFromDb(ctx, state, name)
	if err != nil {
		return err
	}

	clusterManager.StatusLastSuccessTime = successTime
	clusterManager.StatusLastErrorResponse = ""
	clusterManager.StatusLastErrorTime = time.Time{}

	err = state.Database().Transaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
		return UpdateClusterManager(ctx, tx, clusterManager.ID, *clusterManager)
	})

	return err
}

// SetClusterManagerStatusLastError sets the last error status time and response in the database.
func SetClusterManagerStatusLastError(state types.State, ctx context.Context, name string, errorTime time.Time, errorResponse string) error {
	clusterManager, err := loadClusterManagerFromDb(ctx, state, name)
	if err != nil {
		return err
	}

	clusterManager.StatusLastErrorTime = errorTime
	clusterManager.StatusLastErrorResponse = errorResponse

	err = state.Database().Transaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
		return UpdateClusterManager(ctx, tx, clusterManager.ID, *clusterManager)
	})

	return err
}

func loadClusterManagerFromDb(ctx context.Context, state types.State, name string) (*ClusterManager, error) {
	var clusterManager *ClusterManager

	err := state.Database().Transaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
		filter := ClusterManagerFilter{
			Name: &name,
		}

		clusterManagers, err := GetClusterManagers(ctx, tx, filter)
		if err != nil {
			return err
		}

		if len(clusterManagers) < 1 {
			return api.StatusErrorf(http.StatusNotFound, "Cluster manager not found")
		}

		clusterManager = &clusterManagers[0]

		return nil
	})
	if err != nil {
		return nil, err
	}

	return clusterManager, nil
}
