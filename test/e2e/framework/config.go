package framework

import (
	"os"
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
