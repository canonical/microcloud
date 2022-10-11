// Package client provides a full Go API client.
package client

import (
	"context"
	"fmt"
	"time"

	"github.com/lxc/lxd/shared/api"

	"github.com/canonical/microcluster/client"
	"github.com/canonical/microcloud/microcloud/api/types"
)

// ExtendedPostCmd is a client function that sets a context timeout and sends a POST to /1.0/extended using the given
// client. This function is expected to be called from an api endpoint handler, which gives us access to the
// daemon state, from which we can create a client.
func ExtendedPostCmd(ctx context.Context, c *client.Client, data *types.ExtendedType) (string, error) {
	queryCtx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	var outStr string
	err := c.Query(queryCtx, "POST", api.NewURL().Path("extended"), data, &outStr)
	if err != nil {
		clientURL := c.URL()
		return "", fmt.Errorf("Failed performing action on %q: %w", clientURL.String(), err)
	}

	return outStr, nil
}
