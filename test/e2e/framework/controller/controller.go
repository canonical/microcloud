package controller

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/onsi/ginkgo/v2"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework/testbed"
)

const (
	// rebootPollInterval is the polling interval used while waiting for a member to come back up
	// after a reboot.
	rebootPollInterval = time.Second

	// readinessWaitTimeout bounds how long waitReady should wait for.
	readinessWaitTimeout = 90 * time.Second

	// DefaultRebootTimeout bounds how long Reboot waits for a cluster member to become reachable
	// and ready again after being restarted.
	DefaultRebootTimeout = 15 * time.Minute
)

// Controller abstracts how to reach and control a MicroCloud deployment's cluster members'
// underlying hosts. LXDController and SSHController are its two implementations, selected by New
// based on which section (lxd or ssh) a testbed file configures.
type Controller interface {
	// Exec runs argv on the named cluster member and returns its output. A non-zero exit status is
	// not an error: it is returned as exitCode along with the captured output. err is only set
	// when the command could not be started or its result could not be determined.
	Exec(member string, argv []string, env map[string]string) (stdout string, stderr string, exitCode int, err error)

	// Reboot restarts the named cluster member and blocks until it is reachable and ready again,
	// or DefaultRebootTimeout elapses.
	Reboot(member string) error

	// GetMembers returns the cluster member names this Controller is configured to control (the
	// local LXD project's instance names for an LXDController, or the testbed file's ssh.nodes
	// keys for an SSHController).
	GetMembers() ([]string, error)
}

// New returns the Controller to use for the current deployment, based on which section (lxd or
// ssh) cfg configures:
//
//  1. cfg.LXD set: an LXDController.
//  2. cfg.SSH set: an SSHController.
//  3. neither set: an error -- there is no way to reach this deployment's members without a valid
//     testbed file.
//
// Each implementation validates that it can actually control the configured members (e.g. the
// local LXD daemon is reachable, or every configured ssh node can be dialed) before returning.
func New(cfg testbed.Config) (Controller, error) {
	switch {
	case cfg.LXD != nil:
		ginkgo.GinkgoWriter.Println("Using lxd testbed config")

		lc, err := newLXDController(cfg)
		if err != nil {
			return nil, err
		}

		return lc, nil

	case cfg.SSH != nil:
		ginkgo.GinkgoWriter.Println("Using ssh testbed config")

		sc, err := newSSHController(cfg.SSH.Nodes)
		if err != nil {
			return nil, err
		}

		return sc, nil

	default:
		return nil, errors.New("Cannot determine how to control this MicroCloud deployment's members: no valid lxd/ssh testbed config 'E2E_TESTBED_FILE'")
	}
}

// pollUntil calls condition every rebootPollInterval until it returns true, or returns an error
// wrapping timeoutMsg once deadline passes.
func pollUntil(deadline time.Time, timeoutMsg string, condition func() bool) error {
	for {
		if condition() {
			return nil
		}

		if time.Now().After(deadline) {
			return errors.New(timeoutMsg)
		}

		time.Sleep(rebootPollInterval)
	}
}

// waitReady waits for member to become reachable again, then best-effort waits for systemd and
// MicroCloud's own readiness signals to settle -- tolerating their own errors/timeouts, since a
// reboot is still considered successful even if they don't fully settle within budget. sudo is
// prefixed for controllers that don't already run as root (e.g. SSHController).
func waitReady(c Controller, member string, deadline time.Time, sudo bool) error {
	err := pollUntil(deadline, fmt.Sprintf("Timed out waiting for %q to become reachable after reboot", member), func() bool {
		_, _, rc, err := c.Exec(member, []string{"true"}, nil)
		return err == nil && rc == 0
	})
	if err != nil {
		return err
	}

	var prefix []string
	if sudo {
		prefix = []string{"sudo"}
	}

	timeoutSeconds := strconv.Itoa(int(readinessWaitTimeout.Seconds()))

	argv := append(prefix, "timeout", timeoutSeconds, "systemctl", "is-system-running", "--wait", "--quiet")

	stdout, stderr, rc, execErr := c.Exec(member, argv, nil)
	if execErr != nil || rc != 0 {
		ginkgo.GinkgoWriter.Printf("waitReady: %q: systemctl is-system-running: rc=%d err=%v stdout=%s stderr=%s\n", member, rc, execErr, stdout, stderr)
	}

	time.Sleep(5 * time.Second)

	argv = append(prefix, "microcloud", "waitready", "--timeout="+timeoutSeconds)

	stdout, stderr, rc, execErr = c.Exec(member, argv, map[string]string{"TEST_CONSOLE": "0"})
	if execErr != nil || rc != 0 {
		ginkgo.GinkgoWriter.Printf("waitReady: %q: microcloud waitready: rc=%d err=%v stdout=%s stderr=%s\n", member, rc, execErr, stdout, stderr)
	}

	return nil
}
