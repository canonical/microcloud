package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/microcluster/rest"
	"github.com/canonical/microcluster/state"

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

// servicesPut updates the cluster status of the MicroCloud peer.
func servicesPut(s *state.State, r *http.Request) response.Response {
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

	sh, err := service.NewHandler(s.Name(), addr, s.OS.StateDir, false, false, services...)
	if err != nil {
		return response.SmartError(err)
	}

	err = sh.RunConcurrent(true, true, func(s service.Service) error {
		err = s.Join(joinConfigs[s.Type()])
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
