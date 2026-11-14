package lxd

import (
	"fmt"

	"github.com/canonical/lxd/shared/api"
)

// CreateProject creates a project on conn. It is idempotent: an "already exists" error from the
// server is not treated as a failure.
func CreateProject(conn *Connection, name string, put api.ProjectPut) error {
	err := conn.Server().CreateProject(api.ProjectsPost{Name: name, ProjectPut: put})
	if err != nil && !isAlreadyExists(err) {
		return fmt.Errorf("Cannot create project %q: %w", name, err)
	}

	return nil
}

// DeleteProject deletes the named project on conn. It is safe to call unconditionally: a "not
// found" error is not treated as a failure.
func DeleteProject(conn *Connection, name string) error {
	op, err := conn.Server().DeleteProject(name, false)
	if err == nil {
		err = op.Wait()
	}

	if err != nil && !api.StatusErrorCheck(err, 404) {
		return fmt.Errorf("Cannot delete project %q: %w", name, err)
	}

	return nil
}
