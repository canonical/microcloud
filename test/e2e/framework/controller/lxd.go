package controller

import (
	"fmt"
	"time"

	lxdclient "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"

	lxdframework "github.com/canonical/microcloud/microcloud/test/e2e/framework/lxd"
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
	conn, err := lxdframework.Connect("local", cfg.LXD.Project)
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
func (c *LXDController) Exec(member string, argv []string, env map[string]string) (stdout string, stderr string, exitCode int, err error) {
	return lxdframework.ExecInInstanceOn(c.local, member, argv, env)
}

// Reboot restarts the local LXD instance named member and waits for it to be ready.
func (c *LXDController) Reboot(member string) error {
	op, err := c.local.UpdateInstanceState(member, api.InstanceStatePut{Action: "restart", Force: true}, "")
	if err != nil {
		return fmt.Errorf("Cannot restart local LXD instance %q: %w", member, err)
	}

	err = op.Wait()
	if err != nil {
		return fmt.Errorf("Cannot restart local LXD instance %q: %w", member, err)
	}

	deadline := time.Now().Add(DefaultRebootTimeout)

	err = pollUntil(deadline, fmt.Sprintf("Timed out waiting for %q to boot (no running processes reported)", member), func() bool {
		state, _, err := c.local.GetInstanceState(member)
		return err == nil && state.Processes >= 1
	})
	if err != nil {
		return err
	}

	// ExecInInstanceOn already runs as root inside the instance, so no sudo prefix is needed for
	// the readiness commands.
	return waitReady(c, member, deadline, false)
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
