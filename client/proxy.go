package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/canonical/lxd/shared"
	"github.com/canonical/microcluster/v2/client"

	"github.com/canonical/microcloud/microcloud/api/types"
)

// AuthConfig is used to configure the various authentication settings during trust establishment.
// In case of unverified mTLS, InsecureSkipVerify has to be set to true.
// In case of partially verified mTLS, the remote servers certificate can be set using TLSServerCertificate.
// Request authentication can be made by setting a valid HMAC.
type AuthConfig struct {
	HMAC                 string
	TLSServerCertificate *x509.Certificate
	InsecureSkipVerify   bool
}

// UseAuthProxy takes the given microcluster client and HMAC and proxies requests to other components through the MicroCloud API.
// The HMAC will be set in the Authorization header in lieu of mTLS authentication, if present.
// If no HMAC is present mTLS is assumed.
func UseAuthProxy(c *client.Client, componentType types.ComponentType, conf AuthConfig) (*client.Client, error) {
	tp, ok := c.Transport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("Invalid client transport type")
	}

	// If the client is a unix client, it may not have any TLS config.
	if tp.TLSClientConfig == nil {
		tp.TLSClientConfig = &tls.Config{}
	}

	tp.TLSClientConfig.InsecureSkipVerify = conf.InsecureSkipVerify
	tp.Proxy = AuthProxy(conf.HMAC, componentType)

	c.Transport = tp

	return c, nil
}

// AuthProxy takes a request to a component and sends it to MicroCloud instead,
// to be then forwarded to the unix socket of the corresponding component.
// The HMAC is set in the request header to be used partially in lieu of mTLS authentication.
func AuthProxy(hmac string, componentType types.ComponentType) func(r *http.Request) (*url.URL, error) {
	return func(r *http.Request) (*url.URL, error) {
		if hmac != "" {
			r.Header.Set("Authorization", hmac)
		}

		// MicroCloud itself doesn't need to use the proxy.
		if componentType != types.MicroCloud {
			path := fmt.Sprintf("/1.0/components/%s", strings.ToLower(string(componentType)))
			if !strings.HasPrefix(r.URL.Path, path) {
				r.URL.Path = path + r.URL.Path
			}
		}

		return shared.ProxyFromEnvironment(r)
	}
}
