package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/canonical/microcluster/rest"
	"github.com/canonical/microcluster/state"
	"github.com/lxc/lxd/lxd/response"
	"github.com/lxc/lxd/lxd/util"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/service"
)

var ServicesCmd = rest.Endpoint{
	AllowedBeforeInit: true,
	Name:              "services",
	Path:              "services",

	Put: rest.EndpointAction{Handler: servicesPut, AllowUntrusted: true, ProxyTarget: true},
}

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
		joinConfigs[cfg.Service] = service.JoinConfig{Token: cfg.JoinToken, LXDConfig: req.LXDConfig}
	}

	serverCert, err := s.OS.ServerCert()
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to compare join request fingerprints: %w", err))
	}

	// Skip the join request if the fingerprint does not match what we expect.
	if serverCert.Fingerprint() != req.Fingerprint {
		return response.SmartError(fmt.Errorf("Join request fingerprint does not match issuer config, ignoring join request"))
	}

	// Default to the first iface if none specified.
	addr := util.NetworkInterfaceAddress()
	if req.Address != "" {
		addr = req.Address
	}

	sh, err := service.NewServiceHandler(s.Name(), addr, s.OS.StateDir, false, false, services...)
	if err != nil {
		return response.SmartError(err)
	}

	err = sh.RunConcurrent(true, func(s service.Service) error {
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
