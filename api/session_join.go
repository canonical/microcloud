package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/microcluster/v2/rest"
	"github.com/canonical/microcluster/v2/state"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/service"
)

// SessionJoinCmd represents the /1.0/session/join API on MicroCloud.
var SessionJoinCmd = func(sh *service.Handler) rest.Endpoint {
	return rest.Endpoint{
		AllowedBeforeInit: true,
		Name:              "session/join",
		Path:              "session/join",

		Post: rest.EndpointAction{Handler: authHandlerHMAC(sh, sessionJoinPost(sh)), AllowUntrusted: true},
	}
}

// sessionJoinPost receives join intent requests from new potential members.
func sessionJoinPost(sh *service.Handler) func(state state.State, r *http.Request) response.Response {
	return func(state state.State, r *http.Request) response.Response {
		// Apply delay right at the beginning before doing any validation.
		// This limits the number of join attempts that can be made by an attacker.
		select {
		case <-time.After(100 * time.Millisecond):
		case <-r.Context().Done():
			return response.InternalError(errors.New("Request cancelled"))
		}

		// Parse the request.
		req := types.SessionJoinPost{}

		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			return response.BadRequest(err)
		}

		err = sh.SessionTransaction(true, func(session *service.Session) error {
			// Only validate the intent (services) on the initiator.
			// The joiner has to accept the services from the initiator.
			if session.Role() == types.SessionInitiating {
				err = validateIntent(sh, req)
				if err != nil {
					return api.NewStatusError(http.StatusBadRequest, err.Error())
				}
			}

			fingerprint, err := shared.CertFingerprintStr(req.Certificate)
			if err != nil {
				return api.StatusErrorf(http.StatusBadRequest, "Failed to get fingerprint: %w", err)
			}

			err = session.RegisterIntent(fingerprint)
			if err != nil {
				return api.StatusErrorf(http.StatusBadRequest, "Failed to register join intent: %w", err)
			}

			// Prevent locking in case there isn't anymore an active consumer reading on the channel.
			// This can happen if the initiator's websocket connection isn't anymore active.
			select {
			case session.IntentCh() <- req:
				return nil
			default:
				return fmt.Errorf("No active consumer for join intent")
			}
		})

		return response.SmartError(err)
	}
}

// validateIntent validates the given join intent.
// It checks whether or not the peer is missing any of our services and returns an error if one is missing.
func validateIntent(sh *service.Handler, intent types.SessionJoinPost) error {
	// Reject any peers that are missing our services.
	for service := range sh.Services {
		if !shared.ValueInSlice(service, intent.Services) {
			return fmt.Errorf("Rejecting peer %q due to missing services (%s)", intent.Name, string(service))
		}
	}

	return nil
}
