package lxd

import (
	"fmt"

	"github.com/canonical/lxd/shared/api"
)

// CreateProfile creates a profile on conn. It is idempotent: an "already exists" error from the
// server is not treated as a failure.
func CreateProfile(conn *Connection, name string, put api.ProfilePut) error {
	err := conn.Server().CreateProfile(api.ProfilesPost{Name: name, ProfilePut: put})
	if err != nil && !isAlreadyExists(err) {
		return fmt.Errorf("Cannot create profile %q: %w", name, err)
	}

	return nil
}

// DeleteProfile deletes the named profile on conn. It is safe to call unconditionally: a "not
// found" error is not treated as a failure.
func DeleteProfile(conn *Connection, name string) error {
	err := conn.Server().DeleteProfile(name)
	if err != nil && !api.StatusErrorCheck(err, 404) {
		return fmt.Errorf("Cannot delete profile %q: %w", name, err)
	}

	return nil
}
