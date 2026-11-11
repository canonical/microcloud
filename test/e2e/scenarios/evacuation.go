package scenarios

import (
	"fmt"
	"strings"
	"time"

	//nolint:staticcheck // Ginkgo/Gomega's DSL is conventionally dot-imported.
	. "github.com/onsi/ginkgo/v2"
	//nolint:staticcheck // Ginkgo/Gomega's DSL is conventionally dot-imported.
	. "github.com/onsi/gomega"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework"
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/controller"
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/environment"
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/instance"
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/lxd"
)

var _ = Describe("Evacuation", Label("risk-medium", "builtin"), Ordered, func() {
	// fleet holds the names of the instances deployed for this spec group,
	// declared here (tree-construction time) but only populated in
	// BeforeAll (run time), independent of connectivity.go's own fleet.
	var fleet []string

	BeforeAll(func() {
		containerFingerprint, vmFingerprint, err := environment.DefaultBootstrapper.EnsureBaseImages(framework.ProjectName)
		Expect(err).NotTo(HaveOccurred())

		By("Deploying a fleet across every cluster member")
		fleet, err = instance.DefaultManager.LaunchMixedFleet("e2e-evacuation-", containerFingerprint, vmFingerprint, framework.ProfileName)
		Expect(err).NotTo(HaveOccurred())

		for _, name := range fleet {
			DeferCleanup(instance.DefaultManager.DeleteInstance, name)
		}
	})

	When("each cluster member in turn is evacuated, rebooted, and restored", func() {
		It("every member's instances come back online and the fleet remains fully reachable", func() {
			for _, member := range controller.DefaultController.MemberNames() {
				By("Evacuating " + member)
				Expect(controller.DefaultController.EvacuateMember(member)).To(Succeed())

				By("Rebooting " + member)
				Expect(controller.DefaultController.RebootMember(member)).To(Succeed())

				By("Restoring " + member)
				// Retries on known-transient errors:
				//
				//   - "Target cluster member is offline": LXD's own
				//     clustering layer only considers a member "online"
				//     once a heartbeat has landed within
				//     cluster.offline_threshold (20s by default) of "now";
				//     right after the member comes back up (which only
				//     waits for the member itself -- systemd + microcloud
				//     -- to be locally ready), that heartbeat may not have
				//     landed yet, so the migration the Restore action
				//     triggers can transiently fail with this error.
				//   - "signal: alarm clock" from ovn-nbctl while adding
				//     the member back to an OVS chassis group: OVN's own
				//     clustered southbound DB can still be re-electing a
				//     raft leader / settling right after the member
				//     rejoins, independent of how "ready" the member
				//     itself reports being, so the fixed ovn-nbctl
				//     --timeout can be hit even once the member is
				//     otherwise fully online.
				//
				// Any other error is not retried.
				Eventually(func(g Gomega) {
					err := controller.DefaultController.RestoreMember(member)

					const offlineErr = "Target cluster member is offline"
					const ovnTimeoutErr = "signal: alarm clock"

					nonRetryable := err != nil && !strings.Contains(err.Error(), offlineErr) && !strings.Contains(err.Error(), ovnTimeoutErr)
					if nonRetryable {
						StopTrying(fmt.Sprintf("Restoring %q failed with a non-retryable error", member)).Wrap(err).Now()
					}

					g.Expect(err).NotTo(HaveOccurred())
				}).WithTimeout(lxd.DefaultEventuallyTimeout).WithPolling(time.Second).Should(Succeed())

				By(fmt.Sprintf("Waiting for instances on %s to come back online", member))
				instances, err := instance.DefaultManager.GetInstanceNamesByMember(member)
				Expect(err).NotTo(HaveOccurred())

				for _, inst := range instances {
					Eventually(func(g Gomega) {
						// The Restore action's own instance restart is
						// best-effort and can silently leave an instance
						// stopped if it raced with the member only just
						// having come back online; nudge it to start on
						// every poll.
						g.Expect(instance.DefaultManager.EnsureInstanceRunning(inst)).To(Succeed())

						ipv4, err := instance.DefaultManager.GetInstanceIPv4(inst)
						g.Expect(err).NotTo(HaveOccurred())
						g.Expect(ipv4).NotTo(BeEmpty())
					}).WithTimeout(lxd.DefaultEventuallyTimeout).WithPolling(time.Second).Should(Succeed())
				}

				instance.DefaultManager.CheckFleetConnectivity(fleet)
			}
		})
	})
})
