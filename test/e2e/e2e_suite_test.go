package e2e

import (
	"testing"

	//nolint:staticcheck // Ginkgo/Gomega's DSL is conventionally dot-imported.
	. "github.com/onsi/ginkgo/v2"
	//nolint:staticcheck // Ginkgo/Gomega's DSL is conventionally dot-imported.
	. "github.com/onsi/gomega"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework"
	_ "github.com/canonical/microcloud/microcloud/test/e2e/scenarios"
)

// TestE2E is the single Go test entrypoint that Ginkgo hooks into.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MicroCloud E2E Suite")
}

// SynchronizedBeforeSuite runs once before any spec, in a single process; its second function then
// runs in every parallel process (including the one that ran the first), since none of this
// package's state is shared across ginkgo -p's separate OS processes.
var _ = SynchronizedBeforeSuite(framework.SetupProcess, framework.ConnectProcess)

// SynchronizedAfterSuite is the teardown counterpart: its first function
// runs in every process, its second only once, after every process is done.
var _ = SynchronizedAfterSuite(func() {}, framework.TeardownProcess)
