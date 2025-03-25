package api

import (
	"context"
	"database/sql"
	"errors"

	"github.com/canonical/microcluster/v2/state"

	"github.com/canonical/microcloud/microcloud/database"
)

// LoadClusterManagerId finds the cluster manager id from the database.
func LoadClusterManagerId(state state.State) (int64, error) {
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

// LoadClusterManagerConfig loads the cluster manager configuration from the database.
func LoadClusterManagerConfig(state state.State, ctx context.Context) (*database.ClusterManager, []database.ClusterManagerConfig, error) {
	var clusterManager *database.ClusterManager
	var updateIntervalConfig []database.ClusterManagerConfig

	err := state.Database().Transaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
		clusterManagerId, err := LoadClusterManagerId(state)
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
