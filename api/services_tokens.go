package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/microcluster/rest"
	"github.com/canonical/microcluster/state"
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

		Post: rest.EndpointAction{Handler: authHandler(sh, serviceTokensPost), AllowUntrusted: true, ProxyTarget: true},
	}
}

func IsSafeVarPath(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	varDir := os.Getenv("LXD_DIR")
	if varDir == "" {
		varDir = "/var/lib/lxd"
	}

	if !strings.HasPrefix(absPath, varDir) {
		return errors.New("Absolute path is outside the default LXD path")
	}

	return nil
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

	err = IsSafeVarPath(req.JoinerName)
	if err != nil {
		return response.SmartError(err)
	}

	_ = os.MkdirAll(req.JoinerName, 0700)

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
