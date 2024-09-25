package client

import (
	"context"
	"fmt"
	"time"

	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/microcluster/v2/client"

	"github.com/canonical/microcloud/microcloud/api/types"
)

// JoinServices sends join information to initiate the cluster join process.
func JoinServices(ctx context.Context, c *client.Client, data types.ServicesPut) error {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	err := c.Query(queryCtx, "PUT", types.APIVersion, api.NewURL().Path("services"), data, nil)
	if err != nil {
		return fmt.Errorf("Failed to update cluster status of services: %w", err)
	}

	return nil
}

// RemoteIssueToken issues a token on the remote MicroCloud, trusted by the mDNS auth secret.
func RemoteIssueToken(ctx context.Context, c *client.Client, serviceType types.ServiceType, data types.ServiceTokensPost) (string, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var token string
	err := c.Query(queryCtx, "POST", types.APIVersion, api.NewURL().Path("services", string(serviceType), "tokens"), data, &token)
	if err != nil {
		return "", fmt.Errorf("Failed to issue remote token: %w", err)
	}

	return token, nil
}

// DeleteClusterMember removes the cluster member from any service that it is part of.
func DeleteClusterMember(ctx context.Context, c *client.Client, memberName string, force bool) error {
	queryCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	path := api.NewURL().Path("services", "cluster", memberName)
	if force {
		path = path.WithQuery("force", "1")
	}

	return c.Query(queryCtx, "DELETE", types.APIVersion, path, nil, nil)
}
