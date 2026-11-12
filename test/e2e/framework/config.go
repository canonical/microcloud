package framework

import (
	"os"

	"github.com/onsi/gomega"
)

const (
	// ProfileName is the name of the profile created and used by the test suite.
	ProfileName = "e2e-testing"
	// ProjectName is the name of the project created and used by the test suite.
	ProjectName = "e2e-testing"
)

// Remote is the name of the LXD remote the suite targets. It must point at an already-deployed,
// already-reachable MicroCloud cluster.
func Remote() string {
	return os.Getenv("REMOTE")
}

// E2ETestbedFile returns the path to the testbed config file.
func E2ETestbedFile() string {
	return os.Getenv("E2E_TESTBED_FILE")
}

// CheckEnvVars checks that the required environment variables are set and valid.
func CheckEnvVars() {
	gomega.Expect(Remote()).NotTo(gomega.Equal("local"), "This suite is not meant to be run against the local LXD server")
	gomega.Expect(Remote()).NotTo(gomega.BeEmpty(), "Please set the REMOTE environment variable to the name of the remote LXD server")
}
