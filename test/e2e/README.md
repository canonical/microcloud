# End to end testing of MicroCloud

Once MicroCloud is deployed, end to end testing can verify that everything works as it should.

```{caution}
By running end to end tests against a MicroCloud, you acknowledge that:

1. All instances will be impacted as the cluster member evacuations require them
   to be migrated (and restarted if not live-migrated).
1. Resources will be consumed (instances, memory, CPU, disk space, IP addresses,
   etc).

As such, these end to end tests are best used to ascertain the working condition
of a MicroCloud deployment before deploying production workloads to it.
```

## Overview

This suite is a [Ginkgo](https://onsi.github.io/ginkgo/)/[Gomega](https://onsi.github.io/gomega/)
Go test suite. It sets up a dedicated project using the
[Terraform provider for LXD](https://github.com/terraform-lxd/terraform-provider-lxd)
(through Terratest) and deploys a number of instances on the MicroCloud. Those
instances will be a mix of containers and virtual machines.

`test/e2e` is its own Go module (see [`go.mod`](go.mod)), kept isolated from
the root `github.com/canonical/microcloud/microcloud` module so that
test-only dependencies never leak into the production build.

The module is split into two importable packages so other repositories can
reuse it to write their own specs (see
["Consuming this suite from another repo"](#consuming-this-suite-from-another-repo)):

- [`framework`](framework): A library for writing e2e; no test cases live here.
- [`scenarios`](scenarios): MicroCloud's own built-in test scenarios; every spec in this package carries the Ginkgo label `builtin`.

The root of the module ([`e2e_suite_test.go`](e2e_suite_test.go)) is itself
just a thin harness wiring those two packages together through Ginkgo's
`RunSpecs`/`SynchronizedBeforeSuite`/`SynchronizedAfterSuite`.

## Prerequisites

- A Go toolchain and `terraform` installed.
- An LXD remote reachable to the already-deployed MicroCloud cluster to test.
- A testbed file describing how to reach/control cluster members (see ["Node control"](#node-control)).

### Setting up an LXD remote

Set the `REMOTE` environment variable to the name of the LXD remote corresponding to the MicroCloud deployment to test (`mc` here).

If the LXD remote to test is not configured yet, it's possible to add it with an identity token issued from a MicroCloud cluster member (`micro1` here):

```sh
# Create the needed group
root@micro1:~# lxc auth group create e2e-testing
Group e2e-testing created

# Grant admin permission on server
root@micro1:~# lxc auth group permission add e2e-testing server admin

# Issue the identity token
root@micro1:~# lxc auth identity create tls/e2e-testing
TLS identity "tls/e2e-testing" (d16614b8-f39e-4ef5-8e9b-90764b61b660) pending identity token:
eyJjbGllbnRfbmFtZSI6ImUyZS10ZXN0aW5nIiwiZmluZ2VycHJpbnQiOiIyNzY1OGNjYmRlNmRkZDcwNDliMzliNmY5NzA1MWIzMWVkMWQ0NDM2OTFjZGFjMWIxNmI1ODBjNmI4NzFiNmM2IiwiYWRkcmVzc2VzIjpbIjE3Mi4yNC4yNi4xODo4NDQzIiwiWzIwMDE6NDcwOmIxYzM6Nzk0Njo4NWYzOmQzYTg6ZDcxZTo3YjA1XTo4NDQzIl0sInNlY3JldCI6ImJjNjlmOTM4YzM5NjRhNTI1ZDI3OTlmMTEyM2Q2NWE5ZGExZDE1ZGFmNjVjZjlmNTViYjQ1ZDdmZjBiMjg1YmYiLCJleHBpcmVzX2F0IjoiMDAwMS0wMS0wMVQwMDowMDowMFoiLCJ0eXBlIjoiQ2xpZW50IGNlcnRpZmljYXRlIn0=

# Add the freshly created identity to the freshly created group
root@micro1:~# lxc auth identity group add tls/e2e-testing e2e-testing
```

This token can be used on the machine where the suite will be run:

```sh
lxc remote add mc https://microcloud.lxd:8443 --token eyJjbGllbnRfbmFtZSI6ImUyZS10ZXN0aW5nIiwiZmluZ2VycHJpbnQiOiIyNzY1OGNjYmRlNmRkZDcwNDliMzliNmY5NzA1MWIzMWVkMWQ0NDM2OTFjZGFjMWIxNmI1ODBjNmI4NzFiNmM2IiwiYWRkcmVzc2VzIjpbIjE3Mi4yNC4yNi4xODo4NDQzIiwiWzIwMDE6NDcwOmIxYzM6Nzk0Njo4NWYzOmQzYTg6ZDcxZTo3YjA1XTo4NDQzIl0sInNlY3JldCI6ImJjNjlmOTM4YzM5NjRhNTI1ZDI3OTlmMTEyM2Q2NWE5ZGExZDE1ZGFmNjVjZjlmNTViYjQ1ZDdmZjBiMjg1YmYiLCJleHBpcmVzX2F0IjoiMDAwMS0wMS0wMVQwMDowMDowMFoiLCJ0eXBlIjoiQ2xpZW50IGNlcnRpZmljYXRlIn0=

REMOTE=mc ginkgo -v .
```

```{important}
The client credentials used during tests are not removed from the tested MicroCloud environment at the end. This part of the cleanup needs to be done manually.
```

## Running the suite

Run the suite with `make`:

```sh
REMOTE=mc E2E_TESTBED_FILE=testbed.yaml make e2e

# equivalent to:
REMOTE=mc E2E_TESTBED_FILE=testbed.yaml go run github.com/onsi/ginkgo/v2/ginkgo -v --junit-report=report.xml .
```

`go test` also works (e.g. in contexts where only the standard toolchain is available), but lacks `ginkgo`'s parallelism/label-filtering/report-aggregation support:

```sh
REMOTE=mc go test -timeout 0 -v .
```

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `REMOTE` | *(none, required)* | Name of the LXD remote corresponding to the MicroCloud deployment to test |
| `E2E_DESTROY` | `yes` | Set to `no` to leave the Terraform-managed testing environment in place after the run, for inspection |
| `E2E_EVACUATION_COUNTS` | `1` | Number of full evacuate/reboot/restore cycles to run across every cluster member; `0` disables the evacuation test |
| `E2E_TESTBED_FILE` | *(none)* | Path to a YAML testbed file (lxd/ssh node control config), see ["Node control"](#node-control) |

```sh
# don't do any rolling reboot/evacuation test
E2E_EVACUATION_COUNTS=0 REMOTE=mc E2E_TESTBED_FILE=testbed.yaml make e2e

# do multiple rolling reboots/evacuation tests:
E2E_EVACUATION_COUNTS=5 REMOTE=mc E2E_TESTBED_FILE=testbed.yaml make e2e
```

It is possible to override the variables defined in [`variables.tf`](variables.tf) by creating a file named `terraform.tfvars`:

```sh
# terraform.tfvars
vms_per_host = 5
containers_per_host = 25
```

## Node control

A `NodeController` (see [`framework/controller.go`](framework/controller.go)) abstracts how to reach and control a cluster member's underlying host — used by the suite to health-check a cluster member before any spec runs and to reboot a member during the evacuation test, independently of how the cluster's members are hosted (local LXD VMs, physical hardware, MAAS, ...). There are two built-in implementations, resolved from a testbed file (see below) as follows:

1. If the testbed file configures an `lxd` section, an `LXDController` is used, erroring out if its `project` doesn't actually match every cluster member to a local LXD instance of the same name.
2. Else, if it configures an `ssh` section, an `SSHController` is used, erroring out if any configured node isn't reachable over SSH.
3. Else, the suite fails fast with an explicit error rather than silently skipping health checks or only failing once a reboot is attempted: a testbed file is always required, there is no fallback.

Both implementations run the same checks/sequence: `microcloud status` (must report `Status: HEALTHY`), `microceph status`, `ceph -s` (must report `HEALTH_OK`), and `microovn status` (must report OVN Northbound/Southbound `OK`) on the first cluster member, then `systemctl status --failed` on every member — stopping at the first failing check. Its output is attached to the Ginkgo report (e.g. visible in `--junit-report`'s output) whether the health check passed or failed partway through. Rebooting waits for the member to report a running process, for a command to succeed again, and for `systemctl`/`microcloud` to report ready. Both assume passwordless `sudo` on each member.

### Testbed file

A YAML file (see [`testbed.example.yaml`](testbed.example.yaml)) configuring how to reach/control cluster members.

```yaml
# Either lxd or ssh. If both lxd and ssh exist, lxd always takes precedence.

lxd:
  project: my-project

ssh:
  nodes:
    micro1:
      address: 10.0.0.11
      # port: 22  # optional, defaults to 22
      user: ubuntu
      private_key_file: ~/.ssh/id_ed25519
    micro2:
      address: 10.0.0.12
      user: ubuntu
      private_key_file: ~/.ssh/id_ed25519
```

Point the suite at it with the `E2E_TESTBED_FILE` environment variable. It's a required environment variable.

## Filtering scenarios

Specs are tagged with Ginkgo labels, matching risk tiers (`risk-low`, `risk-medium`, `risk-high`). MicroCloud's own built-in specs (in [`scenarios`](scenarios)) additionally carry the `builtin` label. Use `--label-filter` (via the [`ginkgo` CLI](https://onsi.github.io/ginkgo/#the-ginkgo-cli)) to select a subset:

Extra `ginkgo` arguments (e.g. `--label-filter`) can be forwarded via `ARGS`:

```sh
REMOTE=mc E2E_TESTBED_FILE=testbed.yaml ARGS=--label-filter=risk-low make e2e
```

## Consuming this suite from another repo

`framework` and `scenarios` are regular Go packages (not `_test.go` files), so they can be imported from any other module that wants to write and run its own MicroCloud e2e specs alongside MicroCloud's own built-in ones.

To extend the test scenarios and add test scenarios that are unique to your own deployment, add a dependency on this module, then write a harness that mirrors [`e2e_suite_test.go`](e2e_suite_test.go):

```go
package e2e_test

import (
	"testing"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework"
	_ "github.com/canonical/microcloud/microcloud/test/e2e/scenarios" // MicroCloud's own built-in specs
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "My Custom MicroCloud E2E Suite")
}

var _ = SynchronizedBeforeSuite(framework.SetupSuite, framework.AttachToSuite)
var _ = SynchronizedAfterSuite(framework.TeardownProcess, framework.TeardownSuite)

// Add your own spec here
var _ = Describe("My custom scenario", Label("risk-low"), func() {
	It("does something specific to my deployment", func() {
		names, err := framework.GetInstanceNames()
		Expect(err).NotTo(HaveOccurred())
		// ...
	})
})
```

## JUnit report

The `make e2e` already included `--junit-report=report.xml`, so a JUnit XML report will generated upon completion (whether the tests pass or fail), consumable by most CI systems.
