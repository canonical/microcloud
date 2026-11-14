package environment

import (
	"github.com/canonical/lxd/shared/api"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework/lxd"
)

// EnsureProject creates or deletes the named project, depending on create. Both directions are
// idempotent: creating an already-existing project, or deleting a nonexistent one, is not an
// error.
func (p *Provisioner) EnsureProject(projectName string, create bool) error {
	if create {
		return lxd.CreateProject(p.conn, projectName, api.ProjectPut{
			Config: map[string]string{
				"features.images":          "true",
				"features.profiles":        "true",
				"features.storage.volumes": "true",
				"features.storage.buckets": "true",
			},
		})
	}

	return lxd.DeleteProject(p.conn, projectName)
}
