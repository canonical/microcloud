package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/microcluster/rest"
	"github.com/canonical/microcluster/state"
	"github.com/gorilla/mux"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/service"
)

// endpointHandler is just a convenience for writing clean return types.
type endpointHandler func(*state.State, *http.Request) response.Response

// authHandler ensures a request has been authenticated with the mDNS broadcast secret.
func authHandler(sh *service.Handler, f endpointHandler) endpointHandler {
	return func(s *state.State, r *http.Request) response.Response {
		if r.RemoteAddr == "@" {
			logger.Debug("Allowing unauthenticated request through unix socket")

			return f(s, r)
		}

		secret := r.Header.Get("X-MicroCloud-Auth")
		if secret == "" {
			return response.BadRequest(fmt.Errorf("No auth secret in response"))
		}

		if sh.AuthSecret == "" {
			return response.BadRequest(fmt.Errorf("No generated auth secret"))
		}

		if sh.AuthSecret != secret {
			return response.SmartError(fmt.Errorf("Request secret does not match, ignoring request"))
		}

		return f(s, r)
	}
}

// ServicesCmd represents the /1.0/services API on MicroCloud.
var ServicesCmd = func(sh *service.Handler) rest.Endpoint {
	return rest.Endpoint{
		AllowedBeforeInit: true,
		Name:              "services",
		Path:              "services",

		Put: rest.EndpointAction{Handler: authHandler(sh, servicesPut), AllowUntrusted: true, ProxyTarget: true},
	}
}

// ServiceTokensCmd represents the /1.0/services/serviceType/tokens API on MicroCloud.
var ServiceTokensCmd = func(sh *service.Handler) rest.Endpoint {
	return rest.Endpoint{
		AllowedBeforeInit: true,
		Name:              "services/{serviceType}/tokens",
		Path:              "services/{serviceType}/tokens",

		Post: rest.EndpointAction{Handler: authHandler(sh, serviceTokensPost), AllowUntrusted: true, ProxyTarget: true},
	}
}

// serviceTokensPost issues a token for service using the MicroCloud proxy.
// Normally a token request to a service would be restricted to trusted systems,
// so this endpoint validates the mDNS auth token and then proxies the request to the local unix socket of the remote system.
func serviceTokensPost(s *state.State, r *http.Request) response.Response {
	serviceType, err := url.PathUnescape(mux.Vars(r)["serviceType"])
	if err != nil {
		return response.SmartError(err)
	}

	// Parse the request.
	req := types.ServiceTokensPost{}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return response.BadRequest(err)
	}

	sh, err := service.NewHandler(s.Name(), req.ClusterAddress, s.OS.StateDir, false, false, types.ServiceType(serviceType))
	if err != nil {
		return response.SmartError(err)
	}

	token, err := sh.Services[types.ServiceType(serviceType)].IssueToken(s.Context, req.JoinerName)
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to issue %s token for peer %q: %w", serviceType, req.JoinerName, err))
	}

	return response.SyncResponse(true, token)
}

// servicesPut updates the cluster status of the MicroCloud peer.
func servicesPut(state *state.State, r *http.Request) response.Response {
	// Parse the request.
	req := types.ServicesPut{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return response.BadRequest(err)
	}

	joinConfigs := map[types.ServiceType]service.JoinConfig{}
	services := make([]types.ServiceType, len(req.Tokens))
	for i, cfg := range req.Tokens {
		services[i] = types.ServiceType(cfg.Service)
		joinConfigs[cfg.Service] = service.JoinConfig{Token: cfg.JoinToken, LXDConfig: req.LXDConfig, CephConfig: req.CephConfig}
	}

	// Default to the first iface if none specified.
	addr := util.NetworkInterfaceAddress()
	if req.Address != "" {
		addr = req.Address
	}

	sh, err := service.NewHandler(state.Name(), addr, state.OS.StateDir, false, false, services...)
	if err != nil {
		return response.SmartError(err)
	}

	err = sh.RunConcurrent(true, true, func(s service.Service) error {
		err = s.Join(state.Context, joinConfigs[s.Type()])
		if err != nil {
			return fmt.Errorf("Failed to join %q cluster: %w", s.Type(), err)
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	return response.EmptySyncResponse
}
