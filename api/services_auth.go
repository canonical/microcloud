package api

import (
	"fmt"
	"net/http"

	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/microcluster/v2/state"

	"github.com/canonical/microcloud/microcloud/service"
)

// endpointHandler is just a convenience for writing clean return types.
type endpointHandler func(state.State, *http.Request) response.Response

// authHandler ensures a request has been authenticated with the mDNS broadcast secret.
func authHandler(sh *service.Handler, f endpointHandler) endpointHandler {
	return func(s state.State, r *http.Request) response.Response {
		if r.RemoteAddr == "@" {
			logger.Debug("Allowing unauthenticated request through unix socket")

			return f(s, r)
		}

		// Use certificate based authentication between cluster members.
		if r.TLS != nil && r.Host == s.Address().URL.Host {
			trustedCerts := s.Remotes().CertificatesNative()
			for _, cert := range r.TLS.PeerCertificates {
				trusted, _ := util.CheckMutualTLS(*cert, trustedCerts)
				if trusted {
					return f(s, r)
				}
			}
		}

		secret := r.Header.Get("X-MicroCloud-Auth")
		if secret == "" {
			return response.BadRequest(fmt.Errorf("No auth secret in response"))
		}

		if sh.AuthSecret == "" {
			return response.BadRequest(fmt.Errorf("No generated auth secret"))
		}

		if sh.AuthSecret != secret {
			return response.SmartError(fmt.Errorf("Request secret does not match, ignoring request"))
		}

		return f(s, r)
	}
}
