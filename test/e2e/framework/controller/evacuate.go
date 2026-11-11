package controller

import (
	"fmt"

	"github.com/canonical/lxd/shared/api"
)

// EvacuateMember evacuates member (migrating/stopping its instances elsewhere), waiting for the
// operation to complete.
func (c *Controller) EvacuateMember(member string) error {
	op, err := c.conn.Server().UpdateClusterMemberState(member, api.ClusterMemberStatePost{
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

// RestoreMember restores member (migrating its instances back), waiting for the operation to
// complete. A single attempt is made; see this suite's evacuation scenario for the retry loop
// wrapping known-transient errors right after a member comes back online.
func (c *Controller) RestoreMember(member string) error {
	op, err := c.conn.Server().UpdateClusterMemberState(member, api.ClusterMemberStatePost{
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
