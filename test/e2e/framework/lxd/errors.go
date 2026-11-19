package lxd

import (
	"strings"

	"github.com/canonical/lxd/shared/api"
)

// isAlreadyExists reports whether err looks like an LXD "already exists" error, so create
// operations can be idempotent.
func isAlreadyExists(err error) bool {
	if err == nil {
		return false
	}

	return api.StatusErrorCheck(err, 409) || strings.Contains(strings.ToLower(err.Error()), "already exists")
}
