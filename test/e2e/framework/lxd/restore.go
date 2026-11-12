package lxd

import (
	"fmt"

	"github.com/canonical/lxd/shared/api"
)

// RestoreMember restores member (migrating its instances back) on the MicroCloud deployment
// reached via conn, waiting for the operation to complete.
func RestoreMember(conn *Connection, member string) error {
	op, err := conn.Server().UpdateClusterMemberState(member, api.ClusterMemberStatePost{
		Action: api.ClusterMemberActionRestore,
		Force:  true,
	})
	if err != nil {
		return fmt.Errorf("Cannot restore cluster member %q: %w", member, err)
	}

	err = op.Wait()
	if err != nil {
		return fmt.Errorf("Cannot restore cluster member %q: %w", member, err)
	}

	return nil
}
