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
Go test suite. It sets up a dedicated project directly through the LXD API: an "e2e-testing"
project, a profile, and a cached base image per instance type (container and virtual-machine).
Individual scenarios deploy and delete their own instances as needed.

MicroCloud's own built-in test scenarios all carry the Ginkgo label `builtin`.

## Prerequisites

- A Go toolchain.
- An LXD remote reachable to the already-deployed MicroCloud cluster to test.
- A testbed file describing how to reach/control cluster members (see ["Node control"](#node-control)).

### Setting up an LXD remote

Set the `REMOTE` environment variable to the name of the LXD remote corresponding to the MicroCloud
deployment to test (`mc` here).

If the LXD remote to test is not configured yet, it's possible to add it with an identity token
issued from a MicroCloud cluster member (`micro1` here):

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
The client credentials used during tests are not removed from the tested MicroCloud environment at
the end. This part of the cleanup needs to be done manually.
```

## Running the suite

Run the suite with the `ginkgo` CLI:

```sh
cd test/e2e && REMOTE=mc E2E_TESTBED_FILE=testbed.yaml go run github.com/onsi/ginkgo/v2/ginkgo -v --junit-report=report.xml .
```

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `REMOTE` | *(none, required)* | Name of the LXD remote corresponding to the MicroCloud deployment to test |
| `E2E_TESTBED_FILE` | *(none)* | Path to a YAML testbed file (lxd/ssh node control config), see ["Node control"](#node-control) |

## Node control

There are two built-in ways to reach a cluster member, resolved from a testbed file (see below) as
follows:

1. If the testbed file configures an `lxd` section, members are reached via local LXD daemon.
2. If the testbed file configures an `ssh` section, members are reached over SSH.
3. If the testbed file configures both `lxd` and `ssh`, the `lxd` section takes precedence and the
   `ssh` section is ignored.

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

Specs are tagged with Ginkgo labels, matching risk tiers (`risk-low`, `risk-medium`, `risk-high`).
MicroCloud's own built-in specs additionally carry the `builtin` label. Use `--label-filter` (via
the [`ginkgo` CLI](https://onsi.github.io/ginkgo/#the-ginkgo-cli)) to select a subset:

```sh
cd test/e2e && REMOTE=mc E2E_TESTBED_FILE=testbed.yaml go run github.com/onsi/ginkgo/v2/ginkgo -v --junit-report=report.xml --label-filter=risk-low .
```

## JUnit report

The example invocation above already includes `--junit-report=report.xml`, so a JUnit XML report will be generated upon completion (whether the tests pass or fail), consumable by most CI systems.
