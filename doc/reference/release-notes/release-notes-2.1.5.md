---
myst:
  html_meta:
    description: Release notes for MicroCloud 2.1.5, including highlights about new features, bugfixes, and other updates from the MicroCloud project.
---

(ref-release-notes-2.1.5)=
# MicroCloud 2.1.5 release notes

This is an {ref}`LTS release <ref-releases-microcloud-lts>` and contains bugfixes, package updates and new features.
It's the sixth release of the 2 LTS track.

(ref-release-notes-2.1.5-highlights)=
## Highlights

This section highlights new and improved features in this release.

### Dependency updates

MicroCloud is rebuilt against the latest LXD and MicroCeph releases, and its Go module dependencies are refreshed to pick up upstream fixes, including security fixes for known Go CVEs.

(ref-release-notes-2.1.5-incompatible)=
## Backwards-incompatible changes

These changes are not compatible with older versions of MicroCloud.

### Minimum system requirement changes

The minimum supported version of some components has changed:

* Go: 1.27.2 (used to build MicroCloud)

## Upgrading to the new version

If you are already using MicroCloud 2 LTS, refer to the {ref}`howto-update` guide for information on how to retrieve and use the latest release within the same major version.
If you are on an older major version, refer to the {ref}`howto-upgrade` guide.

(ref-release-notes-2.1.5-changelog)=
## Change log

View the [complete list of all changes in this release](https://github.com/canonical/microcloud/compare/2.1.4...2.1.5).
