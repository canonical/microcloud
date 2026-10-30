package framework

import (
	"fmt"
	"time"

	"github.com/canonical/lxd/shared/api"
)

// LXDController controls cluster members that are themselves local LXD
// instancesvia the local LXD API.
type LXDController struct{}

// newLXDController returns an LXDController.
func newLXDController() (*LXDController, error) {
	_, err := LocalServer()
	if err != nil {
		return nil, fmt.Errorf("Cannot control cluster members via local LXD: %w", err)
	}

	return &LXDController{}, nil
}

// Exec runs argv inside the local LXD instance named member.
func (c *LXDController) Exec(member string, argv []string, env map[string]string) (stdout string, stderr string, exitCode int, err error) {
	s, err := LocalServer()
	if err != nil {
		return "", "", -1, err
	}

	return execInInstanceOn(s, member, argv, env)
}

// Reboot restarts the local LXD instance named member and waits for it to be
// ready.
func (c *LXDController) Reboot(member string) error {
	s, err := LocalServer()
	if err != nil {
		return err
	}

	op, err := s.UpdateInstanceState(member, api.InstanceStatePut{Action: "restart", Force: true}, "")
	if err != nil {
		return fmt.Errorf("Cannot restart local LXD instance %q: %w", member, err)
	}

	err = op.Wait()
	if err != nil {
		return fmt.Errorf("Cannot restart local LXD instance %q: %w", member, err)
	}

	deadline := time.Now().Add(DefaultRebootTimeout)

	err = pollUntil(deadline, fmt.Sprintf("Timed out waiting for %q to boot (no running processes reported)", member), func() bool {
		state, _, err := s.GetInstanceState(member)
		return err == nil && state.Processes >= 1
	})
	if err != nil {
		return err
	}

	// ExecInstance already runs as root inside the instance, so no sudo
	// prefix is needed for the readiness commands.
	return waitReady(c, member, deadline, false)
}
