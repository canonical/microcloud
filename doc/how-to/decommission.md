---
myst:
  html_meta:
    description: Follow these steps to securely decommission a MicroCloud cluster member or cluster.
---

(howto-decommission)=
# How to securely decommission a MicroCloud deployment

```{important}
This process will erase all data associated with your MicroCloud deployment.
Make copies of any data that you need to preserve before proceeding.
Refer to {ref}`lxd:instances-backup` and {ref}`lxd:howto-storage-backup-volume` for relevant details.
```

This guide walks you through the steps to decommission an entire MicroCloud cluster.

If you only need to decommission a single cluster member, first {ref}`remove the member from the cluster <howto-member-remove>`.
After removing the member, {ref}`update the certificate <lxd:cluster-manage-update-certificate>` on the cluster remaining in production.
Then, return to this guide, skip ahead to {ref}`howto-decommission-remove-microcloud`, and follow the instructions in the sections that follow.

Some commands used to decommission MicroCloud are LXD or MicroCeph commands.
Refer to the {ref}`LXD decommissioning guide <lxd:howto-decommission>` and the {ref}`MicroCeph guide to removing disks <microceph:remove-disk>` for related information.

(howto-decommission-remove-offline-member)=
## Remove offline cluster members

Use the `--force` flag to {ref}`remove any offline cluster members <howto-member-remove>` (you will remove online cluster members later in the process):

```bash
sudo microcloud remove --force <offline_member_name>
```

(howto-decommission-revoke-remote)=
## Revoke remote access

List all identities that have access to LXD, then delete each identity:

```bash
lxc auth identity list
lxc auth identity delete <type>/<name_or_identifier>
```

(howto-decommission-list-projects)=
## List projects

Replicators, instances, profiles, and custom volumes are scoped by {ref}`project <lxd:projects>`.
For deployments with more than one project, you must repeat some steps for **each** project, each time using the `--project` flag.
You do not need to use the `--project` flag to decommission deployments with only one project.

Run this command to get a list of all projects:

```bash
lxc project list
```

````{note}
You can also delete a project (except the `default` project) and all of its project-level entities with:

```bash
lxc project delete <project_name> --force
```
````

(howto-decommission-delete-replicators)=
## Delete replicators and cluster links

For each project, list all replicators, then delete each replicator:

```bash
lxc replicator list --project <project_name>
lxc replicator delete <replicator_name> --project <project_name>
```

Likewise, list all cluster links, then delete each cluster link (cluster links are not scoped by project, so you do not need to use the `--project` flag):

```bash
lxc cluster link list
lxc cluster link delete <cluster_link_name>
```

(howto-decommission-delete-data)=
## Delete data

You can run the commands in this section on any online cluster member to delete data.

```{important}
Data deleted by LXD physically remains on disks and can be recovered by users with access to the disks.
To prevent unauthorized data recovery, you must {ref}`destroy and sanitize your data <howto-decommission-destroy-data>`.
```

(howto-decommission-delete-instances)=
### Stop and delete instances

For each project, stop all instances:

```bash
lxc stop --all --project <project_name>
```

Next, for each project, list all instances, then delete each instance:

```bash
lxc list --project <project_name>
lxc delete <instance_name> --project <project_name>
```

If you are unable to stop or delete an instance, use the `--force` flag:

```bash
lxc stop --force <instance_name> --project <project_name>
lxc delete --force <instance_name> --project <project_name>
```

(howto-decommission-delete-profiles)=
### Delete profiles

For each project, list all profiles:

```bash
lxc profile list --project <project_name>
```

Each project has a `default` profile that cannot be deleted.
Delete all other profiles:

```bash
lxc profile delete <profile_name> --project <project_name>
```

(howto-decommission-remove-disk-devices)=
### Remove disk devices from `default` profiles

You cannot delete a storage pool used by an instance, profile, or custom volume.
You must, therefore, remove any disk devices used by the `default` profiles in order to delete any storage pools or custom volumes referenced by those devices.

At a minimum, the `default` profile of the `default` project has a disk device named `root` that references a storage pool.
Remove this device with:

```bash
lxc profile device remove default root --project default
```

To check for additional disk devices, view information about the `default` profile of each project:

```bash
lxc profile show default --project <project_name>
```

Remove any remaining disk devices that reference storage pools:

```bash
lxc profile device remove default <device_name> --project <project_name>
```

(howto-decommission-delete-volumes)=
### Delete custom volumes

