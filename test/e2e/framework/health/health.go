package health

import (
	"fmt"
	"sort"
	"strings"

	"github.com/onsi/ginkgo/v2"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework/controller"
	"github.com/canonical/microcloud/microcloud/test/e2e/framework/lxd"
)

// reportEntryName is the name AddReportEntry attaches the accumulated health-check log.
const reportEntryName = "Cluster health check"

// healthChecker runs health checks via a controller.Controller and accumulates their combined
// output, attaching the full log to the Ginkgo report whether the checks pass or fail partway
// through.
type healthChecker struct {
	controller controller.Controller
	log        strings.Builder
}

// newHealthChecker returns a healthChecker using nc.
func newHealthChecker(nc controller.Controller) *healthChecker {
	return &healthChecker{controller: nc}
}

// attachToReport attaches the log accumulated so far to the Ginkgo report regardless of whether
// the health check has passed or failed so far.
func (h *healthChecker) attachToReport() {
	ginkgo.AddReportEntry(reportEntryName, h.log.String())
}

// check runs argv on member, appends a header plus its combined output to the log, and returns the
// combined output. It calls Fail (via Ginkgo) immediately -- stopping the health check at the
// first failure, after attaching the log so far to the report -- if the command could not be run
// or its exit code is non-zero. If the non-zero exit looks like sudo rejected the command for
// lacking privileges (see controller.IsSudoPermissionDenied), it calls Skip instead of Fail: this
// is treated as a testbed configuration limitation (no passwordless sudo), not a genuine cluster
// health problem.
func (h *healthChecker) check(member string, argv ...string) string {
	name := fmt.Sprintf("%s: %s", member, strings.Join(argv, " "))

	stdout, stderr, rc, err := h.controller.Exec(member, argv, nil)
	output := stdout + stderr

	fmt.Fprintf(&h.log, "Checking %s\n%s\n", name, output)

	if err != nil {
		fmt.Fprintf(&h.log, "Precondition %q failed: %v\n", name, err)
		h.attachToReport()
		ginkgo.Fail(fmt.Sprintf("Health check failed: %q: %v (see %q report entry for full output)", name, err, reportEntryName))
	}

	if rc != 0 {
		if controller.IsSudoPermissionDenied(output) {
			fmt.Fprintf(&h.log, "Precondition %q skipped: testbed user cannot run sudo non-interactively\n", name)
			h.attachToReport()
			ginkgo.Skip(fmt.Sprintf("Skipping cluster health check: %q requires passwordless sudo, which is not configured on this testbed (see %q report entry for full output)", name, reportEntryName))
		}

		fmt.Fprintf(&h.log, "Precondition %q failed: exit code %d\n", name, rc)
		h.attachToReport()
		ginkgo.Fail(fmt.Sprintf("Health check failed: %q exited with code %d (see %q report entry for full output)", name, rc, reportEntryName))
	}

	return output
}

// require calls Fail (stopping at the first failure, like check, after attaching the log so far to
// the report) unless output contains substring.
func (h *healthChecker) require(name, output, substring string) {
	if !strings.Contains(output, substring) {
		fmt.Fprintf(&h.log, "Precondition %q failed: output does not contain %q\n", name, substring)
		h.attachToReport()
		ginkgo.Fail(fmt.Sprintf("Health check failed: %q: output does not contain %q (see %q report entry for full output)", name, substring, reportEntryName))
	}
}

// checkAndRequire runs argv on member (see check) and then requires its output to contain every
// one of substrings (see require).
func (h *healthChecker) checkAndRequire(member string, substrings []string, argv ...string) {
	name := fmt.Sprintf("%s: %s", member, strings.Join(argv, " "))
	output := h.check(member, argv...)
	for _, substring := range substrings {
		h.require(name, output, substring)
	}
}

// done appends the final success line and attaches the full log to the Ginkgo report.
func (h *healthChecker) done() {
	fmt.Fprintln(&h.log, "All checks passed")
	h.attachToReport()
}

// CheckClusterStatus verifies the cluster is healthy by running a fixed set of status checks via c
// against the cluster's member names (see lxd.Connection.GetClusterMembers), stopping at the first
// failure. Output is always attached to the Ginkgo report.
func CheckClusterStatus(c controller.Controller, conn *lxd.Connection) {
	names, err := conn.GetClusterMembers()
	if err != nil {
		ginkgo.Fail(err.Error())
	}

	sort.Strings(names)

	if len(names) == 0 {
		ginkgo.Fail("Cannot check cluster health: no cluster members found")
	}

	ginkgo.By("Checking cluster health")

	h := newHealthChecker(c)

	first := names[0]

	checkMicroCloud(h, first)
	checkMicroCeph(h, first)
	checkMicroOVN(h, first)

	h.done()
}
