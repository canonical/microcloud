// Package environment bootstraps and tears down the "e2e-testing" LXD
// project, profile, and base images shared by every scenario in this suite.
package environment

import (
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/lxd"
)

// Provisioner creates and tears down the "e2e-testing" project, profile, and base images on a
// Connection.
type Provisioner struct {
	conn *lxd.Connection
}

// New returns a Provisioner operating on conn.
func New(conn *lxd.Connection) *Provisioner {
	return &Provisioner{conn: conn}
}

// SetupEnvironment creates the project, profile, and base images needed for the suite, returning
// the base image fingerprints. The counterpart to TeardownEnvironment.
func (p *Provisioner) SetupEnvironment(projectName, profileName string) (containerFingerprint, vmFingerprint string, err error) {
	err = p.EnsureProject(projectName, true)
	if err != nil {
		return "", "", err
	}

	err = p.EnsureProfile(profileName, true)
	if err != nil {
		return "", "", err
	}

	return p.EnsureBaseImages(projectName)
}

// TeardownEnvironment deletes every instance, image, the profile, and the project, in that
// order, unconditionally. The counterpart to SetupEnvironment.
func (p *Provisioner) TeardownEnvironment(projectName, profileName string) error {
	err := p.DeleteAllInstances(projectName)
	if err != nil {
		return err
	}

	err = p.DeleteAllImages(projectName)
	if err != nil {
		return err
	}

	err = p.EnsureProfile(profileName, false)
	if err != nil {
		return err
	}

	return p.EnsureProject(projectName, false)
}
