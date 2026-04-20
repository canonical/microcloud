---
myst:
  html_meta:
    description: Release notes for MicroCloud 2.1.3, including highlights about new features, bugfixes, and other updates from the MicroCloud project.
---

(ref-release-notes-2.1.3)=
# MicroCloud 2.1.3 release notes

This is an {ref}`LTS release <ref-releases-microcloud-lts>` and contains bugfixes, package updates and new features.
It's the fourth release of the 2 LTS track.

(ref-release-notes-2.1.3-highlights)=
## Highlights

This section highlights new and improved features in this release.

### Temporary UI access link

It's fast to deploy a MicroCloud and gain the ability to spawn resources from the command line.
To provide the same experience when {ref}`using the UI <howto-ui>`, we added a new question to the end of the
interactive `microcloud init` questionnaire:

`Would you like to create an initial UI access link? (yes/no) [default=yes]:`

If you answer `yes` (default), MicroCloud creates a temporary UI access link that allows you to immediately enter
the LXD UI after MicroCloud is installed.
The temporary access lasts for 24 hours and is intended to allow you to perform further setup (such as configuring permanent access).

This requires a 6+ version of LXD, which is currently a feature track (non-LTS).

### Documentation changes

When viewing the docs, the version selector now displays `latest` and `default` versions.
The `latest` version returns the documentation for the current development version of MicroCloud (currently MicroCloud 3), and the `default` version points to the latest LTS version (currently MicroCloud 2).
When MicroCloud version 3 becomes the next LTS, the `default` version will be updated accordingly.
This ensures existing links will stay intact.

In addition, the version number is now appended to the documentation site title.

### E2E test report

When running the e2e test suite, the `--report` flag can be added to generate an HTML report with test results:

`e2e/run --report`

The report gets saved to file `$(date +%Y%m%d%H%M%S)-e2e-run.html` based on the current time.

### Respect RBD features from the Ceph cluster 

If the LXD version in use supports the `storage_ceph_use_rbd_defaults` API extension, MicroCloud no longer sets the RBD features
`layering,striping,exclusive-lock,object-map,fast-diff,deep-flatten` on the created LXD `remote` Ceph RBD storage pool.
Instead the cluster's features are used when creating new volumes.

### LXD asynchronous endpoint changes

In the upcoming 6.8 feature release of LXD, some endpoints are changed for asynchronous use. This release of MicroCloud is updated to accommodate these changes.

(ref-release-notes-2.1.3-bugfixes)=
## Bug fixes

The following bug fixes are included in this release.

### Preseed daemon readiness

A race condition was observed in the `microcloud preseed` command that could occur when executed immediately after snap installation.
An additional daemon readiness check was added before proceeding with the actual preseed operations.

### Join token expiry extension

When a MicroCloud gets created with a large number of resources (such as many Ceph OSDs), the MicroCloud installation might take longer
than the lifetime of the underlying join tokens.
Those join tokens are created at the beginning of the MicroCloud installation and are consumed member by member by each of the components (MicroCloud, MicroCeph, MicroOVN and LXD).

By default, these tokens are created with a lifetime of five minutes, which can be too short for larger deployments or slower environments. 
To avoid tokens expiring before they can be used, we have extended the default lifetime to one hour.

## Upgrading to the new version

If you are already using MicroCloud 2 LTS, refer to the {ref}`howto-update` guide for information on how to retrieve and use the latest release within the same major version.
If you are on an older major version, refer to the {ref}`howto-upgrade` guide.

(ref-release-notes-2.1.3-changelog)=
## Change log

View the [complete list of all changes in this release](https://github.com/canonical/microcloud/compare/2.1.2...2.1.3).
