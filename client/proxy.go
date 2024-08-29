package client

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/canonical/lxd/shared"
	"github.com/canonical/microcluster/v2/client"

	"github.com/canonical/microcloud/microcloud/api/types"
)

// UseAuthProxy takes the given microcluster client and secret and proxies requests to other services through the MicroCloud API.
// The secret will be set in the authentication header in lieu of TLS authentication, if present.
func UseAuthProxy(c *client.Client, secret string, serviceType types.ServiceType) (*client.Client, error) {
	tp, ok := c.Transport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("Invalid client transport type")
	}

	// If the client is a unix client, it may not have any TLS config.
	if tp.TLSClientConfig == nil {
		tp.TLSClientConfig = &tls.Config{}
	}

	// Only set InsecureSkipVerify if the secret is non-empty, so we will fallback to regular TLS authentication.
	if secret != "" {
		tp.TLSClientConfig.InsecureSkipVerify = true
	}

	tp.Proxy = AuthProxy(secret, serviceType)

	c.Transport = tp

	return c, nil
}

// AuthProxy takes a request to a service and sends it to MicroCloud instead,
// to be then forwarded to the unix socket of the corresponding service.
// The secret is set in the request header to use in lieu of TLS authentication.
func AuthProxy(secret string, serviceType types.ServiceType) func(r *http.Request) (*url.URL, error) {
	return func(r *http.Request) (*url.URL, error) {
		r.Header.Set("X-MicroCloud-Auth", secret)

		// MicroCloud itself doesn't need to use the proxy other than to set the auth secret.
		if serviceType != types.MicroCloud {
			path := fmt.Sprintf("/1.0/services/%s", strings.ToLower(string(serviceType)))
			if !strings.HasPrefix(r.URL.Path, path) {
				r.URL.Path = path + r.URL.Path
			}
		}

		return shared.ProxyFromEnvironment(r)
	}
}
