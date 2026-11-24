package environment

import (
	"github.com/canonical/lxd/shared/api"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework/lxd"
)

// EnsureProfile creates or deletes the e2e-testing profile (a root disk on the "remote" pool and
// an eth0 NIC on the "default" network), depending on create. Both directions are idempotent:
// creating an already-existing profile, or deleting a nonexistent one, is not an error. Mirrors
// the profile the suite's former Terraform-based setup used to create.
func (p *Provisioner) EnsureProfile(profileName string, create bool) error {
	if create {
		return lxd.CreateProfile(p.conn, profileName, api.ProfilePut{
			Config: map[string]string{
				"limits.cpu":    "1",
				"limits.memory": "384MiB",
			},
			Devices: map[string]map[string]string{
				"root": {
					"type": "disk",
					"pool": "remote",
					"path": "/",
					"size": "4GiB",
				},
				"eth0": {
					"type":    "nic",
					"network": "default",
				},
			},
		})
	}

	return lxd.DeleteProfile(p.conn, profileName)
}
