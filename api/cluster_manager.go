package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/shared"
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
	clusterManager, updateIntervalConfig, err := LoadClusterManagerConfig(state, r.Context())
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
			existingId, err := LoadClusterManagerId(state)
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

	clusterManager, updateIntervalConfig, err := LoadClusterManagerConfig(state, r.Context())
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
		clusterManager, _, err := LoadClusterManagerConfig(state, r.Context())
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
