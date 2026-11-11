package e2e

import (
	"testing"

	//nolint:staticcheck // Ginkgo/Gomega's DSL is conventionally dot-imported.
	. "github.com/onsi/ginkgo/v2"
	//nolint:staticcheck // Ginkgo/Gomega's DSL is conventionally dot-imported.
	. "github.com/onsi/gomega"
)

// TestE2E is the single Go test entrypoint that Ginkgo hooks into.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MicroCloud E2E Suite")
}
