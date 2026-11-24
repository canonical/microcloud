package environment

import (
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/lxd"
)

// DeleteAllInstances deletes every instance in the "e2e-testing" project, unconditionally. This is
// a teardown safety net: scenarios are expected to clean up their own instances via DeferCleanup,
// but a spec that crashed/panicked mid-run could leave some behind, which would otherwise make
// EnsureProject(projectName, false) fail since the project would still contain entities.
func (p *Provisioner) DeleteAllInstances(projectName string) error {
	names, err := lxd.GetInstanceNames(p.conn)
	if err != nil {
		return err
	}

	for _, name := range names {
		err := lxd.DeleteInstance(p.conn, name)
		if err != nil {
			return err
		}
	}

	return nil
}
