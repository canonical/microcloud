---
myst:
  html_meta:
    description: Release notes for MicroCloud 2.1.4, including highlights about new features, bugfixes, and other updates from the MicroCloud project.
---

(ref-release-notes-2.1.4)=
# MicroCloud 2.1.4 release notes

This is an {ref}`LTS release <ref-releases-microcloud-lts>` and contains bugfixes, package updates and new features.
It's the fifth release of the 2 LTS track.

(ref-release-notes-2.1.4-highlights)=
## Highlights

This section highlights new and improved features in this release.

### New home for docs

The integrated doc set was moved to [canonical.com/microcloud/docs](https://canonical.com/microcloud/docs/).
It is now located alongside MicroCloud's [product page](https://canonical.com/microcloud).

The changes were split across several PRs, see:

* https://github.com/canonical/microcloud/pull/1466
* https://github.com/canonical/microcloud/pull/1397
* https://github.com/canonical/microcloud/pull/1460

### Decommissioning guide

Adds a MicroCloud decommissioning guide, adapted from the [LXD decommissioning guide](https://github.com/canonical/lxd/issues/18507).
It's reorganized around decommissioning a full cluster, favors `microcloud` commands over `lxc` and adds MicroCloud-specific details such as MicroCeph logging,
OSD removal, Ceph monmap cleanup, and snap removal covering MicroCloud, LXD, MicroCeph and MicroOVN.

See https://github.com/canonical/microcloud/pull/1450.

### Improved resource naming in Terraform demo

Avoid hardcoding of variable names for the network bridge and support custom VM numbering with a matching enumeration for disk names.

See https://github.com/canonical/microcloud/pull/1428.

### Dynamic OVN northbound DB addresses

LXD [added support](https://github.com/canonical/lxd/pull/17874) to read the OVN northbound DB addresses directly from the `ovn.env`
file which is kept up to date by the MicroOVN snap.
This allows MicroCloud to omit setting the `network.ovn.northbound_connection` configuration option in LXD.
LXD can now react immediately on address changes made in MicroOVN.

Previously, MicroCloud only updated the `network.ovn.northbound_connection` configuration option during cluster initialization and
when adding additional members.

See https://github.com/canonical/microcloud/pull/1505.

### Preseed filter examples

When deploying MicroCloud clusters using preseed, optional storage filters can be provided to find specific disks to be used
for local and remote (Ceph) storage.
The respective how-to guides were extended to show practical usage examples.

See https://github.com/canonical/microcloud/pull/1501.

(ref-release-notes-2.1.4-bugfixes)=
## Bug fixes

The following bug fixes are included in this release.

### Hide the `--state-dir` flag from CLI

This flag is used to select the actual state directory of the MicroCloud snap.
The snap's command wrapper already takes care of setting this to the correct state directory.
A user never needs to set this flag manually.

See https://github.com/canonical/microcloud/pull/1503.

### Drop disk existence check during preseed

In preseed mode, all of the disks selected for remote storage were checked for existence on the initiator.
This does not work for disks which were selected on cluster members other than the initiator.

Wrong disk selections (without using filters) will cause the deployment to error out anyway.

See https://github.com/canonical/microcloud/pull/1496.

## Upgrading to the new version

If you are already using MicroCloud 2 LTS, refer to the {ref}`howto-update` guide for information on how to retrieve and use the latest release within the same major version.
If you are on an older major version, refer to the {ref}`howto-upgrade` guide.

(ref-release-notes-2.1.4-changelog)=
## Change log

View the [complete list of all changes in this release](https://github.com/canonical/microcloud/compare/2.1.3...2.1.4).
