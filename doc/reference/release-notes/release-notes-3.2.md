---
myst:
  html_meta:
    description: Release notes for MicroCloud 3.2, including highlights about new features, bugfixes, and other updates from the MicroCloud project.
---

(ref-release-notes-3.2)=
# MicroCloud 3.2 release notes

This is a {ref}`feature release <ref-releases-microcloud-feature>` and is not recommended for production use.
It's the second feature release of track 3.

(ref-release-notes-3.2-highlights)=
## Highlights

This section highlights new and improved features in this release.

### LXD 6.8 support

{ref}`LXD 6.8 <lxd:ref-release-notes-6.8>` changed many of its API endpoints to be asynchronous (such as storage and network creation).
MicroCloud 3.2 accommodates those changes when creating resources in LXD.

## Upgrading to the new version

If you are currently running MicroCloud 2 LTS, be aware that upgrading to track 3 is not yet recommended or supported for production clusters.
See the {ref}`howto-upgrade` guide for information on how to switch the MicroCloud track.

If you are currently using an earlier release of MicroCloud 3, refer to the {ref}`howto-update` guide for information on how to retrieve and use the latest release on the same track.

(ref-release-notes-3.2-changelog)=
## Change log

View the [complete list of all changes in this release](https://github.com/canonical/microcloud/compare/3.1...3.2).