To delete {ref}`custom volumes <lxd:storage-volume-types>`, you must specify the storage pools used by the volumes.
First, list all storage pools across projects:

```bash
lxc storage list
```

Next, for each storage pool, list the custom volumes.
Use the `--all-projects` flag to view all custom volumes across projects:

```bash
lxc storage volume list <pool_name> type=custom --all-projects
```

Use the `PROJECT` column in the output to identify the project associated with each custom volume.
Then delete each custom volume, specifying both the storage pool and the project:

```bash
lxc storage volume delete <pool_name> <volume_name> --project <project_name>
```

(howto-decommission-delete-pools)=
### Delete storage pools

```{note}
Storage pools are not scoped by project, so you do not need to use the `--project` flag with `lxc storage` commands.
```

List all storage pools, then delete each one:

```bash
lxc storage list
lxc storage delete <pool_name>
```

(howto-decommission-delete-monitoring)=
### Delete monitoring data

Delete data from any external systems that you used to monitor {ref}`LXD events <lxd:howto-security-events>`, {ref}`LXD metrics <lxd:metrics>`, or {ref}`Ceph logging <microceph:secure-deployment-best-practices>`, such as [Loki](https://grafana.com/oss/loki/), [Prometheus](https://prometheus.io/), or [Grafana](https://grafana.com/).
Refer to the documentation for those systems for details.


(howto-decommission-remove-microceph-osds)=
## Remove MicroCeph OSDs

To {ref}`remove MicroCeph OSDs <microceph:remove-disk>`, list all disks, then remove each one:

```bash
microceph disk list
sudo microceph disk remove <osd_id>
```

Finally, verify that the OSDs have been removed:

```
microceph disk list
```

````{note}
If you are unable to remove an OSD, use the `--bypass-safety-checks` flag:

```bash
sudo microceph disk remove <osd_id> --bypass-safety-checks
```
````


(howto-decommission-remove-remaining-members)=
## Remove remaining cluster members

After deleting data, you can remove the online cluster members from the cluster.
First, list all cluster members:

```bash
microcloud cluster list
```

You can then {ref}`remove most cluster members <howto-member-remove>` with:

```bash
sudo microcloud remove <member_name>
```

However, before reducing the cluster from two members to one member, you must {ref}`clean up the Ceph monitor map <howto-member-remove-reduce-cluster>`.

```{note}
As you remove each member, you can run `microcloud status` on the remaining cluster members to verify the removal.
```

(howto-decommission-remove-microcloud)=
## Remove snaps

```{important}
Run these commands on **every** machine that you decommission.

Removing MicroCloud **does not** erase {ref}`ZFS pools (zpools) <lxd:storage-zfs>` or dedicated disks used by MicroCeph as Ceph object storage daemons (OSDs).
To securely decommission MicroCloud, you must {ref}`destroy and sanitize your data <howto-decommission-destroy-data>`.
```

Remove the MicroCloud, LXD, MicroCeph, and MicroOVN snaps.
Use the `--purge` flag, or a snapshot of your data will be preserved:

```bash
sudo snap remove microcloud --purge
sudo snap remove lxd --purge
sudo snap remove microceph --purge
sudo snap remove microovn --purge
```

```{note}
The MicroCeph and MicroOVN snaps may not be installed if you deployed a MicroCloud without those components.
```

Verify that the snaps and associated data were removed.
The following commands should report that none of these snaps are installed and that the `/var/snap/microcloud/`, `/var/snap/lxd/`, `/var/snap/microceph/`, and `/var/snap/microovn` directories do not exist:

```bash
snap list microcloud lxd microceph microovn
ls /var/snap/microcloud/ /var/snap/lxd/ /var/snap/microceph/ /var/snap/microovn/
```

(howto-decommission-destroy-data)=
## Destroy and sanitize data

Data deleted with MicroCloud, LXD, or MicroCeph commands remains readable and can be recovered by users with access to disks used in your deployment.
To prevent unauthorized recovery, you must physically overwrite the data.
Follow your data destruction policy to securely erase or destroy the disks that you are decommissioning.

If you are decommissioning an entire MicroCloud, apply your data destruction policy to any machines used to monitor events, logs, or metrics.
For clusters {ref}`configured with OIDC <lxd:howto-oidc>`, consult your OIDC identity provider for the steps to remove any data associated with your profile.
Likewise, if you used {ref}`ACME services to issue server certificates <lxd:authentication-server-certificate>`, refer to the service provider for the steps to remove any associated data.

```{important}
Sanitized data is irreversibly destroyed and cannot be recovered.
```
