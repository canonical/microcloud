package controller

import (
	"fmt"
	"time"

	lxdclient "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework/lxd"
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/testbed"
)

// LXDController controls cluster members that are themselves local LXD instances, via the local
// LXD API.
type LXDController struct {
	local lxdclient.InstanceServer
}

// newLXDController returns an LXDController connected to the local LXD daemon, scoped to
// cfg.LXD.Project, failing fast if the daemon isn't actually reachable.
func newLXDController(cfg testbed.Config) (*LXDController, error) {
	conn, err := lxd.Connect("local", cfg.LXD.Project)
	if err != nil {
		return nil, fmt.Errorf("Cannot control cluster members via local LXD: %w", err)
	}

	c := &LXDController{local: conn.Server()}

	_, err = c.GetMembers()
	if err != nil {
		return nil, fmt.Errorf("Cannot control cluster members via local LXD: %w", err)
	}

	return c, nil
}

// Exec runs argv inside the local LXD instance named member.
func (c *LXDController) Exec(member string, argv []string, env map[string]string, timeout time.Duration) (stdout string, stderr string, exitCode int, err error) {
	return lxd.ExecInInstanceOn(c.local, member, argv, env, timeout)
}

// GetMembers returns the names of every local LXD instance in the project this LXDController is
// scoped to.
func (c *LXDController) GetMembers() ([]string, error) {
	names, err := c.local.GetInstanceNames(api.InstanceTypeAny)
	if err != nil {
		return nil, fmt.Errorf("Cannot list local LXD instances: %w", err)
	}

	return names, nil
}
