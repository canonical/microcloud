package api

import (
	"context"
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
	"github.com/canonical/microcloud/microcloud/component"
)

// SessionJoinCmd represents the /1.0/session/join API on MicroCloud.
var SessionJoinCmd = func(sh *component.Handler) rest.Endpoint {
	return rest.Endpoint{
		AllowedBeforeInit: true,
		Name:              "session/join",
		Path:              "session/join",

		Post: rest.EndpointAction{Handler: authHandlerHMAC(sh, sessionJoinPost(sh)), AllowUntrusted: true},
	}
}

// sessionJoinPost receives join intent requests from new potential members.
func sessionJoinPost(sh *component.Handler) func(state state.State, r *http.Request) response.Response {
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

		err = sh.SessionTransaction(true, func(session *component.Session) error {
			// Only validate the intent (components) on the initiator.
			// The joiner has to accept the components from the initiator.
			if session.Role() == types.SessionInitiating {
				err = validateIntent(r.Context(), sh, req)
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
			// Wait up to 10 seconds for an active consumer.
			// When the initiator returns the dice-generated passphrase, a joiner can go ahead and send
			// its intent to join. If the initiator hasn't yet started to listen on join intents (too slow),
			// the API might return an error as there isn't yet any active consumer.
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()

			select {
			case session.IntentCh() <- req:
				return nil
			case <-ctx.Done():
				return fmt.Errorf("Timeout waiting for an active consumer of the join intent")
			}
		})

		return response.SmartError(err)
	}
}

// validateIntent validates the given join intent.
// It checks whether or not the peer is missing any of our components and returns an error if one is missing.
// Also compares each component's daemon version between the joiner and initiator.
func validateIntent(ctx context.Context, sh *component.Handler, intent types.SessionJoinPost) error {
	// Reject any peers that are missing our components.
	for _, component := range sh.Components {
		intentVersion, ok := intent.Components[component.Type()]
		if !ok {
			return fmt.Errorf("Rejecting peer %q due to missing components (%s)", intent.Name, string(component.Type()))
		}

		version, err := component.GetVersion(ctx)
		if err != nil {
			return fmt.Errorf("Unable to determine initiator's %s version: %w", component.Type(), err)
		}

		if intentVersion != version {
			return fmt.Errorf("Rejecting peer %q due to invalid %s version. (Want: %q, Detected: %q)", intent.Name, component.Type(), version, intentVersion)
		}
	}

	return nil
}
