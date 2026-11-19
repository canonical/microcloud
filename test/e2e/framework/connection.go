package framework

import (
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/lxd"
)

// remoteConnection is the cached *lxd.Connection returned by GetRemoteConnection.
var remoteConnection *lxd.Connection

// GetRemoteConnection returns the shared *lxd.Connection to Remote(), scoped to ProjectName,
// connecting lazily on first call and reusing the same connection on every subsequent call.
func GetRemoteConnection() (*lxd.Connection, error) {
	if remoteConnection != nil {
		return remoteConnection, nil
	}

	conn, err := lxd.Connect(Remote(), ProjectName)
	if err != nil {
		return nil, err
	}

	remoteConnection = conn

	return remoteConnection, nil
}
