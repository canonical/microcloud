package framework

import (
	//nolint:staticcheck // Ginkgo/Gomega's DSL is conventionally dot-imported.
	. "github.com/onsi/ginkgo/v2"
	//nolint:staticcheck // Ginkgo/Gomega's DSL is conventionally dot-imported.
	. "github.com/onsi/gomega"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework/controller"
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/environment"
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/instance"
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/lxd"
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/testbed"
)

// SetupProcess runs setup steps that only need to be done once, in a single process, before any
// specs are run.
func SetupProcess() []byte {
	Expect(Remote()).NotTo(Equal("local"), "This suite is not meant to be run against the local LXD server")
	Expect(Remote()).NotTo(BeEmpty(), "Please set the REMOTE environment variable to the name of the remote LXD server")

	conn, err := lxd.Connect(Remote(), ProjectName)
	Expect(err).NotTo(HaveOccurred(), "The %q remote is not usable", Remote())

	members, err := conn.Server().GetClusterMembers()
	Expect(err).NotTo(HaveOccurred(), "Cannot list cluster members")
	Expect(members).NotTo(BeEmpty(), "Remote %q is not part of a cluster", Remote())

	names := make([]string, 0, len(members))

	for _, m := range members {
		Expect(m.Status).To(Equal("Online"),
			"Cluster member %q is not online (status: %s), aborting", m.ServerName, m.Status)

		names = append(names, m.ServerName)
	}

	cfg, err := testbed.Load(E2ETestbedFile())
	Expect(err).NotTo(HaveOccurred())

	controller.DefaultController = controller.New(conn, cfg)
	controller.DefaultController.Prepare(names)
	controller.DefaultController.CheckClusterHealth()

	By("Setting up the e2e environment on " + Remote())

	environment.DefaultBootstrapper = environment.New(conn)
	Expect(environment.DefaultBootstrapper.EnsureProject(ProjectName)).To(Succeed())
	Expect(environment.DefaultBootstrapper.EnsureProfile(ProfileName)).To(Succeed())

	_, _, err = environment.DefaultBootstrapper.EnsureBaseImages(ProjectName)
	Expect(err).NotTo(HaveOccurred())

	instance.DefaultManager = instance.New(conn)

	return nil
}

// TeardownProcess runs in every parallel Ginkgo process (ginkgo -p runs
// separate OS processes, so there is nothing shared to clean up here).
func TeardownProcess() {
	if environment.DefaultBootstrapper == nil {
		// SynchronizedBeforeSuite failed before setting this up (e.g. the
		// cluster health check failed): nothing to tear down.
		return
	}

	By("Tearing down the e2e environment")
	Expect(environment.DefaultBootstrapper.TeardownEnvironment(ProjectName, ProfileName)).To(Succeed())
}
