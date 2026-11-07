package scenarios

import (
	"fmt"
	"math/rand"

	//nolint:staticcheck // Ginkgo/Gomega's DSL is conventionally dot-imported.
	. "github.com/onsi/ginkgo/v2"
	//nolint:staticcheck // Ginkgo/Gomega's DSL is conventionally dot-imported.
	. "github.com/onsi/gomega"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework"
)

// connectivitySampleSize is the number of instances sampled as sources and
// destinations in checkAllConnectivity.
const connectivitySampleSize = 3

// sshProbePort is the TCP port probed by execInSSHProbe to check reachability
// between instances.
const sshProbePort = 22

// checkConnectivity asserts that src can reach dst's SSH port, retrying (via
// Gomega's Eventually) while dst's instance is still acquiring an IPv4
// address.
func checkConnectivity(src, dst string) {
	By(fmt.Sprintf("%s => %s", src, dst))

	Eventually(func(g Gomega) {
		dstIPv4, err := framework.GetInstanceIPv4(dst)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(dstIPv4).NotTo(BeEmpty(), "instance %q has no IPv4 address yet", dst)

		output, err := execInSSHProbe(src, dstIPv4)
		g.Expect(err).NotTo(HaveOccurred(), "output: %s", output)
	}).WithTimeout(framework.DefaultEventuallyTimeout).WithPolling(framework.DefaultRetryInterval).Should(Succeed())
}

// execInSSHProbe runs a command inside src that attempts to open a TCP
// connection to dst's SSH port. It returns the combined stdout/stderr and an
// error if the command exited non-zero.
func execInSSHProbe(src, dstIPv4 string) (string, error) {
	command := []string{"bash", "-c", fmt.Sprintf("grep -qm1 ^SSH- < /dev/tcp/%s/%d", dstIPv4, sshProbePort)}

	stdout, stderr, rc, err := framework.ExecInInstance(src, command, nil)
	output := stdout + stderr
	if err != nil {
		return output, fmt.Errorf("Probe from %q to %s:%d failed: %w", src, dstIPv4, sshProbePort, err)
	}

	if rc != 0 {
		return output, fmt.Errorf("Probe from %q to %s:%d exited with code %d", src, dstIPv4, sshProbePort, rc)
	}

	return output, nil
}

// checkAllConnectivity exercises connectivity between a random sample of
// instances. Sampling (rather than an exhaustive N x N check) keeps the check
// fast on large deployments while still covering cross-member traffic with
// high probability.
func checkAllConnectivity() {
	instances, err := framework.GetInstanceNames()
	Expect(err).NotTo(HaveOccurred())
	Expect(instances).NotTo(BeEmpty(), "No instances found in project %q", framework.ProjectName())

	for _, src := range randomPick(instances, connectivitySampleSize) {
		for _, dst := range randomPick(instances, connectivitySampleSize) {
			if src == dst {
				continue
			}

			checkConnectivity(src, dst)
		}
	}
}

// randomPick returns up to count distinct, randomly selected elements of list.
func randomPick(list []string, count int) []string {
	shuffled := make([]string, len(list))
	copy(shuffled, list)
	rand.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

	if count > len(shuffled) {
		count = len(shuffled)
	}

	return shuffled[:count]
}

var _ = Describe("Connectivity", Label("risk-low", "builtin"), func() {
	It("allows instances to reach each other over the network", func() {
		checkAllConnectivity()
	})
})
