package client

import (
	"context"
	"fmt"
	"time"

	"github.com/canonical/microcluster/client"
	"github.com/lxc/lxd/shared/api"

	"github.com/canonical/microcloud/microcloud/api/types"
)

// JoinServices sends join information to initiate the cluster join process.
func JoinServices(ctx context.Context, c *client.Client, data types.ServicesPut) error {
	queryCtx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	err := c.Query(queryCtx, "PUT", api.NewURL().Path("services"), data, nil)
	if err != nil {
		return fmt.Errorf("Failed to update cluster status of services: %w", err)
	}

	return nil
}
