package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/microcluster/v2/rest"
	"github.com/canonical/microcluster/v2/state"
	"github.com/gorilla/mux"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/service"
)

// ServiceTokensCmd represents the /1.0/services/serviceType/tokens API on MicroCloud.
var ServiceTokensCmd = func(sh *service.Handler) rest.Endpoint {
	return rest.Endpoint{
		AllowedBeforeInit: true,
		Name:              "services/{serviceType}/tokens",
		Path:              "services/{serviceType}/tokens",

		Post: rest.EndpointAction{Handler: authHandlerMTLS(sh, serviceTokensPost), ProxyTarget: true},
	}
}

// serviceTokensPost issues a token for service using the MicroCloud proxy.
// Normally a token request to a service would be restricted to trusted systems,
// so this endpoint validates the mDNS auth token and then proxies the request to the local unix socket of the remote system.
func serviceTokensPost(s state.State, r *http.Request) response.Response {
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

	sh, err := service.NewHandler(s.Name(), req.ClusterAddress, s.FileSystem().StateDir, types.ServiceType(serviceType))
	if err != nil {
		return response.SmartError(err)
	}

	token, err := sh.Services[types.ServiceType(serviceType)].IssueToken(r.Context(), req.JoinerName)
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to issue %s token for peer %q: %w", serviceType, req.JoinerName, err))
	}

	return response.SyncResponse(true, token)
}
