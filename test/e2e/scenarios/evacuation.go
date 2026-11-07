package scenarios

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/canonical/lxd/shared/api"
	//nolint:staticcheck // Ginkgo/Gomega's DSL is conventionally dot-imported.
	. "github.com/onsi/ginkgo/v2"
	//nolint:staticcheck // Ginkgo/Gomega's DSL is conventionally dot-imported.
	. "github.com/onsi/gomega"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework"
)

// restoreMemberWithRetry issues the Restore action for member, retrying on
// known-transient errors:
//
//   - "Target cluster member is offline": LXD's own clustering layer only
//     considers a member "online" once a heartbeat has landed within
//     cluster.offline_threshold (20s by default) of "now"; right after the
//     member comes back up (which only waits for the member itself --
//     systemd + microcloud -- to be locally ready), that heartbeat may not
//     have landed yet, so the migration the Restore action triggers can
//     transiently fail with this error.
//   - "signal: alarm clock" from ovn-nbctl while adding the member back to
//     an OVS chassis group: OVN's own clustered southbound DB can still be
//     re-electing a raft leader / settling right after the member rejoins,
//     independent of how "ready" the member itself reports being, so the
//     fixed ovn-nbctl --timeout can be hit even once the member is
//     otherwise fully online.
//
// Any other error is not retried.
func restoreMemberWithRetry(member string) {
	const offlineErr = "Target cluster member is offline"
	const ovnTimeoutErr = "signal: alarm clock"

	Eventually(func(g Gomega) {
		op, err := framework.Server().UpdateClusterMemberState(member, api.ClusterMemberStatePost{
			Action: api.ClusterMemberActionRestore,
			Force:  true,
		})
		if err == nil {
			err = op.Wait()
		}

		if err != nil && !strings.Contains(err.Error(), offlineErr) && !strings.Contains(err.Error(), ovnTimeoutErr) {
			StopTrying(fmt.Sprintf("Restoring %q failed with a non-retryable error", member)).Wrap(err).Now()
		}

		g.Expect(err).NotTo(HaveOccurred())
	}).WithTimeout(framework.DefaultEventuallyTimeout).WithPolling(time.Second).Should(Succeed())
}

// evacuateAndRestoreMember evacuates, reboots, and restores member, then waits
// for its instances to come back online.
func evacuateAndRestoreMember(member string) {
	By("Evacuating " + member)
	op, err := framework.Server().UpdateClusterMemberState(member, api.ClusterMemberStatePost{
		Action: api.ClusterMemberActionEvacuate,
		Force:  true,
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(op.Wait()).To(Succeed())

	By("Rebooting " + member)
	Expect(framework.RebootMember(member)).To(Succeed())

	By("Restoring " + member)
	restoreMemberWithRetry(member)

	By(fmt.Sprintf("Waiting for instances on %s to come back online", member))
	instances, err := framework.GetInstanceNamesByMember(member)
	Expect(err).NotTo(HaveOccurred())

	for _, instance := range instances {
		Eventually(func(g Gomega) {
			// The Restore action's own instance restart is best-effort and
			// can silently leave an instance stopped if it raced with the
			// member only just having come back online (see
			// restoreMemberWithRetry); nudge it to start on every poll.
			g.Expect(framework.EnsureInstanceRunning(instance)).To(Succeed())

			ipv4, err := framework.GetInstanceIPv4(instance)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(ipv4).NotTo(BeEmpty())
		}).WithTimeout(framework.DefaultEventuallyTimeout).WithPolling(time.Second).Should(Succeed())
	}
}

// evacuationCounts reads E2E_EVACUATION_COUNTS (default 1), the number of full
// evacuate/restore cycles to run across every cluster member.
func evacuationCounts() int {
	raw := os.Getenv("E2E_EVACUATION_COUNTS")
	if raw == "" {
		return 1
	}

	n, err := strconv.Atoi(raw)
	Expect(err).NotTo(HaveOccurred(), "Invalid E2E_EVACUATION_COUNTS value: %q", raw)

	return n
}

var _ = Describe("Evacuation", Label("risk-medium", "builtin"), func() {
	It("evacuates, reboots, and restores every cluster member", func() {
		counts := evacuationCounts()
		if counts == 0 {
			Skip("Evacuation test skipped (E2E_EVACUATION_COUNTS=0)")
		}

		for i := 1; i <= counts; i++ {
			By(fmt.Sprintf("Evacuation cycle %d/%d", i, counts))

			members, err := framework.Server().GetClusterMembers()
			Expect(err).NotTo(HaveOccurred())

			for _, m := range members {
				evacuateAndRestoreMember(m.ServerName)
				checkAllConnectivity()
			}
		}
	})
})
