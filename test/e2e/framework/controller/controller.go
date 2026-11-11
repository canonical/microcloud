package controller

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	//nolint:staticcheck // Ginkgo's DSL is conventionally dot-imported.
	. "github.com/onsi/ginkgo/v2"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework/lxd"
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/testbed"
)

const (
	// DefaultRebootTimeout bounds how long Reboot waits for a cluster member to become reachable
	// and ready again after being restarted.
	DefaultRebootTimeout = 15 * time.Minute

	// rebootPollInterval is the polling interval used while waiting for a member to come back up
	// after a reboot.
	rebootPollInterval = time.Second

	// readinessWaitTimeout bounds how long waitReady should wait for.
	readinessWaitTimeout = 90 * time.Second

	// EnvLXD means the testbed file's lxd section was confirmed valid
	// (see Controller.validateLocalLXDProject): LXDController is used.
	EnvLXD Environment = "lxd"

	// EnvSSH means the testbed file's ssh section was confirmed valid
	// (see Controller.validateSSHNodes): SSHController is used.
	EnvSSH Environment = "ssh"

	// EnvUnknown means neither is validly configured (no testbed file,
	// neither section present, or an invalid lxd/ssh section): Resolve
	// errors out.
	EnvUnknown Environment = "unknown"
)

// NodeController abstracts how to reach and control a cluster member's underlying host.
type NodeController interface {
	// Exec runs argv on the named cluster member and returns its output. A non-zero exit status is
	// not an error: it is returned as exitCode along with the captured output. err is only set
	// when the command could not be started or its result could not be determined.
	Exec(member string, argv []string, env map[string]string) (stdout string, stderr string, exitCode int, err error)

	// Reboot restarts the named cluster member and blocks until it is reachable and ready again,
	// or DefaultRebootTimeout elapses.
	Reboot(member string) error
}

// Environment identifies which NodeController mode is in effect for the current deployment.
type Environment string

// Controller resolves and caches the NodeController to use for the current MicroCloud deployment,
// and runs cluster health checks/reboots through it.
//
// A Controller is constructed once per Ginkgo process by framework.SetupProcess
// (test/e2e/framework/framework.go) and exposed to scenario packages via DefaultController.
type Controller struct {
	conn    *lxd.Connection
	cfg     testbed.Config
	env     Environment
	members []string

	resolved NodeController
}

// DefaultController is the Controller constructed by framework.SetupProcess, for scenario packages
// to use. It is nil until SetupProcess (run via SynchronizedBeforeSuite) assigns it.
var DefaultController *Controller

// New returns a Controller operating on conn, using cfg to resolve which NodeController
// implementation applies.
func New(conn *lxd.Connection, cfg testbed.Config) *Controller {
	return &Controller{conn: conn, cfg: cfg, env: EnvUnknown}
}

// Env returns the Environment detected by Prepare.
func (c *Controller) Env() Environment {
	return c.env
}

// MemberNames returns the cluster member names cached by Prepare.
func (c *Controller) MemberNames() []string {
	return c.members
}

// Prepare validates whichever testbed section is configured (lxd or ssh) against names (the
// current cluster member names), setting Env() accordingly. names is cached for later use by
// CheckClusterHealth.
func (c *Controller) Prepare(names []string) {
	c.members = names

	switch {
	case c.cfg.LXD != nil:
		env := EnvUnknown
		if c.validateLocalLXDProject(names) {
			env = EnvLXD
		}

		c.env = env
		By(fmt.Sprintf("Using lxd testbed config (environment: %s)", env))

	case c.cfg.SSH != nil:
		env := EnvUnknown
		if c.validateSSHNodes(c.cfg.SSH.Nodes) {
			env = EnvSSH
		}

		c.env = env
		By(fmt.Sprintf("Using ssh testbed config (environment: %s)", env))

	default:
		c.env = EnvUnknown
	}
}

// Resolve returns the NodeController to use for the current deployment,
// based on the Environment set by Prepare:
//
//  1. EnvLXD: an LXDController.
//  2. EnvSSH: an SSHController.
//  3. EnvUnknown: an error -- there is no way to reach this deployment's
//     members without a valid testbed file.
//
// The result is cached after the first successful resolution.
func (c *Controller) Resolve() (NodeController, error) {
	if c.resolved != nil {
		return c.resolved, nil
	}

	switch c.env {
	case EnvLXD:
		lc, err := newLXDController(c.cfg)
		if err != nil {
			return nil, err
		}

		c.resolved = lc

		return c.resolved, nil

	case EnvSSH:
		c.resolved = newSSHController(c.cfg.SSH.Nodes)
		return c.resolved, nil

	default:
		return nil, errors.New("Cannot determine how to control this MicroCloud deployment's members: " +
			"no valid lxd/ssh testbed config (see E2E_TESTBED_FILE); provide a valid lxd or ssh section (see README.md)")
	}
}

// RebootMember reboots member using the resolved NodeController.
func (c *Controller) RebootMember(member string) error {
	nc, err := c.Resolve()
	if err != nil {
		return err
	}

	return nc.Reboot(member)
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

	argv := append(prefix, "timeout", timeoutSeconds, "systemctl", "is-system-running", "--wait", "--quiet")

	stdout, stderr, rc, execErr := c.Exec(member, argv, nil)
	if execErr != nil || rc != 0 {
		GinkgoWriter.Printf("waitReady: %q: systemctl is-system-running: rc=%d err=%v stdout=%s stderr=%s\n", member, rc, execErr, stdout, stderr)
	}

	time.Sleep(5 * time.Second)

	argv = append(prefix, "microcloud", "waitready", "--timeout="+timeoutSeconds)

	stdout, stderr, rc, execErr = c.Exec(member, argv, map[string]string{"TEST_CONSOLE": "0"})
	if execErr != nil || rc != 0 {
		GinkgoWriter.Printf("waitReady: %q: microcloud waitready: rc=%d err=%v stdout=%s stderr=%s\n", member, rc, execErr, stdout, stderr)
	}

	return nil
}
