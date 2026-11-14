package lxd

import (
	"fmt"

	"github.com/canonical/lxd/shared/api"
)

// EvacuateMember evacuates member (migrating/stopping its instances elsewhere) on the MicroCloud
// deployment reached via conn, waiting for the operation to complete. This is a cluster-level LXD
// API operation, independent of which Controller (LXDController/SSHController) is used to reach
// the member's underlying host.
func EvacuateMember(conn *Connection, member string) error {
	op, err := conn.Server().UpdateClusterMemberState(member, api.ClusterMemberStatePost{
		Action: api.ClusterMemberActionEvacuate,
		Force:  true,
	})
	if err != nil {
		return fmt.Errorf("Cannot evacuate cluster member %q: %w", member, err)
	}

	err = op.Wait()
	if err != nil {
		return fmt.Errorf("Cannot evacuate cluster member %q: %w", member, err)
	}

	return nil
}
