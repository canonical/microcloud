(howto-update-upgrade)=
# How to update and upgrade

MicroCloud is made of several snaps that are closely coupled with each other.
The cluster members that are part of a MicroCloud deployment must always run the same version of the snaps. Thus, when the snaps on one of the cluster members are refreshed, they must also be refreshed on all other cluster members. Until this happens, MicroCloud will continue to function as normal in regards to its data plane. However, its control plane will be inoperable, meaning that its configuration cannot be updated. For example, you will not be able to add or remove cluster members or instances.

See {ref}`howto-update-upgrade-deps` for the recommended order.

Before performing an update or upgrade, make sure to backup your data to prevent any data loss in case of failure.
See the following backup guides for each of the snaps:

* {doc}`How to backup MicroCeph <microceph:explanation/taking-snapshots>`
* {ref}`How to backup LXD <lxd:backups>`

In case of error, see {ref}`howto-recover` for troubleshooting details.

(howto-update-upgrade-running-instances)=
## Keep instances running during an update or upgrade

You can update or upgrade MicroCloud and its dependency snaps on a cluster member while it is hosting running instances. 

For the LXD snap, this won't affect the running instances. However, for the MicroCeph or MicroOVN snaps, storage and network services might be briefly affected. For example, when MicroCeph is updated on a cluster member, instances on that machine might experience temporary packet loss.

To avoid such possible effects entirely, use the live migration approach described below when updating or upgrading the MicroCeph and MicroOVN snaps.

- Use virtual machines (VMs) instead of system containers for crucial workloads; in general, containers cannot be live-migrated.
- Each VM must be pre-configured for live migration. See: {ref}`lxd:live-migration-vms` for information on the required configurations.
- Before you update or upgrade a cluster member, use the {ref}`cluster evacuate <lxd:cluster-evacuate>` operation to migrate all instances on the host to other members in the same cluster.
- Once the update or upgrade is complete, use the {ref}`cluster restore <lxd:cluster-restore>` operation to migrate all evacuated instances back to the original host.
- The evacuate and restore operations can live-migrate any VMs that are configured to allow it. If any instances on the cluster member are ineligible for live migration (such as a container, or a VM that is not configured for live migration), then during both evacuation and restoration, those instances are stopped, migrated, and restarted.

For more information on the cluster evacuate and restore operations, see: {ref}`lxd:cluster-evacuate-restore`.

(howto-update-upgrade-update)=
## Update MicroCloud

Updating MicroCloud allows access to the latest set of features and fixes in the tracked channels for the various snaps.
Performing an update requires going through the list of snaps one after another and updating each of the individual cluster members.

```{note}
Depending on which snap gets updated, some services of this snap (such as API endpoints) might not be available while performing the update. Check the respective services' documentation on updates for more information.
```

```{admonition} Users of the 1 track
:class: important
MicroCloud `1/(stable|candidate|edge)` reached {abbr}`EOL (End of Life)` at the end of April 2025.
If you use this track, make sure to upgrade to the `2` LTS track.
See the {ref}`howto-update-upgrade-upgrade` guide below for more information.
```

During an update, the snaps channel won't be modified, so the snaps are updated to the last available version inside the current channel.
This does not introduce breaking changes and can be used on a regular basis to update a MicroCloud deployment.

As MicroCloud consumes the services offered by the dependent snaps (MicroCeph, MicroOVN, and LXD), the update procedure starts by updating the list of dependencies first.

(howto-update-upgrade-deps)=
### Update the dependency snaps

Updating the dependencies can be done by running `snap refresh` against the respective snap.
For MicroCloud, automatic snap refreshes are put on hold. See {ref}`howto-snap-control-updates` for more information.

To start the update procedure, enter the following command on the first machine:

    sudo snap refresh microceph --cohort="+"

If the command succeeds, run the same command on the next machine, and so on.

```{note}
Make sure to validate the health of the recently updated dependency before continuing with the next one.
```

After successfully updating MicroCeph, continue with MicroOVN.
Again enter the following command on the first machine:

    sudo snap refresh microovn --cohort="+"

Run the same command on the remaining machines, one after another, unless an error is encountered.

Next, for LXD, we recommend running updates in parallel. When any cluster member's LXD snap version does not match the rest of its cluster, it's set to a blocked state. This means if you run the updates in sequence, one after another, cluster members that are updated earlier are in a blocked state until the final cluster member is updated.

Furthermore, LXD might automatically update its snaps on cluster members if they remain out of sync for too long.

On all cluster members, run _in parallel_:

    sudo snap refresh lxd --cohort="+"

### Update the MicroCloud snap

Last but not least, we can update MicroCloud.
As before, enter the following command on the first machine:

    sudo snap refresh microcloud --cohort="+"

Continue running the command on the rest of the machines to finish the update.

Confirm that the MicroCloud deployment is in a healthy state after the update by running the following command:

    sudo microcloud status

```{note}
The status command was introduced in MicroCloud version 2.
See {ref}`howto-update-upgrade-upgrade` on how to upgrade to another track.
```

(howto-update-upgrade-upgrade)=
## Upgrade MicroCloud

Upgrading MicroCloud allows switching to another track with major improvements and enhanced functionality.
Performing an upgrade requires going through the list of snaps one after another and upgrading each of the individual cluster members.

```{note}
Depending on which snap gets upgraded, some services of this snap (such as API endpoints) might not be available while performing the upgrade. Check the respective services' documentation on upgrades for more information.
```

During an upgrade, the snaps channel will be switched to another track.
This might introduce breaking changes for MicroCloud and its dependencies and should be done with care.
See {ref}`howto-update-upgrade-update` for regular non-breaking updates.

As MicroCloud consumes the services offered by the dependent snaps (MicroCeph, MicroOVN and LXD), the update procedure starts by updating the list of dependencies first.

### Upgrade the dependency snaps

Upgrading the dependencies can be done by running `snap refresh --channel <new track/stable>` against the respective snap.

Make sure to consult the dedicated upgrade guides of each dependency before you perform the actual upgrade:

* {doc}`How to upgrade MicroCeph <microceph:how-to/major-upgrade>`
* {doc}`How to upgrade MicroOVN <microovn:how-to/major-upgrades>`
* {doc}`How to upgrade LXD <lxd:howto/cluster_manage>`

To start the upgrade procedure, enter the following command on the first machine:

    sudo snap refresh microceph --channel "squid/stable" --cohort="+"

If the command succeeds, run the same command on the next machine, and so on.

```{note}
Make sure to validate the health of the recently upgraded dependency before continuing with the next one.
```

After successfully upgrading MicroCeph, continue with MicroOVN.
Again, enter the following command on the first machine:

    sudo snap refresh microovn --channel "24.03/stable" --cohort="+"

Run the same command on the remaining machines, one after another, unless an error is encountered.

Next, for LXD, we recommend running upgrades in parallel. When any cluster member's LXD snap version does not match the rest of its cluster, it's set to a blocked state. This means if you run the upgrades in sequence, one after another, cluster members upgraded earlier are in a blocked state until the final cluster member is upgraded.

On all cluster members, run _in parallel_:

    sudo snap refresh lxd --channel "5.21/stable" --cohort="+"

### Upgrade MicroCloud snap

Last but not least, we can upgrade MicroCloud.
As before, enter the following command on the first machine:

    sudo snap refresh microcloud --channel "2/stable" --cohort="+"

Continue running the command on the rest of the machines to finish the upgrade.
Confirm that the MicroCloud deployment is in a healthy state after the update by running the following command:

    sudo microcloud status
