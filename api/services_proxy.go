package api

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/canonical/lxd/client"
	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/shared/ws"
	"github.com/canonical/microcluster/v2/microcluster"
	"github.com/canonical/microcluster/v2/rest"
	"github.com/canonical/microcluster/v2/state"

	"github.com/canonical/microcloud/microcloud/service"
)

// LXDProxy proxies all requests from MicroCloud to LXD.
func LXDProxy(sh *service.Handler) rest.Endpoint {
	return proxy(sh, "lxd", "services/lxd/{rest:.*}", lxdHandler)
}

// CephProxy proxies all requests from MicroCloud to MicroCeph.
func CephProxy(sh *service.Handler) rest.Endpoint {
	return proxy(sh, "microceph", "services/microceph/{rest:.*}", microHandler("microceph", MicroCephDir))
}

// OVNProxy proxies all requests from MicroCloud to MicroOVN.
func OVNProxy(sh *service.Handler) rest.Endpoint {
	return proxy(sh, "microovn", "services/microovn/{rest:.*}", microHandler("microovn", MicroOVNDir))
}

// LXDDir is the path to the state directory of the LXD snap.
const LXDDir = "/var/snap/lxd/common/lxd"

// MicroCephDir is the path to the state directory of the MicroCeph snap.
const MicroCephDir = "/var/snap/microceph/common/state"

// MicroOVNDir is the path to the state directory of the MicroOVN snap.
const MicroOVNDir = "/var/snap/microovn/common/state"

// proxy returns a proxy endpoint with the given handler and access applied to all REST methods.
func proxy(sh *service.Handler, name, path string, handler endpointHandler) rest.Endpoint {
	return rest.Endpoint{
		AllowedBeforeInit: true,
		Name:              name,
		Path:              path,

		Get:    rest.EndpointAction{Handler: authHandlerMTLS(sh, handler), ProxyTarget: true},
		Put:    rest.EndpointAction{Handler: authHandlerMTLS(sh, handler), ProxyTarget: true},
		Post:   rest.EndpointAction{Handler: authHandlerMTLS(sh, handler), ProxyTarget: true},
		Patch:  rest.EndpointAction{Handler: authHandlerMTLS(sh, handler), ProxyTarget: true},
		Delete: rest.EndpointAction{Handler: authHandlerMTLS(sh, handler), ProxyTarget: true},
	}
}

// lxdHandler forwards a request made to /1.0/services/lxd/<rest> to /1.0/<rest> on the LXD unix socket.
func lxdHandler(s state.State, r *http.Request) response.Response {
	_, path, ok := strings.Cut(r.URL.Path, "/1.0/services/lxd")
	if !ok {
		return response.SmartError(fmt.Errorf("Invalid path %q", r.URL.Path))
	}

	if r.Header.Get("Upgrade") == "websocket" {
		client, err := lxd.ConnectLXDUnix(filepath.Join(LXDDir, "unix.socket"), nil)
		if err != nil {
			return response.SmartError(fmt.Errorf("Failed to connect to local LXD: %w", err))
		}

		// RawWebsocket assigns /1.0, so remove it here.
		_, path, _ = strings.Cut(path, "/1.0")
		sock, err := client.RawWebsocket(path)
		if err != nil {
			return response.SmartError(err)
		}

		// Perform the websocket proxy.
		return response.ManualResponse(func(w http.ResponseWriter) error {
			conn, err := ws.Upgrader.Upgrade(w, r, nil)
			if err != nil {
				return err
			}

			defer conn.Close()

			<-ws.Proxy(sock, conn)

			return nil
		})
	}

	// Must unset the RequestURI. It is an error to set this in a client request.
	r.RequestURI = ""
	r.URL.Path = path
	r.URL.Scheme = "http"
	r.URL.Host = "unix.socket"
	r.Host = r.URL.Host
	client, err := lxd.ConnectLXDUnix(filepath.Join(LXDDir, "unix.socket"), nil)
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to connect to local LXD: %w", err))
	}

	resp, err := client.DoHTTP(r)
	if err != nil {
		return response.SmartError(err)
	}

	return NewResponse(resp)
}

// microHandler forwards a request made to /1.0/services/<microcluster-service>/<rest> to /1.0/<rest> on the service unix socket.
func microHandler(service string, stateDir string) func(state.State, *http.Request) response.Response {
	return func(s state.State, r *http.Request) response.Response {
		_, path, ok := strings.Cut(r.URL.Path, fmt.Sprintf("/1.0/services/%s", service))
		if !ok {
			return response.SmartError(fmt.Errorf("Invalid path %q", r.URL.Path))
		}

		// Must unset the RequestURI. It is an error to set this in a client request.
		r.RequestURI = ""
		r.URL.Path = path
		r.URL.Scheme = "http"
		r.URL.Host = "control.socket"
		r.Host = r.URL.Host
		client, err := microcluster.App(microcluster.Args{StateDir: stateDir})
		if err != nil {
			return response.SmartError(err)
		}

		c, err := client.LocalClient()
		if err != nil {
			return response.SmartError(err)
		}

		resp, err := c.Do(r)
		if err != nil {
			return response.SmartError(err)
		}

		return NewResponse(resp)
	}
}
