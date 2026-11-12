package framework

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework/controller"
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/health"
)

// connectDefaults connects to Remote() and validates the testbed/cluster are usable, returning the
// validated controller.Controller. Called once per Ginkgo process -- by SetupProcess in process 1,
// and by ConnectProcess in every process (including process 1 again) -- since ginkgo -p spawns
// separate OS processes and none of this package's state is shared between them.
func connectDefaults() controller.Controller {
	// Expect valid connection to testbed.
	nc, err := GetTestbedController()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	// Expect valid connection to remote microcloud (lxc remote).
	conn, err := GetRemoteConnection()
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "The %q remote is not usable", Remote())

	members, err := conn.Server().GetClusterMembers()
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Cannot list cluster members")
	gomega.Expect(members).NotTo(gomega.BeEmpty(), "Remote %q is not part of a cluster", Remote())

	for _, m := range members {
		gomega.Expect(m.Status).To(gomega.Equal("Online"),
			"Cluster member %q is not online (status: %s), aborting", m.ServerName, m.Status)
	}

	return nc
}

// SetupProcess runs setup steps that only need to be done once, in a single process, before any
// specs are run.
func SetupProcess() []byte {
	CheckEnvVars()

	nc := connectDefaults()

	conn, err := GetRemoteConnection()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	health.CheckClusterStatus(nc, conn)

	ginkgo.By("Setting up the e2e environment on " + Remote())

	p, err := GetProvisioner()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	_, _, err = p.SetupEnvironment(ProjectName, ProfileName)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	return nil
}

// ConnectProcess runs in every parallel Ginkgo process (including the one that ran SetupProcess),
// re-validating the same testbed/cluster connections SetupProcess already validated in process 1,
// without repeating the expensive/mutating steps (cluster health check, project/profile/image
// bootstrap) that only need to happen once.
func ConnectProcess(_ []byte) {
	connectDefaults()
}

// TeardownProcess runs once, after every parallel Ginkgo process has finished (it is the second
// function passed to SynchronizedAfterSuite).
func TeardownProcess() {
	ginkgo.By("Tearing down the e2e environment")

	p, err := GetProvisioner()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	gomega.Expect(p.TeardownEnvironment(ProjectName, ProfileName)).To(gomega.Succeed())
}
