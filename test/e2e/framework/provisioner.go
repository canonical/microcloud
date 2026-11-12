package framework

import (
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/environment"
)

// provisioner is the cached environment.Provisioner returned by GetProvisioner.
var provisioner *environment.Provisioner

// GetProvisioner returns the shared environment.Provisioner for GetRemoteConnection()'s
// connection, constructing it lazily on first call and reusing it on every subsequent call.
func GetProvisioner() (*environment.Provisioner, error) {
	if provisioner != nil {
		return provisioner, nil
	}

	conn, err := GetRemoteConnection()
	if err != nil {
		return nil, err
	}

	provisioner = environment.New(conn)

	return provisioner, nil
}
