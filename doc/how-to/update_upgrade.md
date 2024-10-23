(howto-update-upgrade)=
# How to update and upgrade

MicroCloud is made of several snaps that are closely coupled with each other.
The cluster members that are part of the MicroCloud deployment must always run the same version of the snaps.
This means that when the snaps on one of the cluster members are refreshed, they must also be refreshed on all other cluster members before MicroCloud is operational again.
See {ref}`howto-update-upgrade-deps` for the recommended order.

Before performing an update or upgrade make sure to backup your data to prevent any data loss in case of failure.
See the following backup guides for each of the snaps:

* {doc}`How to backup MicroCeph <microceph:explanation/taking-snapshots>`
* {ref}`How to backup LXD <lxd:backups>`

In case of error see {ref}`howto-recover` for troubleshooting details.

(howto-update-upgrade-update)=
## Update MicroCloud

Updating MicroCloud allows getting the latest set of features and fixes in the tracked channels for the various snaps.
Performing an update requires going through the list of snaps one after another and update each of the individual cluster members.

```{note}
Depending on which snap gets updated, some services of this snap (e.g. API endpoints) might not be available whilst performing the update.
Check the respective services documentation on updates for more information.
```

During an update the snaps channel won't be modified so the snaps get updated to the last available version inside of the current channel.
This does not introduce breaking changes and can be used on a regular basis to update the MicroCloud.

As MicroCloud consumes the services offered by the dependant snaps (MicroCeph, MicroOVN and LXD), the update procedure starts by updating
the list of dependencies first.

(howto-update-upgrade-deps)=
### Update dependency snaps

Updating the dependencies can be done by running `snap refresh` against the respective snap.
For MicroCloud automatic snap refreshes are put on hold. See {ref}`howto-snap-control-updates` for more information.

To start the update procedure, enter the following command on the first machine:

    sudo snap refresh microceph --cohort="+"

If the command has succeeded, run the same command on the next machine and so on.

```{note}
Make sure to validate the health of the recently updated dependency before continuing with the next one.
```

After successfully updating MicroCeph continue with MicroOVN.
Again enter the following command on the first machine:

    sudo snap refresh microovn --cohort="+"

As before run the command on all the other machines one after another if you don't observe any errors.
Next we can continue to update LXD.

The refresh will block until each of the LXD cluster members is updated so make sure to perform the following
command on all machines in parallel:

    sudo snap refresh lxd --cohort="+"

### Update MicroCloud snap

Last but not least we can update MicroCloud.
As before enter the following command on the first machine:

    sudo snap refresh microcloud --cohort="+"

Continue running the command on the rest of the machines to finish the update.
You can confirm a healthy state of the MicroCloud after the update by running the following command:

    sudo microcloud status

```{note}
The status command was introduced in MicroCloud version 2.
See {ref}`howto-update-upgrade-upgrade` on how to upgrade to another track.
```

(howto-update-upgrade-upgrade)=
## Upgrade MicroCloud

Upgrading MicroCloud allows switching to another track with major improvements and enhanced functionality.
Performing an upgrade requires going through the list of snaps one after another and upgrade each of the individual cluster members.

```{note}
Depending on which snap gets upgraded, some services of this snap (e.g. API endpoints) might not be available whilst performing the upgrade.
Check the respective services documentation on upgrades for more information.
```

During an upgrade the snaps channel will be switched to another track.
This might introduce breaking changes for MicroCloud and its dependencies and should be done with care.
See {ref}`howto-update-upgrade-update` for regular non-breaking updates.

As MicroCloud consumes the services offered by the dependant snaps (MicroCeph, MicroOVN and LXD), the update procedure starts by updating
the list of dependencies first.

### Upgrade dependency snaps

Upgrading the dependencies can be done by running `snap refresh --channel <new track/stable>` against the respective snap.

Make sure to consult the dedicated upgrade guides of each dependency before you perform the actual upgrade:

* {doc}`How to upgrade MicroCeph <microceph:how-to/reef-upgrade>`
* {doc}`How to upgrade MicroOVN <microovn:how-to/major-upgrades>`
* {doc}`How to upgrade LXD <lxd:howto/cluster_manage>`

To start the upgrade procedure, enter the following command on the first machine:

    sudo snap refresh microceph --channel "squid/stable" --cohort="+"

If the command has succeeded, run the same command on the next machine and so on.

```{note}
Make sure to validate the health of the recently upgraded dependency before continuing with the next one.
```

After successfully upgrading MicroCeph continue with MicroOVN.
Again enter the following command on the first machine:

    sudo snap refresh microovn --channel "24.03/stable" --cohort="+"

As before run the command on all the other machines one after another if you don't observe any errors.
Next we can continue to upgrade LXD.

The installer will block until each of the LXD cluster members is upgraded so make sure to perform the following
command on all machines in parallel:

    sudo snap refresh lxd --channel "5.21/stable" --cohort="+"

### Upgrade MicroCloud snap

Last but not least we can upgrade MicroCloud.
As before enter the following command on the first machine:

    sudo snap refresh microcloud --channel "2/stable" --cohort="+"

Continue running the command on the rest of the machines to finish the upgrade.
You can confirm a healthy state of the MicroCloud after the upgrade by running the following command:

    sudo microcloud status
