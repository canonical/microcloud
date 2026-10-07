---
myst:
  html_meta:
    description: Release notes for MicroCloud 3.4.0, including highlights about new features, bugfixes, and other updates from the MicroCloud project.
---

(ref-release-notes-3.4.0)=
# MicroCloud 3.4.0 release notes

This is an {ref}`LTS release <ref-releases-microcloud-lts>` and is recommended for production use.
It's the first LTS release of track 3, succeeding the 2.1._z_ LTS track and superseding the 3.1, 3.2, and 3.3 feature releases.

(ref-release-notes-3.4.0-highlights)=
## Highlights

This section highlights new and improved features in this release.

### Cluster Manager improvements

{ref}`Cluster Manager <howto-cluster-manager>` was introduced in MicroCloud 3.1 to let you manage and monitor multiple MicroCloud clusters from a single application.
This release adds further integration between MicroCloud and Cluster Manager:

* The LXD URL reported to Cluster Manager can now be overridden, instead of always using the auto-detected address. This is useful when the LXD UI is reachable through a different address than the one MicroCloud would otherwise advertise (for example, behind a reverse proxy or load balancer).

  ```bash
  microcloud cluster-manager set lxd_url https://example.com:8443
  ```

  See https://github.com/canonical/microcloud/pull/1539.

* The cluster's UUID is now included in the heartbeat status payload sent to Cluster Manager, making it easier to correlate a MicroCloud cluster with its representation in Cluster Manager.

  See https://github.com/canonical/microcloud/pull/1557.

### Observability integration via Juju charm

MicroCloud can now be observed through the [Canonical Observability Stack (COS)](https://charmhub.io/topics/canonical-observability-stack) using the `microcloud` Juju charm.
The charm attaches to an already-deployed MicroCloud cluster and collects metrics from LXD, MicroCeph, and MicroOVN on each member, which can then be related to COS for visualization in Grafana.

See {ref}`howto-charm-metrics`.

### Snap base updated to `core26`

The MicroCloud snap now builds on the `core26` base instead of `core24`.

See https://github.com/canonical/microcloud/pull/1574.

(ref-release-notes-3.4.0-incompatible)=
## Backwards-incompatible changes

These changes are not compatible with older versions of MicroCloud.

### Minimum system requirement changes

The minimum supported version of some components has changed:

* Go: 1.27.1 (used to build MicroCloud)
* snapd: 2.75 (required by the `core26` base)

## Upgrading to the new version

If you are currently running MicroCloud 2 LTS, this release is the recommended target for production upgrades to track 3.
See the {ref}`howto-upgrade` guide for information on how to switch the MicroCloud track.

If you are currently using an earlier 3.1, 3.2, or 3.3 feature release, refer to the {ref}`howto-update` guide for information on how to move onto the 3.4.0 LTS version.

(ref-release-notes-3.4.0-changelog)=
## Change log

View the [complete list of all changes in this release](https://github.com/canonical/microcloud/compare/3.3...3.4.0).
</content>
