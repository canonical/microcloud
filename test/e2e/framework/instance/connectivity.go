package instance

import (
	"fmt"
	"strconv"
	"time"

	//nolint:staticcheck // Ginkgo/Gomega's DSL is conventionally dot-imported.
	. "github.com/onsi/ginkgo/v2"
	//nolint:staticcheck // Ginkgo/Gomega's DSL is conventionally dot-imported.
	. "github.com/onsi/gomega"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework/lxd"
)

// sshProbePort is the TCP port probed by execInSSHProbe to check reachability
// between instances.
const sshProbePort = 22

// connectivityTimeout bounds how long checkConnectivity waits for a
// just-launched instance to become reachable. This needs to be generous: a
// VM reports Running and acquires its DHCP-visible IPv4 address well before
// its guest OS has actually finished booting (cloud-init, network stack,
// sshd) — measured live in this environment at ~205s from launch to the
// guest agent/sshd actually responding, compared to ~45s for the IP alone.
// This is on top of lxd.DefaultEventuallyTimeout, which is tuned for
// already-running instances (e.g. post-restore reachability, where the
// instance was already fully booted before being stopped/restarted).
const connectivityTimeout = 6 * time.Minute

// CheckFleetConnectivity exercises connectivity exhaustively (every ordered
// pair) between the given instances. Exhaustive rather than sampled, since
// callers are expected to pass a small, fixed-size fleet rather than every
// instance in the project.
func (m *Manager) CheckFleetConnectivity(fleet []string) {
	GinkgoHelper()

	for _, src := range fleet {
		for _, dst := range fleet {
			if src == dst {
				continue
			}

			m.checkConnectivity(src, dst)
		}
	}
}

// checkConnectivity asserts that src can reach dst's SSH port, retrying (via
// Gomega's Eventually) while dst's instance is still booting/acquiring an
// IPv4 address.
func (m *Manager) checkConnectivity(src, dst string) {
	GinkgoHelper()

	By(fmt.Sprintf("%s => %s", src, dst))

	Eventually(func(g Gomega) {
		dstIPv4, err := m.GetInstanceIPv4(dst)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(dstIPv4).NotTo(BeEmpty(), "instance %q has no IPv4 address yet", dst)

		output, err := m.execInSSHProbe(src, dstIPv4)
		g.Expect(err).NotTo(HaveOccurred(), "output: %s", output)
	}).WithTimeout(connectivityTimeout).WithPolling(lxd.DefaultRetryInterval).Should(Succeed())
}

// execInSSHProbe runs a command inside src that attempts to open a TCP
// connection to dst's SSH port. It returns the combined stdout/stderr and an
// error if the command exited non-zero.
//
// The connection attempt is bounded to probeTimeout: bash's /dev/tcp has no
// timeout of its own, and a genuinely unreachable destination (as opposed to
// one that actively refuses the connection) would otherwise block for the
// kernel's default TCP SYN-retry duration (commonly ~2 minutes), which would
// blow through Eventually's polling budget in a single attempt.
func (m *Manager) execInSSHProbe(src, dstIPv4 string) (string, error) {
	GinkgoHelper()

	const probeTimeout = 5 * time.Second

	command := []string{"timeout", strconv.Itoa(int(probeTimeout.Seconds())), "bash", "-c", fmt.Sprintf("grep -qm1 ^SSH- < /dev/tcp/%s/%d", dstIPv4, sshProbePort)}

	stdout, stderr, rc, err := m.ExecInInstance(src, command, nil)
	output := stdout + stderr
	if err != nil {
		return output, fmt.Errorf("Probe from %q to %s:%d failed: %w", src, dstIPv4, sshProbePort, err)
	}

	if rc != 0 {
		return output, fmt.Errorf("Probe from %q to %s:%d exited with code %d", src, dstIPv4, sshProbePort, rc)
	}

	return output, nil
}
