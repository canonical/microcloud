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
	"github.com/canonical/microcloud/microcloud/component"
)

// ComponentsTokensCmd represents the /1.0/components/componentType/tokens API on MicroCloud.
var ComponentsTokensCmd = func(sh *component.Handler) rest.Endpoint {
	return rest.Endpoint{
		AllowedBeforeInit: true,
		Name:              "components/{componentType}/tokens",
		Path:              "components/{componentType}/tokens",

		Post: rest.EndpointAction{Handler: authHandlerMTLS(sh, componentTokensPost), ProxyTarget: true},
	}
}

// componentTokensPost issues a token for component using the MicroCloud proxy.
// Normally a token request to a component is restricted to trusted systems,
// so this endpoint makes use of the estblished mTLS and then proxies the request to the local unix socket of the remote system.
func componentTokensPost(s state.State, r *http.Request) response.Response {
	componentType, err := url.PathUnescape(mux.Vars(r)["componentType"])
	if err != nil {
		return response.SmartError(err)
	}

	// Parse the request.
	req := types.ComponentTokensPost{}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return response.BadRequest(err)
	}

	sh, err := component.NewHandler(s.Name(), req.ClusterAddress, s.FileSystem().StateDir, types.ComponentType(componentType))
	if err != nil {
		return response.SmartError(err)
	}

	token, err := sh.Components[types.ComponentType(componentType)].IssueToken(r.Context(), req.JoinerName)
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to issue %s token for peer %q: %w", componentType, req.JoinerName, err))
	}

	return response.SyncResponse(true, token)
}
