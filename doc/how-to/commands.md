(howto-commands)=
# How to work with MicroCloud (command cheat sheet)

This guide lists the commands you need to know to do common operations in MicroCloud.
This command list is not meant to be exhaustive, but it gives a general overview and serves as an entry point to working with MicroCloud.

Make sure to also check the {doc}`LXD documentation <lxd:index>`.
Most commands you use in MicroCloud are actually LXD client commands and are documented in more detail in the LXD documentation.
There, you can also find the {ref}`man pages <lxd:lxc.md>` for the {command}`lxc` command.

The sections of the command list provide direct links to specific pages containing more information about the respective topics.

## Create and manage instances

See {ref}`lxd:instances`.

### Check available images

See {ref}`lxd:images-remote`.

```{list-table}
:widths: 2 3

* - List remotes
  - {command}`lxc remote list`
* - Switch remotes
  - {command}`lxc remote switch <remote>`
* - List images
  - {command}`lxc image list [<remote>:]`
```

### Create instances

See {ref}`lxd:instances-create` and {ref}`lxd:instances-console`.

```{list-table}
 :widths: 2 3

 * - Create a container (without starting it)
   - {command}`lxc init [<remote>:]<image> [<name>] [flags]`
 * - Create and start a container
   - {command}`lxc launch [<remote>:]<image> [<name>] [flags]`
 * - Create a VM (without starting it)
   - {command}`lxc init [<remote>:]<image> [<name>] --vm [flags]`
 * - Create and start a VM and connect to its VGA console
   - {command}`lxc launch [<remote>:]<image> [<name>] --vm --console=vga [flags]`
```

### Manage instances

See {ref}`lxd:instances-manage`.

```{list-table}
 :widths: 2 3

 * - List instances
   - {command}`lxc list`
 * - Show status information about an instance
   - {command}`lxc info <instance>`
 * - Start an instance
   - {command}`lxc start <instance> [flags]`
 * - Stop an instance
   - {command}`lxc stop <instance> [flags]`
 * - Force-stop an instance
   - {command}`lxc stop <instance> --force`
 * - Delete an instance
   - {command}`lxc delete <instance> [--force|--interactive]`
 * - Copy an instance
   - {command}`lxc copy <instance> <new_name> [flags]`
```

## Access instances

See {ref}`lxd:run-commands`, {ref}`lxd:instances-console`, and {ref}`instances-access-files`.

```{list-table}
 :widths: 2 3

 * - Run a command inside an instance
   - {command}`lxc exec <instance> -- <command>`
 * - Get shell access to an instance (if {command}`bash` is installed)
   - {command}`lxc exec <instance> -- bash`
 * - Get console access to an instance
   - {command}`lxc console <instance> [flags]`
 * - Pull a file from an instance
   - {command}`lxc file pull <instance>/<instance_filepath> <local_filepath>`
 * - Push a file to an instance
   - {command}`lxc file pull <local_filepath> <instance>/<instance_filepath>`
```

## Configure instances

See {ref}`lxd:instances-configure`, {ref}`lxd:profiles`, and {ref}`lxd:instance-config` (and sub-pages).

```{list-table}
 :widths: 2 3

 * - Show the configuration of an instance
   - {command}`lxc config show <instance>`
 * - Show the configuration of an instance, including configurations inherited from a profile
   - {command}`lxc config show <instance> --expanded`
 * - Set some configuration options for an instance (this example limits memory and CPU usage)
   - {command}`lxc config set <instance> limits.memory=8GiB limits.cpu=4`

     ```{tip}
     See {ref}`lxd:instance-options` for all available instance options.
     ```
 * - Override some device options for an instance (this example sets the root disk size)
   - {command}`lxc config device override <instance> root size=10GiB`

     ```{tip}
     See {ref}`lxd:devices` for the device options that are available for each device type.
     ```
 * - Edit the full configuration of an instance
   - {command}`lxc config edit <instance>`
 * - Apply a profile to an instance
   - {command}`lxc profile add <instance> <profile>`
```

### Use `cloud-init`

```{rst-class} spellexception
```

See {ref}`lxd:cloud-init`.

For example, to import an SSH key:

1. Create a profile: {command}`lxc profile create <profile>`
1. Run {command}`lxc profile edit <profile>` and add the following configuration to the profile:

       config:
         cloud-init.user-data: |
           #cloud-config
           ssh_authorized_keys:
             - <public_key>
1. Launch the instance using that profile (in addition to the `default` profile): {command}`lxc launch <image> [<name>] --profile default --profile <profile>`

## Manage instance snapshots

