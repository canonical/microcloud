package framework

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	//nolint:staticcheck // Ginkgo's DSL is conventionally dot-imported.
	. "github.com/onsi/ginkgo/v2"
)

// DefaultRebootTimeout bounds how long Reboot waits for a cluster member to
// become reachable and ready again after being restarted.
const DefaultRebootTimeout = 15 * time.Minute

// rebootPollInterval is the polling interval used while waiting for a member
// to come back up after a reboot.
const rebootPollInterval = time.Second

// readinessWaitTimeout bounds how long waitReady should waits for.
const readinessWaitTimeout = 90 * time.Second

// NodeController abstracts how to reach and control a cluster member's
// underlying host.
type NodeController interface {
	// Exec runs argv on the named cluster member and returns its output.
	// A non-zero exit status is not an error: it is returned as exitCode
	// along with the captured output. err is only set when the command
	// could not be started or its result could not be determined.
	Exec(member string, argv []string, env map[string]string) (stdout string, stderr string, exitCode int, err error)

	// Reboot restarts the named cluster member and blocks until it is
	// reachable and ready again, or DefaultRebootTimeout elapses.
	Reboot(member string) error
}

// Environment identifies which NodeController mode is in effect for the
// current deployment.
type Environment string

const (
	// EnvLXD means the testbed file's lxd section was confirmed valid
	// (see validateLocalLXDProject): LXDController is used.
	EnvLXD Environment = "lxd"

	// EnvSSH means the testbed file's ssh section was confirmed valid
	// (see validateSSHNodes): SSHController is used.
	EnvSSH Environment = "ssh"

	// EnvUnknown means neither is validly configured (no testbed file,
	// neither section present, or an invalid lxd/ssh section):
	// resolveController errors out.
	EnvUnknown Environment = "unknown"
)

// detectedEnv is the Environment computed once by prepareNodeControl and
// cached for the remainder of the suite run.
var detectedEnv Environment = EnvUnknown

// Env returns the Environment computed by prepareNodeControl. EnvUnknown
// until then (or until SetEnv has run).
func Env() Environment {
	return detectedEnv
}

// SetEnv overrides the cached Environment. Normally called from
// prepareNodeControl; exposed for custom bootstraps.
func SetEnv(env Environment) {
	detectedEnv = env
}

// controller caches the NodeController resolved by resolveController,
// avoiding re-resolving (and for LXDController, reconnecting) on every
// call.
var controller NodeController

// resolveController returns the NodeController to use for the current
// deployment, based on the Environment computed by prepareNodeControl:
//
//  1. EnvLXD: an LXDController.
//  2. EnvSSH: an SSHController.
//  3. EnvUnknown: an error -- there is no way to reach this deployment's
//     members without a valid testbed file.
//
// The result is cached after the first successful resolution.
func resolveController() (NodeController, error) {
	if controller != nil {
		return controller, nil
	}

	switch Env() {
	case EnvLXD:
		c, err := newLXDController()
		if err != nil {
			return nil, err
		}

		controller = c

		return controller, nil

	case EnvSSH:
		controller = newSSHController(Testbed().SSH.Nodes)
		return controller, nil

	default:
		return nil, errors.New("Cannot determine how to control this MicroCloud deployment's members: " +
			"no valid lxd/ssh testbed config (see E2E_TESTBED_FILE); provide a valid lxd or ssh section (see README.md)")
	}
}

// pollUntil calls condition every rebootPollInterval until it returns
// true, or returns an error wrapping timeoutMsg once deadline passes.
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

// waitReady waits for member to become reachable again, then best-effort
// waits for systemd and MicroCloud's own readiness signals to settle --
// tolerating their own errors/timeouts, since a reboot is still considered
// successful even if they don't fully settle within budget. sudo is
// prefixed for controllers that don't already run as root (e.g.
// SSHController).
func waitReady(c NodeController, member string, deadline time.Time, sudo bool) error {
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

	stdout, stderr, rc, execErr := c.Exec(member, append(prefix, "timeout", timeoutSeconds, "systemctl", "is-system-running", "--wait", "--quiet"), nil)
	if execErr != nil || rc != 0 {
		GinkgoWriter.Printf("waitReady: %q: systemctl is-system-running: rc=%d err=%v stdout=%s stderr=%s\n", member, rc, execErr, stdout, stderr)
	}

	time.Sleep(5 * time.Second)

	stdout, stderr, rc, execErr = c.Exec(member, append(prefix, "microcloud", "waitready", "--timeout="+timeoutSeconds), map[string]string{"TEST_CONSOLE": "0"})
	if execErr != nil || rc != 0 {
		GinkgoWriter.Printf("waitReady: %q: microcloud waitready: rc=%d err=%v stdout=%s stderr=%s\n", member, rc, execErr, stdout, stderr)
	}

	return nil
}
