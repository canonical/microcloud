package framework

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/gruntwork-io/terratest/modules/testing"
	//nolint:staticcheck // Ginkgo/Gomega's DSL is conventionally dot-imported.
	. "github.com/onsi/ginkgo/v2"
	//nolint:staticcheck // Ginkgo/Gomega's DSL is conventionally dot-imported.
	. "github.com/onsi/gomega"
)

// DefaultRetryInterval is the wait time between retries of flaky operations
// (instance readiness polling, etc).
const DefaultRetryInterval = 5 * time.Second

// DefaultEventuallyTimeout bounds how long specs wait for eventually-
// consistent conditions (e.g. an instance acquiring an IPv4 address after
// being evacuated/restored) before failing.
const DefaultEventuallyTimeout = 30 * time.Second

// terraformMaxRetries is the value passed as Terratest's MaxRetries, i.e. the
// number of retries after an initial attempt, giving terraformMaxRetries+1
// attempts in total.
const terraformMaxRetries = 2

// terraformDir is the directory containing main.tf and other Terraform files,
// used as the working directory for all terraform commands.
const terraformDir = "terraform"

// Remote is the name of the LXD remote the suite targets. It must point at an
// already-deployed, already-reachable MicroCloud cluster.
func Remote() string {
	return os.Getenv("REMOTE")
}

// DestroyAfterSuite controls whether the Terraform-managed e2e environment is
// torn down once the suite finishes. 
func DestroyAfterSuite() bool {
	return os.Getenv("E2E_DESTROY") != "no"
}

// terraformOptions builds the Terratest options used for every Terraform
// invocation. Transient upstream LXD races are retried at a fixed
// DefaultRetryInterval; anything else fails immediately:
//
//   - https://github.com/canonical/lxd/issues/11636: multiple cluster
//     members racing to insert the same image volume row.
//   - https://github.com/canonical/lxd/issues/15315: a clone racing ahead
//     of its parent image snapshot becoming visible.
//   - Multiple cluster members racing to unpack the same cached image onto
//     a Ceph-backed storage pool: the loser's cleanup of its own
//     leftover/partially-unpacked RBD volume races the winner's still-open
//     watcher on that same volume, so the delete transiently fails with
//     "image still has watchers".
func terraformOptions() *terraform.Options {
	return &terraform.Options{
		TerraformDir: terraformDir,
		Vars:         map[string]any{"remote": Remote()},
		NoColor:      true,
		Logger:       logger.New(ginkgoLogger{}),
		RetryableTerraformErrors: map[string]string{
			`UNIQUE constraint failed: index 'storage_volumes_unique_storage_pool_id_node_id_project_id_name_type'`: "LXD image volume insert race (lxd#11636)",
			`failed to open parent image`:          "LXD clone raced parent image snapshot (lxd#15315)",
			`rbd: error: image still has watchers`: "Ceph RBD image volume cleanup raced a concurrent unpack's open watcher",
		},
		MaxRetries:         terraformMaxRetries,
		TimeBetweenRetries: DefaultRetryInterval,
	}
}

// ginkgoLogger streams Terratest log output to GinkgoWriter.
type ginkgoLogger struct{}

// Logf implements logger.TestLogger by streaming Terratest's log output to
// GinkgoWriter.
func (ginkgoLogger) Logf(_ testing.TestingT, format string, args ...any) {
	fmt.Fprintf(GinkgoWriter, format+"\n", args...)
}

// SetupTerraform runs `terraform init` + `terraform apply`.
func SetupTerraform() {
	_, err := terraform.InitAndApplyContextE(GinkgoT(), context.Background(), terraformOptions())
	Expect(err).NotTo(HaveOccurred(), "terraform init/apply failed")
}

// TeardownTerraform runs `terraform destroy`. It is a no-op when
// DestroyAfterSuite() is false.
func TeardownTerraform() {
	if !DestroyAfterSuite() {
		return
	}

	_, err := terraform.DestroyContextE(GinkgoT(), context.Background(), terraformOptions())
	Expect(err).NotTo(HaveOccurred(), "terraform destroy failed")
}
