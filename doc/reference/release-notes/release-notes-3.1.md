---
myst:
  html_meta:
    description: Release notes for MicroCloud 3.1, including highlights about new features, bugfixes, and other updates from the MicroCloud project.
---

(ref-release-notes-3.1)=
# MicroCloud 3.1 release notes

This is a {ref}`feature release <ref-releases-microcloud-feature>` and is not recommended for production use.
It's the first feature release of the new track 3.

(ref-release-notes-3.1-highlights)=
## Highlights

This section highlights new and improved features in this release.

### Cluster Manager

Cluster Manager allows you to manage and monitor multiple MicroCloud clusters in a single application.
Starting with this release, a MicroCloud can be connected to an existing deployment of Cluster Manager.

Check {ref}`howto-cluster-manager` to deploy Cluster Manager.
The {ref}`howto-cluster-manager-requirements` section lists the requirements to get started.
See the {ref}`ref-cluster-manager-architecture` page for an in-depth view of the core components.

If you already have a Cluster Manager deployment, see {ref}`howto-cluster-manager-enroll` to learn
how to enroll a MicroCloud cluster.

### Temporary UI access link

It's fast to deploy a MicroCloud and gain the ability to spawn resources from the command line.
To provide the same experience when {ref}`using the UI <howto-ui>`, we added a new question to the end of the
interactive `microcloud init` questionnaire:

`Would you like to create an initial UI access link? (yes/no) [default=yes]:`

If you answer `yes` (default), MicroCloud creates a temporary UI access link that allows you to immediately enter
the LXD UI after MicroCloud is installed.
The temporary access lasts for 24 hours and is intended to allow you to perform further setup (such as configuring permanent access).

This requires a 6+ version of LXD, which is currently a feature track (non-LTS).

### MicroOVN 26.03 support (experimental)

MicroCloud 3.1 can use MicroOVN 26.03, which is a feature release currently available through their `latest/edge` channel. Using this instead of the MicroOVN 24.03 LTS release (snap channel `24.03/stable`) allows us to test the latest features before the 26.03 release is published as the next LTS.

Make sure you have the `core26` snap installed before trying to install MicroOVN from `latest/edge`:

```bash
snap install core26 --channel="latest/edge"
snap install microovn --channel="latest/edge" --cohort="+"
```

## Upgrading to the new version

If you are currently running MicroCloud 2 LTS, be aware that upgrading to track 3 is not yet recommended or supported for production clusters.
See the {ref}`howto-upgrade-microcloud` guide for information on how to switch the MicroCloud track.

(ref-release-notes-3.1-changelog)=
## Change log

View the [complete list of all changes in this release](https://github.com/canonical/microcloud/compare/2.1.0...3.1).
