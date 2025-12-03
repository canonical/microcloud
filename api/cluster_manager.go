package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/microcluster/v3/microcluster/rest"
	"github.com/canonical/microcluster/v3/microcluster/rest/response"
	"github.com/canonical/microcluster/v3/state"
	"github.com/gorilla/mux"

	apiTypes "github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcloud/microcloud/database"
	"github.com/canonical/microcloud/microcloud/service"
)

// ClusterManagersJoinCmd represents the /1.0/cluster-managers API on MicroCloud.
var ClusterManagersJoinCmd = func(sh *service.Handler) rest.Endpoint {
	return rest.Endpoint{
		Path: "cluster-managers",

		Post: rest.EndpointAction{Handler: authHandlerMTLS(sh, clusterManagerPost(sh))},
	}
}

// ClusterManagersCmd represents the /1.0/cluster-managers/{name} API on MicroCloud.
var ClusterManagersCmd = func(sh *service.Handler) rest.Endpoint {
	return rest.Endpoint{
		Path: "cluster-managers/{name}",

		Delete: rest.EndpointAction{Handler: authHandlerMTLS(sh, clusterManagerDelete(sh))},
		Get:    rest.EndpointAction{Handler: authHandlerMTLS(sh, clusterManagerGet)},
		Put:    rest.EndpointAction{Handler: authHandlerMTLS(sh, clusterManagerPut)},
	}
}

// clusterManagerGet returns the cluster manager configuration.
func clusterManagerGet(state state.State, r *http.Request) response.Response {
	name, err := nameFromPath(r)
	if err != nil {
		return response.SmartError(err)
	}

	clusterManager, clusterManagerConfig, err := database.LoadClusterManager(state, r.Context(), name)
	if err != nil {
		return response.SmartError(err)
	}

	// convert clusterManagerConfig into a map string/string
	// to make it easier to access the values
	config := make(map[string]string)
	for _, configPair := range clusterManagerConfig {
		config[configPair.Key] = configPair.Value
	}

	resp := apiTypes.ClusterManager{
		Addresses:               []string{clusterManager.Addresses},
		CertificateFingerprint:  clusterManager.CertificateFingerprint,
		StatusLastSuccessTime:   clusterManager.StatusLastSuccessTime,
		StatusLastErrorTime:     clusterManager.StatusLastErrorTime,
		StatusLastErrorResponse: clusterManager.StatusLastErrorResponse,
		Config:                  config,
	}

	return response.SyncResponse(true, resp)
}

