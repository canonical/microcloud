package scenarios

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework"
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/controller"
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/lxd"
)

var _ = ginkgo.Describe("Evacuate, Reboot, and Restore", ginkgo.Label("risk-medium", "builtin"), ginkgo.Ordered, func() {
	// instances holds the names of the instances deployed for this spec group,
	// declared here (tree-construction time) but only populated in
	// BeforeAll (run time), independent of connectivity.go's own instances.
	var instances []string

	ginkgo.BeforeAll(func() {
		p, err := framework.GetProvisioner()
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		containerFingerprint, vmFingerprint, err := p.EnsureBaseImages(framework.ProjectName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		conn, err := framework.GetRemoteConnection()
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		ginkgo.By("Deploying instances across every cluster member")
		instances, err = lxd.LaunchMixedInstances(conn, "e2e-evacuation-", containerFingerprint, vmFingerprint, framework.ProfileName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		for _, name := range instances {
			ginkgo.DeferCleanup(lxd.DeleteInstance, conn, name)
		}
	})

	ginkgo.When("each cluster member is in turn evacuated, rebooted, and restored", func() {
		ginkgo.It("all instance in the cluster remains fully reachable after the process ", func() {
			tc, err := framework.GetTestbedController()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			conn, err := framework.GetRemoteConnection()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			names, err := tc.GetMembers()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			for _, member := range names {
				ginkgo.By("Evacuating " + member)
				gomega.Expect(lxd.EvacuateMember(conn, member)).To(gomega.Succeed())

				ginkgo.By("Rebooting " + member)
				err = tc.Reboot(member)
				if errors.Is(err, controller.ErrSudoPermissionDenied) {
					ginkgo.GinkgoWriter.Printf("Reboot of %q skipped: passwordless sudo is not configured on this testbed (%v)\n", member, err)
				} else {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				}

				ginkgo.By("Restoring " + member)
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
				gomega.Eventually(func(g gomega.Gomega) {
					err := lxd.RestoreMember(conn, member)

					const offlineErr = "Target cluster member is offline"
					const ovnTimeoutErr = "signal: alarm clock"

					nonRetryable := err != nil && !strings.Contains(err.Error(), offlineErr) && !strings.Contains(err.Error(), ovnTimeoutErr)
					if nonRetryable {
						gomega.StopTrying(fmt.Sprintf("Restoring %q failed with a non-retryable error", member)).Wrap(err).Now()
					}

					g.Expect(err).NotTo(gomega.HaveOccurred())
				}).WithTimeout(lxd.DefaultEventuallyTimeout).WithPolling(time.Second).Should(gomega.Succeed())

				ginkgo.By(fmt.Sprintf("Waiting for instances on %s to come back online", member))
				memberInstances, err := lxd.GetInstanceNamesByMember(conn, member)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				for _, inst := range memberInstances {
					gomega.Eventually(func(g gomega.Gomega) {
						// The Restore action's own instance restart is
						// best-effort and can silently leave an instance
						// stopped if it raced with the member only just
						// having come back online; nudge it to start on
						// every poll.
						g.Expect(lxd.EnsureInstanceRunning(conn, inst)).To(gomega.Succeed())

						ipv4, err := lxd.GetInstanceIPv4(conn, inst)
						g.Expect(err).NotTo(gomega.HaveOccurred())
						g.Expect(ipv4).NotTo(gomega.BeEmpty())
					}).WithTimeout(lxd.DefaultEventuallyTimeout).WithPolling(time.Second).Should(gomega.Succeed())
				}

				for _, src := range instances {
					for _, dst := range instances {
						if src == dst {
							continue
						}

						ginkgo.By(fmt.Sprintf("%s => %s", src, dst))

						gomega.Eventually(func(g gomega.Gomega) {
							output, err := lxd.ProbeSSHReachability(conn, src, dst)
							g.Expect(err).NotTo(gomega.HaveOccurred(), "output: %s", output)
						}).WithTimeout(connectivityTimeout).WithPolling(lxd.DefaultRetryInterval).Should(gomega.Succeed())
					}
				}
			}
		})
	})
})
