package lxd

import (
	"fmt"
	"path/filepath"
	"time"

	lxdclient "github.com/canonical/lxd/client"
)

const (
	// DefaultRetryInterval is the wait time between retries of flaky operations (instance readiness
	// polling, etc).
	DefaultRetryInterval = 5 * time.Second

	// DefaultEventuallyTimeout bounds how long specs wait for eventually- consistent conditions (e.g.
	// an instance acquiring an IPv4 address after being evacuated/restored) before failing.
	DefaultEventuallyTimeout = 30 * time.Second
)

// connectNamedRemote connects to remoteName (as configured via `lxc remote add`, or the special
// "local" remote), scoped to project.
func connectNamedRemote(remoteName, project string) (lxdclient.InstanceServer, error) {
	configDir, err := lxcConfigDir()
	if err != nil {
		return nil, err
	}

	conf, err := loadLXCConfig(configDir)
	if err != nil {
		return nil, err
	}

	s, err := conf.GetInstanceServer(remoteName)
	if err != nil {
		return nil, fmt.Errorf("Cannot connect to remote %q (using LXD client config from %q): %w", remoteName, filepath.Join(configDir, "config.yml"), err)
	}

	return s.UseProject(project), nil
}

// Connection is a connection to a MicroCloud deployment's (or a single local LXD daemon's) LXD
// API, scoped to the project passed to Connect.
type Connection struct {
	server  lxdclient.InstanceServer
	project string
}

// Connect connects to remoteName (as configured via `lxc remote add`, or the special "local"
// remote -- the LXD daemon running on this test host, reached over its unix socket), scoped to
// projectName.
func Connect(remoteName string, projectName string) (*Connection, error) {
	s, err := connectNamedRemote(remoteName, projectName)
	if err != nil {
		return nil, err
	}

	return &Connection{server: s, project: projectName}, nil
}

// Server returns the connection's underlying LXD server.
func (c *Connection) Server() lxdclient.InstanceServer {
	return c.server
}

// Project returns the project this connection is scoped to.
func (c *Connection) Project() string {
	return c.project
}

// GetClusterMembers returns the names of every member of the MicroCloud cluster this connection
// is connected to, as reported by its own clustering API. This is the authoritative source of
// which members really belong to the cluster, unlike a Controller's GetMembers (testbed config or
// a local LXD project's instance list, which is not guaranteed to match 1:1 with real cluster
// membership -- e.g. a stray/orphaned instance in that project).
func (c *Connection) GetClusterMembers() ([]string, error) {
	members, err := c.server.GetClusterMembers()
	if err != nil {
		return nil, fmt.Errorf("Cannot list cluster members: %w", err)
	}

	names := make([]string, 0, len(members))
	for _, m := range members {
		names = append(names, m.ServerName)
	}

	return names, nil
}
