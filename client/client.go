package client

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/canonical/lxd/shared/api"
	microTypes "github.com/canonical/microcluster/v3/microcluster/types"
	"github.com/gorilla/websocket"

	"github.com/canonical/microcloud/microcloud/api/types"
)

// GetStatus fetches a set of status information for the whole cluster.
func GetStatus(ctx context.Context, c microTypes.Client) ([]types.Status, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var statuses []types.Status
	err := c.Query(queryCtx, "GET", types.APIVersion, &api.NewURL().Path("status").URL, nil, &statuses)
	if err != nil {
		return nil, err
	}

	return statuses, nil
}

// DeleteToken allows deleting a token using the given client.
// We don't use Microcluster's own token deletion func from the app to allow this func being called both on
// the local Microcluster member as well as a remote member.
// It's using Microcluster's stable API.
func DeleteToken(ctx context.Context, name string, c microTypes.Client) error {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	return c.Query(queryCtx, "DELETE", microTypes.PublicEndpoint, &api.NewURL().Path("tokens", name).URL, nil, nil)
}

// GetClusterMembers returns a list of cluster members.
// We don't use Microcluster's own cluster member list func from the app to allow this func being called both on
// the local Microcluster member as well as a remote member.
// It's using Microcluster's stable API.
func GetClusterMembers(ctx context.Context, c microTypes.Client) ([]microTypes.ClusterMember, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var clusterMembers []microTypes.ClusterMember
	err := c.Query(queryCtx, "GET", microTypes.PublicEndpoint, &api.NewURL().Path("cluster").URL, nil, &clusterMembers)
	if err != nil {
		return nil, err
	}

	return clusterMembers, nil
}

// StartSession starts a new session and returns the underlying websocket connection.
func StartSession(ctx context.Context, c microTypes.Client, role string, sessionTimeout time.Duration) (*websocket.Conn, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	url := api.NewURL().Path("session", role).WithQuery("timeout", sessionTimeout.String())
	conn, err := c.Websocket(queryCtx, types.APIVersion, &url.URL)
	if err != nil {
		return nil, fmt.Errorf("Failed to start session websocket: %w", err)
	}

	return conn, nil
}

// StopSession is called from the initiator to stop a joiner session.
func StopSession(ctx context.Context, c microTypes.Client, stopMsg string) error {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	data := types.SessionStopPut{Reason: stopMsg}
	err := c.Query(queryCtx, "PUT", types.APIVersion, &api.NewURL().Path("session", "stop").URL, data, nil)
	if err != nil {
		return fmt.Errorf("Failed to stop joiner session: %w", err)
	}

	return nil
}

// JoinServices sends join information to initiate the cluster join process.
func JoinServices(ctx context.Context, c microTypes.Client, data types.ServicesPut) error {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	err := c.Query(queryCtx, "PUT", types.APIVersion, &api.NewURL().Path("services").URL, data, nil)
	if err != nil {
		return fmt.Errorf("Failed to update cluster status of services: %w", err)
	}

	return nil
}

// JoinIntent sends the join intent to a potential cluster.
func JoinIntent(ctx context.Context, c microTypes.Client, data types.SessionJoinPost) (*x509.Certificate, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// The join intent request is using HMAC authorization.
	// Therefore we have to marshal the data ourselves as the JSON encoder used
	// by the query functions is appending a newline at the end.
	// See https://pkg.go.dev/encoding/json#Encoder.Encode.
	// This newline will cause the HMAC verification to fail on the server side
	// as the server will recreate the HMAC based on the request body.
	// The JSON marshaller doesn't add a newline.
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal join intent: %w", err)
	}

	path := api.NewURL().Path("session", "join")

	// We can pass a reader to indicate to the query functions the body is already marshalled.
	resp, err := c.QueryRaw(queryCtx, "POST", types.APIVersion, &path.URL, bytes.NewBuffer(dataBytes))
	if err != nil {
		return nil, fmt.Errorf("Failed to send join intent: %w", err)
	}

	// Parse the response to check for errors.
	_, err = microTypes.ParseResponse(resp)
	if err != nil {
		return nil, err
	}

	if len(resp.TLS.PeerCertificates) == 0 {
		return nil, errors.New("Peer's certificate is missing")
	}

	return resp.TLS.PeerCertificates[0], nil
}

// RemoteIssueToken issues a token on the remote MicroCloud.
func RemoteIssueToken(ctx context.Context, c microTypes.Client, serviceType types.ServiceType, data types.ServiceTokensPost) (string, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var token string
	err := c.Query(queryCtx, "POST", types.APIVersion, &api.NewURL().Path("services", string(serviceType), "tokens").URL, data, &token)
	if err != nil {
		return "", fmt.Errorf("Failed to issue remote token: %w", err)
	}

	return token, nil
}

// DeleteClusterMember removes the cluster member from any service that it is part of.
func DeleteClusterMember(ctx context.Context, c microTypes.Client, memberName string, force bool) error {
	queryCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	path := api.NewURL().Path("services", "cluster", memberName)
	if force {
		path = path.WithQuery("force", "1")
	}

	return c.Query(queryCtx, "DELETE", types.APIVersion, &path.URL, nil, nil)
}
