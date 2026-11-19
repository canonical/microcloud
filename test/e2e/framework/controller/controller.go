package controller

import (
	"errors"
	"time"

	"github.com/onsi/ginkgo/v2"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework/testbed"
)

// DefaultCommandTimeout bounds how long a single Exec call may block before being cancelled and a
// timeout error is returned.
const DefaultCommandTimeout = 3 * time.Minute

// Controller abstracts how to reach and control a MicroCloud deployment's cluster members'
// underlying hosts. LXDController and SSHController are its two implementations, selected by New
// based on which section (lxd or ssh) a testbed file configures.
type Controller interface {
	// Exec runs argv on the named cluster member and returns its output. A non-zero exit status is
	// not an error: it is returned as exitCode along with the captured output. err is only set
	// when the command could not be started or its result could not be determined, including when
	// it does not complete within timeout.
	Exec(member string, argv []string, env map[string]string, timeout time.Duration) (stdout string, stderr string, exitCode int, err error)

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
		return nil, errors.New("Cannot determine how to control this MicroCloud deployment's members: No valid lxd/ssh testbed config 'E2E_TESTBED_FILE'")
	}
}
