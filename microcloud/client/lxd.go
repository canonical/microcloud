package client

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/canonical/microcluster/client"
	"github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared"
)

// NewLXDClient returns a LXD client with the underlying Client.
func NewLXDClient(path string, c *client.Client) (lxd.InstanceServer, error) {
	client, err := lxd.ConnectLXDUnix(path, &lxd.ConnectionArgs{
		HTTPClient: c.Client.Client,
		Proxy: func(r *http.Request) (*url.URL, error) {
			if !strings.HasPrefix(r.URL.Path, "/1.0/services/lxd") {
				r.URL.Path = "/1.0/services/lxd" + r.URL.Path
			}

			return shared.ProxyFromEnvironment(r)
		},
	})

	if err != nil {
		return nil, err
	}

	return client, nil
}
