package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/lxd/shared/trust"
	"github.com/canonical/microcluster/v2/state"

	"github.com/canonical/microcloud/microcloud/service"
)

// endpointHandler is just a convenience for writing clean return types.
type endpointHandler func(state.State, *http.Request) response.Response

// authHandlerMTLS ensures a request has been authenticated using mTLS.
func authHandlerMTLS(sh *service.Handler, f endpointHandler) endpointHandler {
	return func(s state.State, r *http.Request) response.Response {
		if r.RemoteAddr == "@" {
			logger.Debug("Allowing unauthenticated request through unix socket")

			return f(s, r)
		}

		// Use certificate based authentication between cluster members.
		if r.TLS != nil {
			trustedCerts := s.Remotes().CertificatesNative()
			for _, cert := range r.TLS.PeerCertificates {
				// First evaluate the permanent turst store.
				trusted, _ := util.CheckMutualTLS(*cert, trustedCerts)
				if trusted {
					return f(s, r)
				}

				// Second evaluate the temporary trust store.
				// This is the fallback during the forming of the cluster.
				trusted, _ = util.CheckMutualTLS(*cert, sh.TemporaryTrustStore())
				if trusted {
					return f(s, r)
				}
			}
		}

		return response.Forbidden(fmt.Errorf("Failed to authenticate using mTLS"))
	}
}

// authHandlerHMAC ensures a request has been authenticated using the HMAC in the Authorization header.
func authHandlerHMAC(sh *service.Handler, f endpointHandler) endpointHandler {
	return func(s state.State, r *http.Request) response.Response {
		sessionFunc := func(session *service.Session) error {
			h, err := trust.NewHMACArgon2([]byte(session.Passphrase()), nil, trust.NewDefaultHMACConf(HMACMicroCloud10))
			if err != nil {
				return err
			}

			err = trust.HMACEqual(h, r)
			if err != nil {
				attemptErr := session.RegisterFailedAttempt()
				if attemptErr != nil {
					errorCause := errors.New("Stopping session after too many failed attempts")

					// Immediately stop the session to not allow further join attempts.
					stopErr := session.Stop(errorCause)
					if stopErr != nil {
						return fmt.Errorf("Cannot stop session after too many failed attempts: %w", stopErr)
					}

					// Log the error and return it to the caller
					logger.Warn(errorCause.Error())
					return errorCause
				}

				return err
			}

			return nil
		}

		// Run a r/w transaction against the session as we might stop it due to too many failed attempts.
		err := sh.SessionTransaction(false, sessionFunc)
		if err != nil {
			return response.SmartError(err)
		}

		return f(s, r)
	}
}
