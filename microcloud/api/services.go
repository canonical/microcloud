package api

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/canonical/microcluster/microcluster"
	"github.com/canonical/microcluster/rest"
	"github.com/canonical/microcluster/state"
	"github.com/lxc/lxd/client"
	"github.com/lxc/lxd/lxd/response"
	"github.com/lxc/lxd/shared"
)

// LXDProxy proxies all requests from MicroCloud to LXD.
var LXDProxy = Proxy("lxd", "services/lxd/{rest:.*}", lxdHandler)

// CephProxy proxies all requests from MicroCloud to MicroCeph.
var CephProxy = Proxy("microceph", "services/microceph/{rest:.*}", cephHandler)

// LXDDir is the path to the state directory of the LXD snap.
const LXDDir = "/var/snap/lxd/common/lxd"

// MicroCephDir is the path to the state directory of the MicroCeph snap.
const MicroCephDir = "/var/snap/microceph/common/state"

// Proxy returns a proxy endpoint with the given handler and access applied to all REST methods.
func Proxy(name, path string, handler func(*state.State, *http.Request) response.Response) rest.Endpoint {
	return rest.Endpoint{
		AllowedBeforeInit: true,
		Name:              name,
		Path:              path,

		Get:    rest.EndpointAction{Handler: handler, AllowUntrusted: true, ProxyTarget: true},
		Put:    rest.EndpointAction{Handler: handler, AllowUntrusted: true, ProxyTarget: true},
		Post:   rest.EndpointAction{Handler: handler, AllowUntrusted: true, ProxyTarget: true},
		Patch:  rest.EndpointAction{Handler: handler, AllowUntrusted: true, ProxyTarget: true},
		Delete: rest.EndpointAction{Handler: handler, AllowUntrusted: true, ProxyTarget: true},
	}
}

// lxdHandler forwards a request made to /1.0/services/lxd/<rest> to /1.0/<rest> on the LXD unix socket.
func lxdHandler(s *state.State, r *http.Request) response.Response {
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
		ws, err := client.RawWebsocket(path)
		if err != nil {
			return response.SmartError(err)
		}

		// Perform the websocket proxy.
		return response.ManualResponse(func(w http.ResponseWriter) error {
			conn, err := shared.WebsocketUpgrader.Upgrade(w, r, nil)
			if err != nil {
				return err
			}

			defer conn.Close()

			<-shared.WebsocketProxy(ws, conn)

			return nil
		})
	}

	// Must unset the RequestURI. It is an error to set this in a client request.
	r.RequestURI = ""
	r.URL.Path = path
	r.URL.Scheme = "http"
	r.URL.Host = filepath.Join(LXDDir, "unix.socket")
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

// cephHandler forwards a request made to /1.0/services/ceph/<rest> to /1.0/<rest> on the MicroCeph unix socket.
func cephHandler(s *state.State, r *http.Request) response.Response {
	_, path, ok := strings.Cut(r.URL.Path, "/1.0/services/microceph")
	if !ok {
		return response.SmartError(fmt.Errorf("Invalid path %q", r.URL.Path))
	}

	// Must unset the RequestURI. It is an error to set this in a client request.
	r.RequestURI = ""
	r.URL.Path = path
	r.URL.Scheme = "http"
	r.URL.Host = filepath.Join(MicroCephDir, "control.socket")
	r.Host = r.URL.Host
	client, err := microcluster.App(s.Context, microcluster.Args{StateDir: MicroCephDir})
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
