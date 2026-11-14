package scenarios

import (
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework"
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/lxd"
)

// connectivityTimeout bounds how long the connectivity probes below wait for a just-launched
// instance to become reachable. This needs to be generous: a VM reports Running and acquires its
// DHCP-visible IPv4 address well before its guest OS has actually finished booting (cloud-init,
// network stack, sshd) -- measured live in this environment at ~205s from launch to the guest
// agent/sshd actually responding, compared to ~45s for the IP alone. This is on top of
// lxd.DefaultEventuallyTimeout, which is tuned for already-running instances (e.g. post-restore
// reachability, where the instance was already fully booted before being stopped/restarted).
const connectivityTimeout = 6 * time.Minute

var _ = ginkgo.Describe("Connectivity", ginkgo.Label("risk-low", "builtin"), ginkgo.Ordered, func() {
	// instances holds the names of the instances deployed for this spec group,
	// declared here (tree-construction time) but only populated in
	// BeforeAll (run time), so it starts fresh on every suite run.
	var instances []string

	ginkgo.BeforeAll(func() {
		p, err := framework.GetProvisioner()
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		containerFingerprint, vmFingerprint, err := p.EnsureBaseImages(framework.ProjectName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		conn, err := framework.GetRemoteConnection()
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		ginkgo.By("Deploying a small mixed instances across the cluster")
		instances, err = lxd.LaunchMixedInstances(conn, "e2e-connectivity-", containerFingerprint, vmFingerprint, framework.ProfileName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		for _, name := range instances {
			ginkgo.DeferCleanup(lxd.DeleteInstance, conn, name)
		}

		ginkgo.By("Confirming the instances is running with an IPv4 address")
		for _, name := range instances {
			gomega.Eventually(func(g gomega.Gomega) {
				status, err := lxd.GetInstanceStatus(conn, name)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(status).To(gomega.Equal("Running"))

				ipv4, err := lxd.GetInstanceIPv4(conn, name)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(ipv4).NotTo(gomega.BeEmpty())
			}).WithTimeout(lxd.DefaultEventuallyTimeout).WithPolling(lxd.DefaultRetryInterval).Should(gomega.Succeed())
		}
	})

	ginkgo.When("every instance attempts to reach every other instance's SSH port", func() {
		ginkgo.It("all probes succeed", func() {
			conn, err := framework.GetRemoteConnection()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

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
		})
	})
})
