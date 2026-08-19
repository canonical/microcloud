---
myst:
  html_meta:
    description: Release notes for MicroCloud 3.3, including highlights about new features, bugfixes, and other updates from the MicroCloud project.
---

(ref-release-notes-3.3)=
# MicroCloud 3.3 release notes

This is a {ref}`feature release <ref-releases-microcloud-feature>` and is not recommended for production use.
It's the third feature release of track 3.

(ref-release-notes-3.3-highlights)=
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

### Cluster manager reverse tunnel

The cluster manager is now capable of triggering a defined set of actions through a secure reverse tunnel opened up by MicroCloud.
For each MicroCloud cluster you can enable the tunnel to be opened up with the already configured cluster manager.
By default the tunnel is deactivated. You can activate it with:

```bash
microcloud cluster-manager set reverse_tunnel true
```

See https://github.com/canonical/microcloud/pull/801.

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

### Preview support of MicroCeph tentacle

MicroCloud allows the creation of new clusters using MicroCeph's `tentacle/*` snap channels to try out the latest features.
This is not yet intended for production use. MicroCloud prints a warning to indicate the use of a non-LTS component.

See https://github.com/canonical/microcloud/pull/1338.

(ref-release-notes-3.3-bugfixes)=
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

If you are currently running MicroCloud 2 LTS, be aware that upgrading to track 3 is not yet recommended or supported for production clusters.
See the {ref}`howto-upgrade` guide for information on how to switch the MicroCloud track.

If you are currently using an earlier release of MicroCloud 3, refer to the {ref}`howto-update` guide for information on how to retrieve and use the latest release on the same track.

(ref-release-notes-3.3-changelog)=
## Change log

View the [complete list of all changes in this release](https://github.com/canonical/microcloud/compare/3.2...3.3).