See {ref}`lxd:instances-snapshots`.

```{list-table}
 :widths: 2 3

 * - Create a snapshot
   - {command}`lxc snapshot <instance> [<snapshot_name>] [flags]`
 * - View information about a snapshot
   - {command}`lxc config show <instance>/<snapshot_name>`
 * - View all snapshots of an instance
   - {command}`lxc info <instance>`
 * - Restore a snapshot
   - {command}`lxc restore <instance> <snapshot_name> [--stateful]`
 * - Delete a snapshot
   - {command}`lxc delete <instance>/<snapshot_name>`
 * - Create an instance from a snapshot
   - {command}`lxc copy <instance>/<snapshot_name> <new_instance>`
```

## Configure storage

See {ref}`lxd:howto-storage-volumes`.

To create a storage pool, see {ref}`lxd:howto-storage-pools`.
However, keep in mind that for MicroCloud to be able to use the storage pool, it must be created for the cluster and not only for one machine.
Therefore, the following example commands use the `remote` storage pool that is automatically set up in MicroCloud.

```{list-table}
 :widths: 2 3

 * - Create a custom storage volume of content type `filesystem` in the `remote` storage pool
   - {command}`lxc storage volume create remote <volume>`
 * - Create a custom storage volume of content type `block` in the `remote` storage pool
   - {command}`lxc storage volume create remote <volume> --type=block`
 * - Attach a custom storage volume of content type `filesystem` to an instance
   - {command}`lxc storage volume attach remote <volume> <instance> <location>`
 * - Attach a custom storage volume of content type `block` to an instance
   - {command}`lxc storage volume attach remote <volume> <instance>`
```

## Configure networking

See {ref}`lxd:networking` (and sub-pages).

```{list-table}
:widths: 2 3

* - Create a network
  - {command}`lxc network create <network> --type=<type> [options]`

    ```{tip}
    See {ref}`lxd:network-create` for detailed information.
    ```
* - Attach an instance to a network
  - {command}`lxc network attach <network> <instance> [<device>] [<interface>]`

    ```{tip}
    See {ref}`lxd:network-attach` for detailed information.
    ```
* - Create and apply a network ACL to an instance
  - {command}`lxc network acl rule add <ACL> ingress|egress [properties]`

    {command}`lxc network set <network> security.acls="<ACL>"`

    ```{tip}
    See {ref}`lxd:network-acls` for detailed information.
    ```
* - Expose an instance on an external IP
  - {command}`lxc network forward <network> create <external_IP> target_address=<instance_IP>`

    ```{tip}
    See {ref}`lxd:network-forwards` for detailed information.
    ```
```

## Use projects

See {ref}`lxd:exp-projects` and {ref}`projects` (and sub-pages).

```{list-table}
 :widths: 2 3

 * - Create a project
   - {command}`lxc project create <project> [--config <option>]`
 * - Configure a project
   - {command}`lxc project set <project> <option>`
 * - Switch to a project
   - {command}`lxc project switch <project>`
```

## Configure the LXD server

See {ref}`lxd:server-configure`.

```{list-table}
:widths: 2 3

* - Configure server options
  - {command}`lxc config set <key> <value>`

    ```{tip}
    See {ref}`lxd:server` for all available server options.
    ```
* - Enable GUI access to the LXD cluster
  - {command}`lxc config set core.https_address :8443`

    Then enable the UI on the snap and reload the snap:

        snap set lxd ui.enable=true
        sudo systemctl reload snap.lxd.daemon

    ```{tip}
    See {ref}`lxd:access-ui` for detailed information.
    ```
```
## Manage the MicroCloud cluster

See {ref}`lxd:cluster-manage-instance` and {ref}`lxd:cluster-evacuate`.

```{list-table}
 :widths: 2 3

 * - Inspect the cluster status for all services at once
   - {command}`microcloud service list`

 * - Inspect the cluster status for each service
   - {command}`microcloud cluster list`

     {command}`lxc cluster list`

     {command}`microceph cluster list`

     {command}`microovn cluster list`
 * - Move an instance to a different cluster member
   - {command}`lxc move <instance> --target <member>`
 * - Copy an instance from a different LXD server
   - Add one of the MicroCloud cluster members as a remote on the different LXD server and copy or move the instance from that server.

     {command}`lxc copy <instance> <remote>`

     ```{tip}
     See {ref}`lxd:move-instances` for details.
     ```
 * - Evacuate a cluster member
   - {command}`lxc cluster evacuate <member>`
 * - Restore a cluster member
   - {command}`lxc cluster restore <member>`
```
