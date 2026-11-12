package scenarios

import (
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework"
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/lxd"
)

var _ = ginkgo.Describe("Reboot", ginkgo.Label("risk-medium", "builtin"), ginkgo.Ordered, func() {
	// instances holds the names of the instances deployed for this spec group,
	// declared here (tree-construction time) but only populated in
	// BeforeAll (run time), independent of other scenarios' own instance sets.
	var instances []string

	ginkgo.BeforeAll(func() {
		p, err := framework.GetProvisioner()
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		containerFingerprint, vmFingerprint, err := p.EnsureBaseImages(framework.ProjectName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		conn, err := framework.GetRemoteConnection()
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		ginkgo.By("Deploying instances across every cluster member")
		instances, err = lxd.LaunchMixedInstances(conn, "e2e-reboot-", containerFingerprint, vmFingerprint, framework.ProfileName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		for _, name := range instances {
			ginkgo.DeferCleanup(lxd.DeleteInstance, conn, name)
		}
	})

	ginkgo.When("each cluster member in turn is rebooted", func() {
		ginkgo.It("every member's instances come back online and the full instance set remains fully reachable", func() {
			tc, err := framework.GetTestbedController()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			conn, err := framework.GetRemoteConnection()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			names, err := tc.GetMembers()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			for _, member := range names {
				ginkgo.By("Rebooting " + member)
				gomega.Expect(tc.Reboot(member)).To(gomega.Succeed())

				ginkgo.By(fmt.Sprintf("Waiting for instances on %s to come back online", member))
				memberInstances, err := lxd.GetInstanceNamesByMember(conn, member)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				for _, inst := range memberInstances {
					gomega.Eventually(func(g gomega.Gomega) {
						// The instance's own auto-start on boot is
						// best-effort and can silently leave it stopped
						// if it raced with the member only just having
						// come back online; nudge it to start on every
						// poll.
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
