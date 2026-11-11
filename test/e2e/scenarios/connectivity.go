package scenarios

import (
	//nolint:staticcheck // Ginkgo/Gomega's DSL is conventionally dot-imported.
	. "github.com/onsi/ginkgo/v2"
	//nolint:staticcheck // Ginkgo/Gomega's DSL is conventionally dot-imported.
	. "github.com/onsi/gomega"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework"
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/environment"
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/instance"
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/lxd"
)

var _ = Describe("Connectivity", Label("risk-low", "builtin"), Ordered, func() {
	// fleet holds the names of the instances deployed for this spec group,
	// declared here (tree-construction time) but only populated in
	// BeforeAll (run time), so it starts fresh on every suite run.
	var fleet []string

	BeforeAll(func() {
		containerFingerprint, vmFingerprint, err := environment.DefaultBootstrapper.EnsureBaseImages(framework.ProjectName)
		Expect(err).NotTo(HaveOccurred())

		By("Deploying a small mixed fleet across the cluster")
		fleet, err = instance.DefaultManager.LaunchMixedFleet("e2e-connectivity-", containerFingerprint, vmFingerprint, framework.ProfileName)
		Expect(err).NotTo(HaveOccurred())

		for _, name := range fleet {
			DeferCleanup(instance.DefaultManager.DeleteInstance, name)
		}

		By("Confirming the fleet is running with an IPv4 address")
		for _, name := range fleet {
			Eventually(func(g Gomega) {
				status, err := instance.DefaultManager.GetInstanceStatus(name)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(status).To(Equal("Running"))

				ipv4, err := instance.DefaultManager.GetInstanceIPv4(name)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ipv4).NotTo(BeEmpty())
			}).WithTimeout(lxd.DefaultEventuallyTimeout).WithPolling(lxd.DefaultRetryInterval).Should(Succeed())
		}
	})

	When("every instance attempts to reach every other instance's SSH port", func() {
		It("all probes succeed", func() {
			instance.DefaultManager.CheckFleetConnectivity(fleet)
		})
	})
})
