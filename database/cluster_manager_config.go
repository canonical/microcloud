package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/canonical/microcluster/v2/state"
)

// ClusterManagerDefaultName is the default name for the cluster manager.
const ClusterManagerDefaultName = "default"

// UpdateIntervalField is the field name for the update interval configuration.
const UpdateIntervalField = "UpdateInterval"

// ClusterManagerIsConfigured finds if cluster manager is setup in the database.
func ClusterManagerIsConfigured(state state.State, ctx context.Context) (bool, error) {
	_, err := loadDefaultClusterManager(ctx, state)
	if err != nil {
		if err.Error() == "Cluster manager not found" {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// LoadClusterManagerConfig loads the cluster manager configuration from the database.
func LoadClusterManagerConfig(state state.State, ctx context.Context) (*ClusterManager, []ClusterManagerConfig, error) {
	clusterManager, err := loadDefaultClusterManager(ctx, state)
	if err != nil {
		return nil, nil, err
	}

	var updateIntervalConfig []ClusterManagerConfig
	err = state.Database().Transaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
		updateIntervalField := UpdateIntervalField
		updateIntervalConfig, err = GetClusterManagerConfig(ctx, tx, ClusterManagerConfigFilter{
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

// SetClusterManagerStatusLastSuccess sets the last successful status time in the database.
func SetClusterManagerStatusLastSuccess(state state.State, ctx context.Context, successTime time.Time) error {
	clusterManager, err := loadDefaultClusterManager(ctx, state)
	if err != nil {
		return err
	}

	err = state.Database().Transaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
		clusterManager.StatusLastSuccessTime = successTime
		return UpdateClusterManager(ctx, tx, clusterManager.ID, *clusterManager)
	})

	return err
}

// SetClusterManagerStatusLastError sets the last error status time and response in the database.
func SetClusterManagerStatusLastError(state state.State, ctx context.Context, errorTime time.Time, errorResponse string) error {
	clusterManager, err := loadDefaultClusterManager(ctx, state)
	if err != nil {
		return err
	}

	err = state.Database().Transaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
		clusterManager.StatusLastErrorTime = errorTime
		clusterManager.StatusLastErrorResponse = errorResponse
		return UpdateClusterManager(ctx, tx, clusterManager.ID, *clusterManager)
	})

	return err
}

func loadDefaultClusterManager(ctx context.Context, state state.State) (*ClusterManager, error) {
	var clusterManager *ClusterManager

	err := state.Database().Transaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
		name := ClusterManagerDefaultName
		filter := ClusterManagerFilter{
			Name: &name,
		}

		clusterManagers, err := GetClusterManagers(ctx, tx, filter)
		if err != nil {
			return err
		}

		if len(clusterManagers) < 1 {
			return errors.New("Cluster manager not found")
		}

		clusterManager = &clusterManagers[0]

		return nil
	})
	if err != nil {
		return nil, err
	}

	return clusterManager, nil
}
