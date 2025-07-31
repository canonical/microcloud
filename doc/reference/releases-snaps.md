(ref-releases-snaps)=
# Releases and snaps

(ref-releases-matrix)=
## Supported and compatible releases

```{table}
:align: center

| Ubuntu LTS | MicroCloud  | LXD        | MicroCeph       | MicroOVN |
| ---------- | ----------- | ---------- | --------------- | -------- |
| 22.04      | 2.1*        | 5.21*      | Squid           | 24.03    |
| 24.04      | 2.1*        | 5.21*      | Squid           | 24.03    |
```

`*` For MicroCloud and LXD, the most recent {ref}`feature releases <ref-releases-microcloud-feature>` (after the last LTS) are also supported.

The releases above are currently under standard support, meaning they receive bugfixes and security updates. An [Ubuntu Pro](https://ubuntu.com/pro) subscription can provide additional support.

The snaps for MicroCloud, LXD, MicroCeph, and MicroOVN must be installed on all members of the same MicroCloud cluster. The versions installed by each snap must be compatible with one another, and the same version of each snap must be installed on all cluster members.

Also see: {ref}`howto-update-upgrade` and {ref}`howto-snap`.

(ref-releases-microcloud)=
## MicroCloud releases

The MicroCloud team maintains both Long Term Support (LTS) and feature releases in parallel. Release notes are published on [Discourse](https://discourse.ubuntu.com/c/lxd/microcloud/145/).

(ref-releases-microcloud-lts)=
### LTS releases

LTS releases are **intended for production use**.

MicroCloud follows the [Ubuntu release cycle](https://ubuntu.com/about/release-cycle) cadence, meaning that an LTS release of MicroCloud is created every two years. The release names follow the format _x.y.z_, always including the point number _z_. Updates are provided through point releases, incrementing _z_.

(ref-releases-microcloud-lts-support)=
#### Support

LTS releases receive standard support for five years, meaning that it receives continuous updates according to the support levels described below. An [Ubuntu Pro](https://ubuntu.com/pro) subscription can provide additional support and extends the support duration by an additional five years.

Standard support for an LTS release starts at full support for its first two years, then moves to maintenance support for the remaining three years. Once an LTS reaches End of Life (EOL), it no longer receives any updates.

- **Full support**: Bugfixes and security updates are provided regularly.
- **Maintenance support**: High impact bugfixes and critical security updates are provided as needed.

The only currently supported MicroCloud LTS release is 2.1._z_. This LTS is supported until June 2029 and is currently in full support phase.

(ref-releases-microcloud-feature)=
### Feature releases

MicroCloud feature releases contain the newest features. Since they are less tested than LTS releases, they are **not recommended for production use**.

These releases follow the format _x.y_, and they never include a point number _z_. Feature releases for MicroCloud are numbered {{current_feature_track}}._y_, with _y_ incrementing for each new release.

Each feature release replaces the one before it, up to the next LTS release. After an LTS release, no feature release is considered to exist until the next feature release cycle begins, incrementing the major version number.

(ref-releases-microcloud-feature-support)=
#### Support

Feature releases receive continuous updates via each new release. The newest release at any given time is also eligible for additional support through an [Ubuntu Pro](https://ubuntu.com/pro) subscription.

Currently, since no feature release has been published since the last LTS, there is no supported feature release.

(ref-snaps-microcloud)=
## MicroCloud snap

MicroCloud is distributed as a [snap](https://snapcraft.io/docs). A key benefit of snap packaging is that it includes all required dependencies. This allows packages to run in a consistent environment on many different Linux distributions. Using the snap also streamlines updates through its [channels](https://snapcraft.io/docs/channels).

(ref-snaps-microcloud-channels)=
### Channels

Each installed snap follows a [channel](https://snapcraft.io/docs/channels). Channels include a {ref}`track <ref-snaps-microcloud-tracks>` and a {ref}`risk level <ref-snaps-microcloud-risk>` (for example, the {{current_feature_track}}/stable channel). Each channel points to one release at a time, and when a new release is published to a channel, it replaces the previous one. {ref}`Updating the snap <howto-update-upgrade>` then updates to that release.

To view all available channels for MicroCloud, run:

```bash
snap info microcloud
```

(ref-snaps-microcloud-tracks)=
### Tracks

MicroCloud releases are grouped under [snap tracks](https://snapcraft.io/docs/channels#heading--tracks).

The current feature track is {{current_feature_track}}, and the currently supported LTS track is {{current_lts_track}}. The `1` track reached {abbr}`EOL (End of Life)` at the end of April 2025.

(ref-snaps-microcloud-tracks-lts)=
#### LTS tracks

MicroCloud LTS tracks use the format _x.y_, corresponding to the major and minor numbers of {ref}`ref-releases-microcloud-lts`.

(ref-snaps-microcloud-track-feature)=
#### Feature track

The MicroCloud feature track uses the major number of the current {ref}`feature release <ref-releases-microcloud-feature>` series. Feature releases within the same major version are published to the same track, replacing the previous release. This simplifies updates, as you don't need to switch channels to access new feature releases within the same major version.

The current feature track is {{current_feature_track}}. No feature release has yet been published to this track. The most recent development updates can be found in the {{current_feature_track}}/`edge` channel, for testing purposes only.

(ref-snaps-microcloud-track-default)=
#### The default track

If you {ref}`install the MicroCloud snap <installing-snap-package>` without specifying a track, the recommended default is used. The default track always points to the most recent LTS track, which is currently {{current_lts_track}}.

(ref-snaps-microcloud-risk)=
### Risk levels

For each MicroCloud track, there are three [risk levels](https://snapcraft.io/docs/channels#heading--risk-levels): `stable`, `candidate`, and `edge`.

We recommend that you use the `stable` risk level to install fully tested releases; this is the only risk level supported under [Ubuntu Pro](https://ubuntu.com/pro), as well as the default risk level if one is not specified at install. The `candidate` and `edge` levels offer newer but less-tested updates, posing higher risk.

(ref-releases-snaps-components)=
## For MicroCloud components

(ref-releases-snaps-lxd)=
### LXD

LXD follows a similar approach to its releases and snap as MicroCloud, including an LTS release every two years and more frequent feature releases on a feature track. For details, see {ref}`LXD releases and snap <lxd:ref-releases-snap>`.

(ref-releases-snaps-microceph)=
### MicroCeph

[Ceph](https://ceph.io) is the upstream project of {doc}`microceph:index`.

The version of Ceph initially included in the release of an LTS version of Ubuntu is supported for the entire lifecycle of that Ubuntu version. This support applies to MicroCeph as well.

MicroCeph typically does not publish feature releases, but provides periodic non-breaking updates to existing releases, along with a new stable release corresponding to each Ceph release series. These MicroCeph releases share their upstream's release names (such as `quincy` or `squid`).

For details about MicroCeph, see {doc}`microceph:index`. For more information about the Ceph release cycle, visit the Ceph documentation: {ref}`ceph:ceph-releases-general`.

(ref-releases-snaps-microovn)=
### MicroOVN releases and snap

The upstream [OVN](https://www.ovn.org/) project follows a six-month release cadence. Every year, they release `YY.03` in March, and `YY.09` in September.

Every two years, the March version of OVN becomes an LTS version, such as the `24.03` version released in March of 2024. MicroOVN publishes versions that correspond to these upstream LTS versions. Stable maintenance is provided through upstream point releases for OVN and bugfixes for the snap deployment.

For more information, see the MicroOVN documentation:

- {doc}`microovn:developers/release-process`
- {ref}`microovn:snap channels`

(ref-snaps-updates)=
## Updates

By default, installed snaps update automatically when new releases are published to the channel they're tracking. [Progressive snap releases](https://documentation.ubuntu.com/snapcraft/stable/how-to/publishing/manage-revisions-and-releases/#deliver-a-progressive-release) also mean that updates might not be immediately available to all machines at the same time.

With MicroCloud, this can be problematic because its component snaps must always use {ref}`compatible versions <ref-releases-matrix>`, and because all members of a cluster must use the same version of each snap.

To prevent issues, {ref}`hold updates for MicroCloud and its components <howto-update-hold>`. Furthermore, ensure that the all snaps are set to `in-cohort` (see {ref}`howto-update-sync`).

## Related topics

How-to guides:

- {ref}`howto-support`
- {ref}`howto-install`
- {ref}`howto-snap`
- {ref}`howto-update-upgrade`
