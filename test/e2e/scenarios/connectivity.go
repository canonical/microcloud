package scenarios

import (
	"fmt"
	"time"

	//nolint:staticcheck // Ginkgo/Gomega's DSL is conventionally dot-imported.
	. "github.com/onsi/ginkgo/v2"
	//nolint:staticcheck // Ginkgo/Gomega's DSL is conventionally dot-imported.
	. "github.com/onsi/gomega"

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

var _ = Describe("Connectivity", Label("risk-low", "builtin"), Ordered, func() {
	// fleet holds the names of the instances deployed for this spec group,
	// declared here (tree-construction time) but only populated in
	// BeforeAll (run time), so it starts fresh on every suite run.
	var fleet []string

	BeforeAll(func() {
		p, err := framework.GetProvisioner()
		Expect(err).NotTo(HaveOccurred())

		containerFingerprint, vmFingerprint, err := p.EnsureBaseImages(framework.ProjectName)
		Expect(err).NotTo(HaveOccurred())

		conn, err := framework.GetRemoteConnection()
		Expect(err).NotTo(HaveOccurred())

		By("Deploying a small mixed fleet across the cluster")
		fleet, err = lxd.LaunchMixedInstances(conn, "e2e-connectivity-", containerFingerprint, vmFingerprint, framework.ProfileName)
		Expect(err).NotTo(HaveOccurred())

		for _, name := range fleet {
			DeferCleanup(lxd.DeleteInstance, conn, name)
		}

		By("Confirming the fleet is running with an IPv4 address")
		for _, name := range fleet {
			Eventually(func(g Gomega) {
				status, err := lxd.GetInstanceStatus(conn, name)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(status).To(Equal("Running"))

				ipv4, err := lxd.GetInstanceIPv4(conn, name)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ipv4).NotTo(BeEmpty())
			}).WithTimeout(lxd.DefaultEventuallyTimeout).WithPolling(lxd.DefaultRetryInterval).Should(Succeed())
		}
	})

	When("every instance attempts to reach every other instance's SSH port", func() {
		It("all probes succeed", func() {
			conn, err := framework.GetRemoteConnection()
			Expect(err).NotTo(HaveOccurred())

			for _, src := range fleet {
				for _, dst := range fleet {
					if src == dst {
						continue
					}

					By(fmt.Sprintf("%s => %s", src, dst))

					Eventually(func(g Gomega) {
						output, err := lxd.ProbeSSHReachability(conn, src, dst)
						g.Expect(err).NotTo(HaveOccurred(), "output: %s", output)
					}).WithTimeout(connectivityTimeout).WithPolling(lxd.DefaultRetryInterval).Should(Succeed())
				}
			}
		})
	})
})