// clusterManagerPost creates a new cluster manager configuration from a token.
func clusterManagerPost(sh *service.Handler) func(state state.State, r *http.Request) response.Response {
	return func(state state.State, r *http.Request) response.Response {
		args := apiTypes.ClusterManagersPost{}
		err := json.NewDecoder(r.Body).Decode(&args)
		if err != nil {
			return response.BadRequest(err)
		}

		if args.Token == "" {
			return response.BadRequest(errors.New("No token provided"))
		}

		if args.Name != database.ClusterManagerDefaultName {
			errMsg := fmt.Sprintf("Invalid cluster manager name, only %s is allowed", database.ClusterManagerDefaultName)
			return response.BadRequest(errors.New(errMsg))
		}

		joinToken, err := shared.JoinTokenDecode(args.Token)
		if err != nil {
			return response.BadRequest(err)
		}

		// ensure cluster manager is not already configured
		existingClusterManager, _, err := database.LoadClusterManager(state, r.Context(), args.Name)
		if err != nil {
			if api.StatusErrorCheck(err, http.StatusNotFound) {
				// ignore, this is the expected path
			} else {
				return response.SmartError(err)
			}
		}
		if existingClusterManager != nil {
			return response.BadRequest(errors.New("Cluster manager already configured"))
		}

		clusterManager := database.ClusterManager{
			Name:                   args.Name,
			Addresses:              strings.Join(joinToken.Addresses, ","),
			CertificateFingerprint: joinToken.Fingerprint,
		}

		cloud := sh.Services[apiTypes.MicroCloud].(*service.CloudService)
		clusterCert, err := cloud.ClusterCert()
		if err != nil {
			return response.SmartError(err)
		}

		// register in remote cluster manager (also ensures the token is valid)
		clusterManagerClient := client.NewClusterManagerClient(&clusterManager)
		err = clusterManagerClient.Join(clusterCert, joinToken.ServerName, args.Token)
		if err != nil {
			return response.SmartError(err)
		}

		// store cluster manager configuration in local database
		err = state.Database().Transaction(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
			clusterManagerId, err := database.CreateClusterManager(ctx, tx, clusterManager)
			if err != nil {
				return err
			}

			updateIntervalConfig := database.ClusterManagerConfig{
				ClusterManagerID: clusterManagerId,
				Key:              database.UpdateIntervalSecondsKey,
				Value:            strconv.Itoa(database.UpdateIntervalDefaultSeconds),
			}

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
	name, err := nameFromPath(r)
	if err != nil {
		return response.SmartError(err)
	}

	clusterManager, _, err := database.LoadClusterManager(state, r.Context(), name)
	if err != nil {
		return response.SmartError(err)
	}

	args := apiTypes.ClusterManagerPut{}
	err = json.NewDecoder(r.Body).Decode(&args)
	if err != nil {
		return response.BadRequest(err)
	}

	hasChangedAddress := len(args.Addresses) > 0
	if hasChangedAddress {
		clusterManager.Addresses = strings.Join(args.Addresses, ",")
	}

	hasChangedFingerprint := args.CertificateFingerprint != nil
	if hasChangedFingerprint {
		clusterManager.CertificateFingerprint = *args.CertificateFingerprint
	}

	if hasChangedAddress || hasChangedFingerprint {
		err = database.StoreClusterManager(state, r.Context(), *clusterManager)
		if err != nil {
			return response.SmartError(err)
		}
	}

	if args.UpdateInterval != nil {
		err = database.StoreClusterManagerConfig(state, r.Context(), name, database.UpdateIntervalSecondsKey, *args.UpdateInterval)
		if err != nil {
			return response.SmartError(err)
		}
	}

	return response.SyncResponse(true, nil)
}

// clusterManagerDelete clears the cluster manager configuration.
func clusterManagerDelete(sh *service.Handler) func(state state.State, r *http.Request) response.Response {
	return func(state state.State, r *http.Request) response.Response {
		name, err := nameFromPath(r)
		if err != nil {
			return response.SmartError(err)
		}

		args := apiTypes.ClusterManagerDelete{}
		err = json.NewDecoder(r.Body).Decode(&args)
		if err != nil {
			return response.SmartError(err)
		}

		clusterManager, _, err := database.LoadClusterManager(state, r.Context(), name)
		if err != nil {
			return response.SmartError(err)
		}

		if !args.Force {
			cloud := sh.Services[apiTypes.MicroCloud].(*service.CloudService)
			clusterCert, err := cloud.ClusterCert()
			if err != nil {
				return response.SmartError(err)
			}

			clusterManagerClient := client.NewClusterManagerClient(clusterManager)
			err = clusterManagerClient.Delete(clusterCert)
			if err != nil {
				return response.SmartError(err)
			}
		}

		err = database.RemoveClusterManager(state, r.Context(), *clusterManager)
		if err != nil {
			return response.SmartError(err)
		}

		return response.SyncResponse(true, nil)
	}
}

func nameFromPath(r *http.Request) (string, error) {
	name, err := url.PathUnescape(mux.Vars(r)["name"])
	if err != nil {
		return "", err
	}

	if name != database.ClusterManagerDefaultName {
		return "", errors.New("Only default cluster manager is supported")
	}

	return name, nil
}
