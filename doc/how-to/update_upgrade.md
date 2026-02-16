(howto-update-upgrade)=
# How to update and upgrade

The snaps for MicroCloud, LXD, MicroCeph, and MicroOVN installed on MicroCloud cluster members must always run the same version of each snap. Thus, when the snaps on any cluster member are refreshed, they must also be refreshed on all other cluster members. See {ref}`howto-update-upgrade-order` for the recommended order.

If the cluster members' snaps are not synchronized, MicroCloud continues to function as normal in regards to its data plane. However, the configuration of its control plane cannot be altered. For example, you cannot add or remove cluster members or instances. To prevent automatic updates from causing snaps to run different versions, make sure to always {ref}`hold updates <howto-update-hold>` as well as {ref}`synchronize updates using the cohort flag <howto-update-sync>`.

Performing an update or upgrade requires going through the list of snaps one after another and updating or upgrading each of the individual cluster members. Some services of a snap (such as API endpoints) might become unavailable during this process. Check the respective services' documentation for more information.

(howto-update-upgrade-backup)=
## Back up data
Before performing an update or upgrade, make sure to back up your data to prevent any data loss in case of failure.
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
- Each VM must be pre-configured for live migration. See: {ref}`lxd:live-migration` for information on the required configurations.
- Before you update or upgrade a cluster member, use the {ref}`cluster evacuate <lxd:cluster-evacuate>` operation to migrate all instances on the host to other members in the same cluster.
- Once the update or upgrade is complete, use the {ref}`cluster restore <lxd:cluster-restore>` operation to migrate all evacuated instances back to the original host.
- The evacuate and restore operations can live-migrate any VMs that are configured to allow it. If any instances on the cluster member are ineligible for live migration (such as a container, or a VM that is not configured for live migration), then during both evacuation and restoration, those instances are stopped, migrated, and restarted.

For more information on the cluster evacuate and restore operations, see: {ref}`lxd:cluster-evacuate-restore`.

(howto-update-upgrade-order)=
## Update and upgrade order

As MicroCloud consumes the services offered by its components, when updating or upgrading, follow this recommended order:

1. MicroCeph
1. MicroOVN
1. LXD
1. MicroCloud

Update the same component's snap on all cluster members before moving to the next component. For example, update MicroCeph on all cluster members, then update MicroOVN on all cluster members, and so on.

(howto-update-sync)=
## Synchronize updates using the cohort flag

Even with manual snap updates, versions can fall out of sync; see {ref}`ref-snaps-updates` for details.

To ensure synchronized updates, the `--cohort="+"` flag must be set on all cluster members. You only need to set this flag once per snap on each cluster member, either during {ref}`installation <howto-install>`, or the first time you {ref}`perform a manual update <howto-update>`.

To set this flag during installation:

```bash
sudo snap install <snap> --cohort="+"
```

To set this flag later, during a manual update:

```bash
sudo snap refresh <snap>> --cohort="+"
```

After you set this flag, `snap list <snap>` shows `in-cohort` in the `Notes` column. Example:

```{terminal}
:input: snap list lxd
:user: root
:host: instance

Name  Version         Rev    Tracking     Publisher   Notes
lxd   5.21.3-c5ae129  33110  5.21/stable  canonical✓  in-cohort
```

Subsequent updates to this snap automatically use the `--cohort="+"` flag. Thus, once the snap is `in-cohort`, you can omit that flag for future updates.

````{admonition} Workaround if the cohort flag malfunctions
:class: tip

If for some reason, the `--cohort="+"` flag does not work as expected, you can update using a matching revision on all cluster members manually:

```bash
sudo snap refresh <snap> --revision=<revision_number>
```

Example:

```bash
sudo snap refresh lxd --revision=33110
```

````

(howto-update-hold)=
## Hold updates

Once you install a snap, it begins tracking the specified snap channel, or the default if not specified. Whenever a new version is published to the tracked channel, the snap version on your system automatically updates.

