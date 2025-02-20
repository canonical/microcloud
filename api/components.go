package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/microcluster/v2/rest"
	"github.com/canonical/microcluster/v2/state"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/component"
)

// ComponentCmd represents the /1.0/components API on MicroCloud.
var ComponentCmd = func(sh *component.Handler) rest.Endpoint {
	return rest.Endpoint{
		AllowedBeforeInit: true,
		Name:              "components",
		Path:              "components",

		Put: rest.EndpointAction{Handler: authHandlerMTLS(sh, componentsPut), ProxyTarget: true},
	}
}

// componentsPut updates the cluster status of the MicroCloud peer.
func componentsPut(state state.State, r *http.Request) response.Response {
	// Parse the request.
	req := types.ComponentsPut{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return response.BadRequest(err)
	}

	joinConfigs := map[types.ComponentType]component.JoinConfig{}
	components := make([]types.ComponentType, len(req.Tokens))
	for i, cfg := range req.Tokens {
		components[i] = types.ComponentType(cfg.Component)
		joinConfigs[cfg.Component] = component.JoinConfig{Token: cfg.JoinToken, LXDConfig: req.LXDConfig, CephConfig: req.CephConfig, OVNConfig: req.OVNConfig}
	}

	// Default to the first iface if none specified.
	addr := util.NetworkInterfaceAddress()
	if req.Address != "" {
		addr = req.Address
	}

	sh, err := component.NewHandler(state.Name(), addr, state.FileSystem().StateDir, components...)
	if err != nil {
		return response.SmartError(err)
	}

	err = sh.RunConcurrent(types.MicroCloud, types.LXD, func(s component.Component) error {
		// set a 5 minute context for completing the join request in case the system is very slow.
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()

		err = s.Join(ctx, joinConfigs[s.Type()])
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
