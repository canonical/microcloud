package controller

import (
	"fmt"
	"sort"
	"strings"

	//nolint:staticcheck // Ginkgo's DSL is conventionally dot-imported.
	. "github.com/onsi/ginkgo/v2"
)

// reportEntryName is the name AddReportEntry attaches the accumulated
// health-check log.
const reportEntryName = "Cluster health check"

// healthChecker runs health checks via a NodeController and accumulates their
// combined output, attaching the full log to the Ginkgo report whether the
// checks pass or fail partway through.
type healthChecker struct {
	controller NodeController
	log        strings.Builder
}

// newHealthChecker returns a healthChecker using nc.
func newHealthChecker(nc NodeController) *healthChecker {
	return &healthChecker{controller: nc}
}

// attachToReport attaches the log accumulated so far to the Ginkgo report
// regardless of whether the health check has passed or failed so far.
func (h *healthChecker) attachToReport() {
	AddReportEntry(reportEntryName, h.log.String())
}

// check runs argv on member, appends a header plus its combined output to the
// log, and returns the combined output. It calls Fail (via Ginkgo) immediately
// -- stopping the health check at the first failure, after attaching the log
// so far to the report -- if the command could not be run or its exit code is
// non-zero.
func (h *healthChecker) check(member string, argv ...string) string {
	name := fmt.Sprintf("%s: %s", member, strings.Join(argv, " "))

	stdout, stderr, rc, err := h.controller.Exec(member, argv, nil)
	output := stdout + stderr

	fmt.Fprintf(&h.log, "Checking %s\n%s\n", name, output)

	if err != nil {
		fmt.Fprintf(&h.log, "Precondition %q failed: %v\n", name, err)
		h.attachToReport()
		Fail(fmt.Sprintf("Health check failed: %q: %v (see %q report entry for full output)", name, err, reportEntryName))
	}

	if rc != 0 {
		fmt.Fprintf(&h.log, "Precondition %q failed: exit code %d\n", name, rc)
		h.attachToReport()
		Fail(fmt.Sprintf("Health check failed: %q exited with code %d (see %q report entry for full output)", name, rc, reportEntryName))
	}

	return output
}

// require calls Fail (stopping at the first failure, like check, after
// attaching the log so far to the report) unless output contains substring.
func (h *healthChecker) require(name, output, substring string) {
	if !strings.Contains(output, substring) {
		fmt.Fprintf(&h.log, "Precondition %q failed: output does not contain %q\n", name, substring)
		h.attachToReport()
		Fail(fmt.Sprintf("Health check failed: %q: output does not contain %q (see %q report entry for full output)", name, substring, reportEntryName))
	}
}

// checkAndRequire runs argv on member (see check) and then requires its output
// to contain every one of substrings (see require).
func (h *healthChecker) checkAndRequire(member string, substrings []string, argv ...string) {
	name := fmt.Sprintf("%s: %s", member, strings.Join(argv, " "))
	output := h.check(member, argv...)
	for _, substring := range substrings {
		h.require(name, output, substring)
	}
}

// done appends the final success line and attaches the full log to the Ginkgo
// report.
func (h *healthChecker) done() {
	fmt.Fprintln(&h.log, "All checks passed")
	h.attachToReport()
}

// CheckClusterHealth verifies the cluster is healthy by running a fixed set
// of status checks via the resolved NodeController against the cluster
// member names cached by Prepare, stopping at the first failure. Output is
// always attached to the Ginkgo report.
func (c *Controller) CheckClusterHealth() {
	nc, err := c.Resolve()
	if err != nil {
		Fail(err.Error())
	}

	names := append([]string(nil), c.members...)
	sort.Strings(names)

	By("Checking cluster health")

	h := newHealthChecker(nc)

	first := names[0]

	h.checkAndRequire(first, []string{"Status: HEALTHY"}, "sudo", "microcloud", "status")
	h.check(first, "sudo", "microceph", "status")
	h.checkAndRequire(first, []string{"HEALTH_OK"}, "sudo", "ceph", "-s")
	h.checkAndRequire(first, []string{"OVN Northbound: OK", "OVN Southbound: OK"}, "sudo", "microovn", "status")

	for _, member := range names {
		h.check(member, "sudo", "systemctl", "status", "--failed")
	}

	h.done()
}
