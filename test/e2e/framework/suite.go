package framework

import (
	"fmt"
	"os/exec"

	"github.com/canonical/lxd/shared/api"
	//nolint:staticcheck // Ginkgo/Gomega's DSL is conventionally dot-imported.
	. "github.com/onsi/ginkgo/v2"
	//nolint:staticcheck // Ginkgo/Gomega's DSL is conventionally dot-imported.
	. "github.com/onsi/gomega"
)

// memberNames returns the ServerName of every cluster member.
func memberNames(members []api.ClusterMember) []string {
	names := make([]string, 0, len(members))
	for _, m := range members {
		names = append(names, m.ServerName)
	}

	return names
}

// prepareNodeControl loads the testbed config and validates whichever section
// it configures (lxd or ssh), setting Env() accordingly.
func prepareNodeControl(names []string) {
	err := LoadTestbedConfig()
	Expect(err).NotTo(HaveOccurred())

	switch {
	case Testbed().LXD != nil:
		env := EnvUnknown
		if validateLocalLXDProject(names) {
			env = EnvLXD
		}

		SetEnv(env)
		By(fmt.Sprintf("Using lxd testbed config (environment: %s)", env))

	case Testbed().SSH != nil:
		env := EnvUnknown
		if validateSSHNodes(Testbed().SSH.Nodes) {
			env = EnvSSH
		}

		SetEnv(env)
		By(fmt.Sprintf("Using ssh testbed config (environment: %s)", env))

	default:
		SetEnv(EnvUnknown)
	}
}

// CheckPreconditions verifies REMOTE is set, is not "local", the remote is
// reachable, and all cluster members are ONLINE. It connects Server() as a
// side effect.
func CheckPreconditions() {
	remoteName := Remote()
	Expect(remoteName).NotTo(BeEmpty(),
		"Please set the REMOTE environment variable to the name of the remote LXD server")
	Expect(remoteName).NotTo(Equal("local"),
		"This suite is not meant to be run against the local LXD server")

	s, err := ConnectRemoteServer(remoteName)
	Expect(err).NotTo(HaveOccurred(), "The %q remote is not usable; see "+
		"https://documentation.ubuntu.com/lxd/latest/howto/server_expose/#authenticate-with-the-lxd-server", remoteName)
	SetServer(s)

	members, err := Server().GetClusterMembers()
	Expect(err).NotTo(HaveOccurred(), "Cannot list cluster members")
	Expect(members).NotTo(BeEmpty(), "Remote %q is not part of a cluster", remoteName)

	for _, m := range members {
		Expect(m.Status).To(Equal("Online"),
			"Cluster member %q is not online (status: %s), aborting", m.ServerName, m.Status)
	}

	prepareNodeControl(memberNames(members))

	CheckClusterHealth()

	_, err = exec.LookPath("terraform")
	if err != nil {
		Fail("Missing command: terraform")
	}
}

// SetupSuite is the first function to pass to SynchronizedBeforeSuite: it runs
// once, in a single Ginkgo process, before any spec runs. 
func SetupSuite() []byte {
	CheckPreconditions()

	By("Setting up e2e environment on " + Remote())
	SetupTerraform()

	return nil
}

// AttachToSuite is the second function to pass to SynchronizedBeforeSuite:
// every parallel Ginkgo process (including the one that ran SetupSuite)
// runs it, since ginkgo -p runs separate OS processes and none of this
// package's state is shared across them.
func AttachToSuite(_ []byte) {
	if Server() != nil {
		// This is the process that ran SetupSuite/CheckPreconditions:
		// Server(), Env(), and Testbed() are already populated.
		return
	}

	s, err := ConnectRemoteServer(Remote())
	Expect(err).NotTo(HaveOccurred())
	SetServer(s)

	members, err := Server().GetClusterMembers()
	Expect(err).NotTo(HaveOccurred())

	prepareNodeControl(memberNames(members))
}

// TeardownProcess is the first function to pass to SynchronizedAfterSuite: it
// runs in every process.
func TeardownProcess() {}

// TeardownSuite is the second function to pass to SynchronizedAfterSuite: it
// runs only once, after every process is done.
func TeardownSuite() {
	if !DestroyAfterSuite() {
		GinkgoWriter.Println("Skipping destroy (E2E_DESTROY=no)")
		return
	}

	By("Tearing down e2e environment")
	TeardownTerraform()
}