With MicroCloud and its components, it's important to control the updates. Thus, we direct you to hold updates for these snaps during the {ref}`installation process <howto-install-hold-updates>`.

You can hold snap updates either indefinitely or for a specific duration. For full control of snap updates, set an indefinite hold.

To indefinitely hold all updates to the snaps needed for MicroCloud, run:

```bash
sudo snap refresh --hold lxd microceph microovn microcloud
```

Then you can perform {ref}`manual updates <howto-update>` on a schedule that you control.

For detailed information about holds, see: [Pause or stop automatic updates](https://snapcraft.io/docs/managing-updates#p-32248-pause-or-stop-automatic-updates) in the Snap documentation.

(howto-update)=
## Update MicroCloud

```{admonition} Users of the 1 track
:class: important
The `1` MicroCloud track reached {abbr}`EOL (End of Life)` at the end of April 2025.
If you use this track, make sure to upgrade to the `2` LTS track, as no further updates will be released to the `1` track. See the {ref}`howto-upgrade` guide below for more information. Specific command syntax is provided in {ref}`howto-upgrade-microcloud-full-example`.
```

Updating MicroCloud allows access to the latest set of features and fixes in the tracked channels for the various snaps. During an update, snaps are refreshed to the most up-to-date version for their tracked channel. This does not introduce breaking changes.

Before you update, ensure that all snaps are {ref}`synchronized using the cohort flag <howto-update-sync>`. You only need to set the `--cohort` flag during updates if the snap is not `in-cohort`.

(howto-update-components)=
### Update snaps for MicroCloud components

As MicroCloud consumes the services offered by its component snaps, the update procedure starts by updating the list of components first.

To start the update procedure, enter the following command on the first machine:

```bash
sudo snap refresh microceph [--cohort="+"]
```

If the command succeeds, run the same command on the next machine, and so on. Make sure to validate the health of the recently updated component before continuing with the next one.

After successfully updating MicroCeph, continue with MicroOVN.
Again enter the following command on the first machine:

```bash
sudo snap refresh microovn [--cohort="+"]
```

Run the same command on the remaining machines, one after another, unless an error is encountered.

Next, for LXD, we recommend running updates in parallel. When any cluster member's LXD snap version does not match the rest of its cluster, it's set to a blocked state. This means if you run the updates in sequence, one after another, cluster members that are updated earlier are in a blocked state until the final cluster member is updated.

Furthermore, LXD might automatically update its snaps on cluster members if they remain out of sync for too long.

On all cluster members, run _in parallel_:

```bash
sudo snap refresh lxd [--cohort="+"]
```

(howto-update-microcloud)=
### Update the MicroCloud snap

Last but not least, we can update MicroCloud. As before, enter the following command on the first machine:

```bash
sudo snap refresh microcloud [--cohort="+"]
```

Continue running the command on the rest of the machines to finish the update.

Once finished, confirm that the MicroCloud deployment is in a healthy state by running the following command:

```bash
sudo microcloud status
```

```{note}
The status command was introduced in MicroCloud version 2.
See {ref}`howto-upgrade` on how to upgrade to another track.
```

(howto-upgrade)=
## Upgrade MicroCloud

Upgrading MicroCloud means to switch to a newer track with major improvements and enhanced functionality. An upgrade can potentially introduce breaking changes.

During an upgrade, the snaps channel will be switched to another track.
This might introduce breaking changes for MicroCloud and its components and should be done with care.
See {ref}`howto-update` for regular non-breaking updates.

Before you update, ensure that all snaps are {ref}`synchronized using the cohort flag <howto-update-sync>`. The `--cohort` flag is only necessary if the snap is not `in-cohort`.

(howto-upgrade-components)=
### Upgrade the component snaps

As MicroCloud consumes the services offered by its component snaps, the upgrade procedure starts by upgrading the list of components first.

Make sure to consult the dedicated upgrade guides of each component before you perform the actual upgrade:

* {doc}`How to upgrade MicroCeph <microceph:how-to/major-upgrade>`
* {doc}`How to upgrade MicroOVN <microovn:how-to/major-upgrades>`
* {doc}`How to upgrade LXD <lxd:howto/cluster_manage>`

To start the upgrade procedure, enter the following command on the first machine:

```bash
sudo snap refresh microceph --channel=<new channel> [--cohort="+"]
```

If the command succeeds, run the same command on the next machine, and so on. Make sure to validate the health of the recently upgraded component before continuing with the next one.

After successfully upgrading MicroCeph, continue with MicroOVN. Again, enter the following command on the first machine:

```bash
sudo snap refresh microovn --channel=<new channel> [--cohort="+"]
```

Run the same command on the remaining machines, one after another, unless an error is encountered.

Next, for LXD, we recommend running upgrades in parallel. When any cluster member's LXD snap version does not match the rest of its cluster, it's set to a blocked state. This means if you run the upgrades in sequence, one after another, cluster members upgraded earlier are in a blocked state until the final cluster member is upgraded.

On all cluster members, run _in parallel_:

```bash
sudo snap refresh lxd --channel=<new channel> [--cohort="+"]
```

(howto-upgrade-microcloud)=
### Upgrade MicroCloud snap

Last but not least, we can upgrade MicroCloud.
As before, enter the following command on the first machine:

```bash
sudo snap refresh microcloud --channel=<new channel> [--cohort="+"]
```

Continue running the command on the rest of the machines to finish the upgrade.
Confirm that the MicroCloud deployment is in a healthy state after the update by running the following command:

```bash
sudo microcloud status
```

(howto-upgrade-microcloud-full-example)=
### Example: Upgrade from MicroCloud 1 to 2

Use the commands below to upgrade all components from MicroCloud 1 to MicroCloud 2.

Omit the `--cohort="+"` flag if the MicroCeph snap is already `in-cohort`. If unsure, see: {ref}`howto-update-sync`.

First, upgrade MicroCeph on all cluster members, one by one:

```bash
sudo snap refresh microceph --channel=squid/stable --cohort="+"
```

Next, upgrade MicroOVN on all cluster members, one by one:

```bash
sudo snap refresh microovn --channel=24.03/stable --cohort="+"
```

Next, upgrade LXD on all cluster members, _in parallel_:

```bash
sudo snap refresh lxd --channel=5.21/stable --cohort="+"
```

Finally, upgrade MicroCloud on all cluster members, one by one:

```bash
sudo snap refresh microcloud --channel=2/stable
```

Confirm the cluster's health:

```bash
sudo microcloud status
```

(howto-update-upgrade-proxy)=
## Use an Enterprise Store Proxy

If you manage a large MicroCloud deployment and you need absolute control over when updates are applied, consider installing an Enterprise Store Proxy.

The Enterprise Store Proxy is a separate application that sits between the snap client command on your machines and the snap store. You can configure the Enterprise Store Proxy to make only specific snap revisions available for installation.

See the [Enterprise Store Proxy documentation](https://documentation.ubuntu.com/enterprise-store/) for information about how to install and register the Enterprise Store Proxy.

After setting it up, configure the snap clients on all cluster members to use the proxy.
See [Configuring devices](https://documentation.ubuntu.com/enterprise-store/main/how-to/devices/) for instructions.

You can then configure the Enterprise Store Proxy to override the revisions for the snaps that are needed for MicroCloud:

```bash
sudo snap-proxy override lxd <channel>=<revision>
sudo snap-proxy override microceph <channel>=<revision>
sudo snap-proxy override microovn <channel>=<revision>
sudo snap-proxy override microcloud <channel>=<revision>
```

## Related topics

How-to guides:

- {ref}`howto-support`
- {ref}`howto-install`
- {ref}`howto-snap`

Reference:

- {ref}`ref-releases-snaps`