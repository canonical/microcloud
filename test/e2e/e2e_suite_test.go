// Package e2e_test is the reference harness for running MicroCloud's
// post-deployment end-to-end test suite directly from this repository.
//
// It assumes a MicroCloud cluster is already deployed and reachable through an
// LXD remote (see README.md). It does not provision clusters itself.
package e2e_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework"
	_ "github.com/canonical/microcloud/microcloud/test/e2e/scenarios"
)

// TestE2E is the single Go test entrypoint that Ginkgo hooks into.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MicroCloud E2E Suite")
}

// SynchronizedBeforeSuite runs once before any spec.
var _ = SynchronizedBeforeSuite(framework.SetupSuite, framework.AttachToSuite)

// SynchronizedAfterSuite is the teardown counterpart.
var _ = SynchronizedAfterSuite(framework.TeardownProcess, framework.TeardownSuite)
