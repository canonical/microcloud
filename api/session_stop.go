package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/canonical/lxd/shared/api"
	microTypes "github.com/canonical/microcluster/v3/microcluster/types"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/service"
)

// SessionStopCmd represents the /1.0/session/stop API on MicroCloud.
var SessionStopCmd = func(sh *service.Handler) microTypes.Endpoint {
	return microTypes.Endpoint{
		AllowedBeforeInit: true,
		Name:              "session/stop",
		Path:              "session/stop",

		Put: microTypes.EndpointAction{Handler: authHandlerMTLS(sh, sessionStopPut(sh))},
	}
}

// sessionStopPut stops the current session.
func sessionStopPut(sh *service.Handler) func(state microTypes.State, r *http.Request) microTypes.Response {
	return func(state microTypes.State, r *http.Request) microTypes.Response {
		req := types.SessionStopPut{}

		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			return microTypes.BadRequest(err)
		}

		err = sh.SessionTransaction(true, func(session *service.Session) error {
			err := session.Stop(errors.New(req.Reason))
			if err != nil {
				return api.StatusErrorf(http.StatusBadRequest, "Failed to stop session: %w", err)
			}

			return nil
		})
		if err != nil {
			return microTypes.SmartError(err)
		}

		return microTypes.EmptySyncResponse
	}
}
