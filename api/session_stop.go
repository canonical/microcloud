package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/microcluster/v2/rest"
	"github.com/canonical/microcluster/v2/state"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/service"
)

// SessionStopCmd represents the /1.0/session/stop API on MicroCloud.
var SessionStopCmd = func(sh *service.Handler) rest.Endpoint {
	return rest.Endpoint{
		AllowedBeforeInit: true,
		Name:              "session/stop",
		Path:              "session/stop",

		Put: rest.EndpointAction{Handler: authHandlerMTLS(sh, sessionStopPut(sh))},
	}
}

// sessionStopPut stops the current session.
func sessionStopPut(sh *service.Handler) func(state state.State, r *http.Request) response.Response {
	return func(state state.State, r *http.Request) response.Response {
		req := types.SessionStopPut{}

		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			return response.BadRequest(err)
		}

		err = sh.SessionTransaction(true, func(session *service.Session) error {
			err := session.Stop(errors.New(req.Reason))
			if err != nil {
				return api.StatusErrorf(http.StatusBadRequest, "Failed to stop session: %w", err)
			}

			return nil
		})
		if err != nil {
			return response.SmartError(err)
		}

		return response.EmptySyncResponse
	}
}
