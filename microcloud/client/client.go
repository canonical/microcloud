package client

import (
	"context"
	"fmt"
	"time"

	"github.com/canonical/microcluster/client"
	"github.com/lxc/lxd/shared/api"
)

// WipeDisk wipes the local disk with the given device ID.
func WipeDisk(ctx context.Context, c *client.Client, deviceID string) error {
	queryCtx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	err := c.Query(queryCtx, "PUT", api.NewURL().Path("services", "disks", deviceID), nil, nil)
	if err != nil {
		return fmt.Errorf("Failed to wipe disk: %w", err)
	}

	return nil
}
