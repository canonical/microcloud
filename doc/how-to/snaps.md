(howto-snap)=
# How to manage the snaps

Manage MicroCloud and its components (LXD, MicroCeph, and MicroOVN) through their snap packages.

For the installation guide, see: {ref}`howto-install`. For details about the snaps, including {ref}`supported and compatible releases <ref-releases-matrix>`, {ref}`tracks <ref-snaps-microcloud-tracks>`, and {ref}`release processes <ref-releases-microcloud>`, see: {ref}`ref-releases-snaps`.

(howto-snap-info)=
## View snap information

To view information about a snap, including the available channels and installed version, run:

```bash
snap info <microcloud|lxd|microceph|microovn>
```

To view information about the installed version only, run:

```bash
snap list <microcloud|lxd|microceph|microovn>
```

Sample output:

```{terminal}
:input: snap list microcloud
:user: root
:host: instance

Name        Version        Rev   Tracking  Publisher   Notes
microcloud  2.1.0-3e8b183  1144  2/stable  canonical✓  in-cohort,held
```

The first part of the version string corresponds to the release (in this sample, `2.1.0`).

(howto-snap-daemon)=
## Manage the MicroCloud daemon

Installing the MicroCloud snap creates the MicroCloud daemon as a [snap service](https://snapcraft.io/docs/how-to-guides/manage-snaps/control-services/). Use the following `snap` commands to manage this daemon.

To view the status of the daemon, run:

```bash
snap services microcloud
```

To stop the daemon, run:

```bash
sudo snap stop microcloud
```

To start the daemon, run:

```bash
sudo snap start microcloud
```

To restart the daemon, run:

```bash
sudo snap restart microcloud
```

For more information about managing snap services, visit [Control services](https://snapcraft.io/docs/how-to-guides/manage-snaps/control-services/) in the Snap documentation.

## Related topics

How-to guides:
- {ref}`howto-update-upgrade`
- {ref}`howto-install`

Reference:
- {ref}`ref-releases-snaps`

In the LXD documentation:

- {ref}`lxd:howto-snap`
