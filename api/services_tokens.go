package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	microTypes "github.com/canonical/microcluster/v3/microcluster/types"
	"github.com/gorilla/mux"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/service"
)

// ServiceTokensCmd represents the /1.0/services/serviceType/tokens API on MicroCloud.
var ServiceTokensCmd = func(sh *service.Handler) microTypes.Endpoint {
	return microTypes.Endpoint{
		AllowedBeforeInit: true,
		Name:              "services/{serviceType}/tokens",
		Path:              "services/{serviceType}/tokens",

		Post: microTypes.EndpointAction{Handler: authHandlerMTLS(sh, serviceTokensPost), ProxyTarget: true},
	}
}

// serviceTokensPost issues a token for service using the MicroCloud proxy.
// Normally a token request to a service is restricted to trusted systems,
// so this endpoint makes use of the estblished mTLS and then proxies the request to the local unix socket of the remote system.
func serviceTokensPost(s microTypes.State, r *http.Request) microTypes.Response {
	serviceType, err := url.PathUnescape(mux.Vars(r)["serviceType"])
	if err != nil {
		return microTypes.SmartError(err)
	}

	// Parse the request.
	req := types.ServiceTokensPost{}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return microTypes.BadRequest(err)
	}

	sh, err := service.NewHandler(s.Name(), req.ClusterAddress, s.FileSystem().StateDir(), types.ServiceType(serviceType))
	if err != nil {
		return microTypes.SmartError(err)
	}

	token, err := sh.Services[types.ServiceType(serviceType)].IssueToken(r.Context(), req.JoinerName)
	if err != nil {
		return microTypes.SmartError(fmt.Errorf("Failed to issue %s token for peer %q: %w", serviceType, req.JoinerName, err))
	}

	return microTypes.SyncResponse(true, token)
}
