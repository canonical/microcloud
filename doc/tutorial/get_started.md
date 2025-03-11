---
relatedlinks: https://documentation.ubuntu.com/lxd/
---

(get-started)=
# Get started with MicroCloud

MicroCloud is quick to set up.
Once {ref}`installed <howto-install>`, you can start using MicroCloud in the same way as a regular LXD cluster.

This tutorial guides you through installing and initializing MicroCloud in a confined environment, then starting some instances to see what you can do with MicroCloud.
It uses LXD virtual machines (VMs) for the MicroCloud cluster members, so you don't need any extra hardware to follow the tutorial.

```{tip}
   While VMs are used as cluster members for this tutorial, we recommend that you use physical machines in a production environment. You can use VMs as cluster members in testing or development environments. To do so, your host machine must have nested virtualization enabled. See the [Ubuntu Server documentation on how to check if nested virtualization is enabled](https://documentation.ubuntu.com/server/how-to/virtualisation/enable-nested-virtualisation/#check-if-nested-virtualisation-is-enabled).

   We also limit each machine in this tutorial to 2 GiB of RAM, which is less than the recommended hardware requirements. In the context of this tutorial, this amount of RAM is sufficient. However, in a production environment, make sure to use machines that fulfill the {ref}`hardware-requirements`.
```

## 1. Install and initialize LXD

```{note}
   You can skip this step if you already have a LXD server installed and initialized on your host machine.
   However, you should make sure that you have a storage pool set up that is big enough to store four VMs. We recommend a minimum storage pool size of 40 GiB.
```

MicroCloud requires LXD version 5.21:

1. Install `snapd`:

   1. Run {command}`snap version` to find out if snap is installed on your system:

      ```{terminal}
      :input: snap version

      snap    2.59.4
      snapd   2.59.4
      series  16
      ubuntu  22.04
      kernel  5.15.0-73-generic
      ```

      If you see a table of version numbers, snap is installed.
      If the version for `snapd` is 2.59 or later, you are all set and can continue with the next step of installing LXD.

   1. If the version for `snapd` is earlier than 2.59, or if the {command}`snap version` command returns an error, run the following commands to install the latest version of `snapd`:

          sudo apt update
          sudo apt install snapd

1. If LXD is already installed, enter the following command to update it:

       sudo snap refresh lxd --channel=5.21/stable

   Otherwise, enter the following command to install LXD:

       sudo snap install lxd

1. Enter the following command to initialize LXD:

       lxd init

   Accept the default values except for the following questions:

   - `Size in GiB of the new loop device (1GiB minimum)`

     Enter `40`.

   - `What IPv4 address should be used? (CIDR subnet notation, “auto” or “none”)`

     Enter `10.1.123.1/24`.

   - `What IPv6 address should be used? (CIDR subnet notation, “auto” or “none”)`

     Enter `fd42:1:1234:1234::1/64`.

   - `Would you like the LXD server to be available over the network? (yes/no)`

     Enter `yes`.

1. Modify the default network so we can later define specific IPv6 addresses for the VMs:

       lxc network set lxdbr0 ipv6.dhcp.stateful true

```{note}
In the steps above, we ask you to specify the IP addresses to be used instead of accepting the defaults. While this is not strictly required for this setup, it causes the example IPs displayed in this tutorial to match what you see on your system, which improves clarity.
```

## 2. Provide storage disks

MicroCloud supports both local and remote storage.
For local storage, you need one disk per cluster member.
For remote storage with high availability (HA), you need at least three disks that are located across three different cluster members.

In this tutorial, we will set up each of the four cluster members with local storage. We will also set up three of the cluster members with remote storage. In total, we will set up seven disks.
It's possible to add remote storage on the fourth cluster member, if desired. However, it is not required for HA.

Complete the following steps to create the required disks in a LXD storage pool:

1. Create a ZFS storage pool called `disks`:

       lxc storage create disks zfs size=100GiB

1. Configure the default volume size for the `disks` pool:

       lxc storage set disks volume.size 10GiB

1. Create four disks to use for local storage:

       lxc storage volume create disks local1 --type block
       lxc storage volume create disks local2 --type block
       lxc storage volume create disks local3 --type block
       lxc storage volume create disks local4 --type block

1. Create three disks to use for remote storage:

       lxc storage volume create disks remote1 --type block size=20GiB
       lxc storage volume create disks remote2 --type block size=20GiB
       lxc storage volume create disks remote3 --type block size=20GiB

1. Check that the disks have been created correctly:

   ```{terminal}
   :input: lxc storage volume list disks
   :user: root
   :host: micro1

   +--------+---------+-------------+--------------+---------+
   |  TYPE  |  NAME   | DESCRIPTION | CONTENT-TYPE | USED BY |
   +--------+---------+-------------+--------------+---------+
   | custom | local1  |             | block        | 0       |
   +--------+---------+-------------+--------------+---------+
   | custom | local2  |             | block        | 0       |
   +--------+---------+-------------+--------------+---------+
   | custom | local3  |             | block        | 0       |
   +--------+---------+-------------+--------------+---------+
   | custom | local4  |             | block        | 0       |
   +--------+---------+-------------+--------------+---------+
   | custom | remote1 |             | block        | 0       |
   +--------+---------+-------------+--------------+---------+
   | custom | remote2 |             | block        | 0       |
   +--------+---------+-------------+--------------+---------+
   | custom | remote3 |             | block        | 0       |
   +--------+---------+-------------+--------------+---------+
   ```

## 3. Create a network

MicroCloud requires an uplink network that the cluster members can use for external connectivity.
See {ref}`explanation-networking` for more information.

Complete the following steps to set up this network:

1. Create a bridge network without any parameters:

       lxc network create microbr0

      (tutorial-note-ips)=
1. Enter the following commands to find out the assigned IPv4 and IPv6 addresses for the network, and note them down:

       lxc network get microbr0 ipv4.address
       lxc network get microbr0 ipv6.address

## 4. Create and configure your VMs

Next, we'll create the VMs that will serve as the MicroCloud cluster members.

Complete the following steps:

1. Create the VMs, but don't start them yet:

       lxc init ubuntu:22.04 micro1 --vm --config limits.cpu=2 --config limits.memory=2GiB -d eth0,ipv4.address=10.1.123.10 -d eth0,ipv6.address=fd42:1:1234:1234::10
       lxc init ubuntu:22.04 micro2 --vm --config limits.cpu=2 --config limits.memory=2GiB -d eth0,ipv4.address=10.1.123.20 -d eth0,ipv6.address=fd42:1:1234:1234::20
       lxc init ubuntu:22.04 micro3 --vm --config limits.cpu=2 --config limits.memory=2GiB -d eth0,ipv4.address=10.1.123.30 -d eth0,ipv6.address=fd42:1:1234:1234::30
       lxc init ubuntu:22.04 micro4 --vm --config limits.cpu=2 --config limits.memory=2GiB -d eth0,ipv4.address=10.1.123.40 -d eth0,ipv6.address=fd42:1:1234:1234::40

   ```{tip}
      Run these commands in sequence, not in parallel.

      LXD downloads the image the first time you use it to initialize a VM. For subsequent runs, LXD uses the cached image. Therefore, the {command}`init` command will take longer to complete on the first run.
   ```

1. Attach the disks to the VMs:

       lxc storage volume attach disks local1 micro1
       lxc storage volume attach disks local2 micro2
       lxc storage volume attach disks local3 micro3
       lxc storage volume attach disks local4 micro4
       lxc storage volume attach disks remote1 micro1
       lxc storage volume attach disks remote2 micro2
       lxc storage volume attach disks remote3 micro3

1. Create and add network interfaces that use the dedicated MicroCloud network to each VM:

       lxc config device add micro1 eth1 nic network=microbr0
       lxc config device add micro2 eth1 nic network=microbr0
       lxc config device add micro3 eth1 nic network=microbr0
       lxc config device add micro4 eth1 nic network=microbr0

1. Start the VMs:

       lxc start micro1
       lxc start micro2
       lxc start micro3
       lxc start micro4

## 5. Install MicroCloud on each VM

Before you can create the MicroCloud cluster, you must install the required snaps on each VM.
In addition, you must configure the network interfaces so they can be used by MicroCloud.

Complete the following steps on each VM (`micro1`, `micro2`, `micro3`, and `micro4`):

   ```{tip}
   You can run the following commands in parallel on each VM. We recommend that you open three additional terminals, so that you have a terminal for each VM.
   ```

1. Access the shell in each VM.
   For example, for `micro1`:

       lxc exec micro1 -- bash

   ```{tip}
   If you get an error message stating that the LXD VM agent is not currently running, the VM hasn't fully started up yet.
   Wait a while and then try again.
   If the error persists, try restarting the VM (`lxc restart micro1`).
   ```
1. MicroCloud requires a network interface that doesn't have an IP address assigned. Thus, configure the network interface connected to `microbr0` to refuse any IP addresses:

       cat << EOF > /etc/netplan/99-microcloud.yaml
       # MicroCloud requires a network interface that doesn't have an IP address
       network:
           version: 2
           ethernets:
               enp6s0:
                   accept-ra: false
                   dhcp4: false
                   link-local: []
       EOF
       chmod 0600 /etc/netplan/99-microcloud.yaml

   ```{note}
   `enp6s0` is the name that the VM assigns to the network interface that we previously added as `eth1`.
   ```

1. Bring the network interface up:

       netplan apply

1. Install the required snaps:

       snap install microceph --channel=squid/stable --cohort="+"
       snap install microovn --channel=24.03/stable --cohort="+"
       snap install microcloud --channel=2/stable --cohort="+"

   ```{note}
   The `--cohort="+"` flag in the command ensures that the same version of the snap is installed on all machines.
   See {ref}`howto-snap-cluster` for more information.
   ```

1. The LXD snap is already installed.
   Refresh it to the latest version:

       snap refresh lxd --channel=5.21/stable --cohort="+"

1. Repeat these steps on all VMs.

## 6. Initialize MicroCloud

We use the `micro1` VM to initialize MicroCloud in the instructions below, but you can use any of the four VMs.

Complete the following steps:

1. Access the shell in `micro1` and start the initialization process:

       lxc exec micro1 microcloud init

   ```{tip}
   In this tutorial, we initialize MicroCloud interactively.
   Alternatively, you can use a preseed file for {ref}`howto-initialize-preseed`.
   ```

1. Answer the questions:

   1. Select `yes` to select more than one cluster member.
   1. As the address for MicroCloud's internal traffic, select the listed IPv4 address.
   1. Copy the session passphrase.

1. Head to the other VMs (`micro2`, `micro3`, and `micro4`) and start the join process on each:

       lxc exec micro2 microcloud join

   In each joining cluster member, select the listed IPv4 address for MicroCloud's internal traffic.

   When prompted, enter the session passphrase for each joining  member.

1. Return to `micro1` to continue the initialization process:

   1. Select all listed systems to join the cluster. These should be `micro2`, `micro3`, and `micro4`.
   1. Select `yes` to set up local storage.
   1. Select the listed local disks. These should be `local1`, `local2`, `local3`, and `local4`.

      ```{tip}
      Type `local` to display only the local disks.
      The table is filtered by the characters that you type.
      ```

   1. You don't need to wipe any local disks, because we just created them. Press {kbd}`Enter` without selecting any disks to wipe.
   1. Select `yes` to set up distributed storage.
   1. Select `yes` to confirm that there are fewer disks available than machines.
   1. Select all listed disks (these should be `remote1`, `remote2`, and `remote3`).
   1. You don't need to wipe any remote disks, because we just created them. Press {kbd}`Enter` without selecting any disks to wipe.
   1. You don't need to encrypt any disks to get started.
   1. Select `yes` to optionally configure the CephFS distributed file system.
   1. Press {kbd}`Enter` to accept the default option for the IPv4 or IPv6 CIDR subnet address used for the Ceph internal network.
   1. Press {kbd}`Enter` to accept the default option for the IPv4 or IPv6 CIDR subnet address used for the Ceph public network.
   1. Select `yes` to configure distributed networking.
   1. Select all listed network interfaces. These should be `enp6s0` on all four cluster members.
   1. Specify the IPv4 address that {ref}`you noted down<tutorial-note-ips>` for your `microbr0` network's IPv4 gateway.
   1. Specify an IPv4 address in the address range as the first IPv4 address.
      For example, if your IPv4 gateway is `192.0.2.1/24`, the first address could be `192.0.2.100`.
   1. Specify a higher IPv4 address in the range as the last IPv4 address.
      As we're setting up four machines only, the range must contain a minimum of four addresses, but setting up a bigger range is more fail-safe.
      For example, if your IPv4 gateway is `192.0.2.1/24`, the last address could be `192.0.2.254`.
   1. Specify the IPv6 address that {ref}`you noted down<tutorial-note-ips>` for your `microbr0` network as the IPv6 gateway.
   1. Press {kbd}`Enter` to accept the default option for the DNS addresses for the distributed network.
   1. Press {kbd}`Enter` to accept the default option for configuring an underlay network for OVN.

MicroCloud will now initialize the cluster.
See {ref}`explanation-initialization` for more information.

See the full process here for the initiating side:

(initialization-process)=

```{terminal}
:input: microcloud init
:user: root
:host: micro1
:scroll:

Do you want to set up more than one cluster member? (yes/no) [default=yes]: yes
Select an address for MicroCloud's internal traffic:
Space to select; enter to confirm; type to filter results.
Up/down to move; right to select all; left to select none.
       +----------------------+--------+
       |       ADDRESS        | IFACE  |
       +----------------------+--------+
> [X]  | 10.1.123.10          | enp5s0 |
  [ ]  | fd42:1:1234:1234::10 | enp5s0 |
       +----------------------+--------+

 Using address "10.1.123.10" for MicroCloud

Use the following command on systems that you want to join the cluster:

 microcloud join

When requested enter the passphrase:

 koala absorbing update dorsal

Verify the fingerprint "5d0808de679d" is displayed on joining systems.
Waiting to detect systems ...
Systems will appear in the table as they are detected. Select those that should join the cluster:
Space to select; enter to confirm; type to filter results.
Up/down to move; right to select all; left to select none.
       +---------+-------------+--------------+
       |  NAME   |   ADDRESS   | FINGERPRINT  |
       +---------+-------------+--------------+
> [x]  | micro3  | 10.1.123.30 | 4e80954d6a64 |
  [x]  | micro2  | 10.1.123.20 | 84e0b50e13b3 |
  [x]  | micro4  | 10.1.123.40 | 98667a808a99 |
       +---------+-------------+--------------+

 Selected "micro1" at "10.1.123.10"
 Selected "micro3" at "10.1.123.30"
 Selected "micro2" at "10.1.123.20"
 Selected "micro4" at "10.1.123.40"

Would you like to set up local storage? (yes/no) [default=yes]: yes
Select exactly one disk from each cluster member:
Space to select; enter to confirm; type to filter results.
Up/down to move; right to select all; left to select none.
       +----------+---------------+----------+------+------------------------------------------------------+
       | LOCATION |     MODEL     | CAPACITY | TYPE |                         PATH                         |
       +----------+---------------+----------+------+------------------------------------------------------+
  [x]  | micro1   | QEMU HARDDISK | 10.00GiB | scsi | /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_local1  |
  [ ]  | micro1   | QEMU HARDDISK | 20.00GiB | scsi | /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_remote1 |
  [x]  | micro2   | QEMU HARDDISK | 10.00GiB | scsi | /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_local2  |
  [ ]  | micro2   | QEMU HARDDISK | 20.00GiB | scsi | /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_remote2 |
  [x]  | micro3   | QEMU HARDDISK | 10.00GiB | scsi | /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_local3  |
  [ ]  | micro3   | QEMU HARDDISK | 20.00GiB | scsi | /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_remote3 |
> [x]  | micro4   | QEMU HARDDISK | 10.00GiB | scsi | /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_local4  |
       +----------+---------------+----------+------+------------------------------------------------------+

Select which disks to wipe:
Space to select; enter to confirm; type to filter results.
Up/down to move; right to select all; left to select none.
       +----------+---------------+----------+------+------------------------------------------------------+
       | LOCATION |     MODEL     | CAPACITY | TYPE |                         PATH                         |
       +----------+---------------+----------+------+------------------------------------------------------+
> [ ]  | micro1   | QEMU HARDDISK | 10.00GiB | scsi | /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_local1  |
  [ ]  | micro2   | QEMU HARDDISK | 10.00GiB | scsi | /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_local2  |
  [ ]  | micro3   | QEMU HARDDISK | 10.00GiB | scsi | /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_local3  |
  [ ]  | micro4   | QEMU HARDDISK | 10.00GiB | scsi | /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_local4  |
       +----------+---------------+----------+------+------------------------------------------------------+

 Using "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_local3" on "micro3" for local storage pool
 Using "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_local4" on "micro4" for local storage pool
 Using "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_local1" on "micro1" for local storage pool
 Using "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_local2" on "micro2" for local storage pool

Would you like to set up distributed storage? (yes/no) [default=yes]: yes
Unable to find disks on some systems. Continue anyway? (yes/no) [default=yes]: yes
Select from the available unpartitioned disks:
Space to select; enter to confirm; type to filter results.
Up/down to move; right to select all; left to select none.
       +----------+---------------+----------+------+------------------------------------------------------+
       | LOCATION |     MODEL     | CAPACITY | TYPE |                         PATH                         |
       +----------+---------------+----------+------+------------------------------------------------------+
> [x]  | micro1   | QEMU HARDDISK | 20.00GiB | scsi | /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_remote1 |
  [x]  | micro2   | QEMU HARDDISK | 20.00GiB | scsi | /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_remote2 |
  [x]  | micro3   | QEMU HARDDISK | 20.00GiB | scsi | /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_remote3 |
       +----------+---------------+----------+------+------------------------------------------------------+

Select which disks to wipe:
Space to select; enter to confirm; type to filter results.
Up/down to move; right to select all; left to select none.
       +----------+---------------+----------+------+------------------------------------------------------+
       | LOCATION |     MODEL     | CAPACITY | TYPE |                         PATH                         |
       +----------+---------------+----------+------+------------------------------------------------------+
> [ ]  | micro1   | QEMU HARDDISK | 20.00GiB | scsi | /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_remote1 |
  [ ]  | micro2   | QEMU HARDDISK | 20.00GiB | scsi | /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_remote2 |
  [ ]  | micro3   | QEMU HARDDISK | 20.00GiB | scsi | /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_remote3 |
       +----------+---------------+----------+------+------------------------------------------------------+

 Using 1 disk(s) on "micro1" for remote storage pool
 Using 1 disk(s) on "micro2" for remote storage pool
 Using 1 disk(s) on "micro3" for remote storage pool

Do you want to encrypt the selected disks? (yes/no) [default=no]: no
Would you like to set up CephFS remote storage? (yes/no) [default=yes]:  yes
What subnet (either IPv4 or IPv6 CIDR notation) would you like your Ceph internal traffic on? [default: 10.1.123.0/24]:
What subnet (either IPv4 or IPv6 CIDR notation) would you like your Ceph public traffic on? [default: 10.1.123.0/24]:
Configure distributed networking? (yes/no) [default=yes]:  yes
Select an available interface per system to provide external connectivity for distributed network(s):
Space to select; enter to confirm; type to filter results.
Up/down to move; right to select all; left to select none.
       +----------+--------+----------+
       | LOCATION | IFACE  |   TYPE   |
       +----------+--------+----------+
> [x]  | micro2   | enp6s0 | physical |
  [x]  | micro3   | enp6s0 | physical |
  [x]  | micro1   | enp6s0 | physical |
  [x]  | micro4   | enp6s0 | physical |
       +----------+--------+----------+

 Using "enp6s0" on "micro3" for OVN uplink
 Using "enp6s0" on "micro1" for OVN uplink
 Using "enp6s0" on "micro2" for OVN uplink
 Using "enp6s0" on "micro4" for OVN uplink

Specify the IPv4 gateway (CIDR) on the uplink network (empty to skip IPv4): 192.0.2.1/24
Specify the first IPv4 address in the range to use on the uplink network: 192.0.2.100
Specify the last IPv4 address in the range to use on the uplink network: 192.0.2.254
Specify the IPv6 gateway (CIDR) on the uplink network (empty to skip IPv6): 2001:db8:d:200::1/64
Specify the DNS addresses (comma-separated IPv4 / IPv6 addresses) for the distributed network (default: 192.0.2.1,2001:db8:d:200::1):
Configure dedicated underlay networking? (yes/no) [default=no]:

Initializing new services
 Local MicroCloud is ready
 Local LXD is ready
 Local MicroOVN is ready
 Local MicroCeph is ready
Awaiting cluster formation ...
 Peer "micro2" has joined the cluster
 Peer "micro3" has joined the cluster
 Peer "micro4" has joined the cluster
Configuring cluster-wide devices ...
MicroCloud is ready
```

See the full process here for one of the joining sides (`micro2`):

```{terminal}
:input: microcloud join
:user: root
:host: micro1
:scroll:

Select an address for MicroCloud's internal traffic:
Space to select; enter to confirm; type to filter results.
Up/down to move; right to select all; left to select none.
       +----------------------+--------+
       |        ADDRESS       | IFACE  |
       +----------------------+--------+
> [ ]  | 10.1.123.20          | enp5s0 |
  [ ]  | fd42:1:1234:1234::20 | enp5s0 |
       +----------------------+--------+

Using address "10.1.123.20" for MicroCloud

Verify the fingerprint "84e0b50e13b3" is displayed on the other system.
Specify the passphrase for joining the system: koala absorbing update dorsal
Searching for an eligible system ...

 Found system "micro1" at "10.1.123.10" using fingerprint "5d0808de679d"

Select "micro2" on "micro1" to let it join the cluster

 Received confirmation from system "micro1"

Do not exit out to keep the session alive.
Complete the remaining configuration on "micro1" ...
Successfully joined the MicroCloud cluster and closing the session.
Commencing cluster join of the remaining services (LXD, MicroCeph, MicroOVN)
```

## 7. Inspect your MicroCloud setup

You can now inspect your cluster setup.

```{tip}
You can run these commands on any of the cluster members.
We continue using `micro1`, but you will see the same results on the others.
```
1. Inspect the cluster setup:

   ```{terminal}
   :input: lxc cluster list
   :user: root
   :host: micro1
   :scroll:

   +--------+--------------------------+------------------+--------------+----------------+-------------+--------+-------------------+
   |  NAME  |            URL           |      ROLES       | ARCHITECTURE | FAILURE DOMAIN | DESCRIPTION | STATE  |      MESSAGE      |
   +--------+--------------------------+------------------+--------------+----------------+-------------+--------+-------------------+
   | micro1 | https://10.1.123.10:8443 | database-leader  | x86_64       | default        |             | ONLINE | Fully operational |
   |        |                          | database         |              |                |             |        |                   |
   +--------+--------------------------+------------------+--------------+----------------+-------------+--------+-------------------+
   | micro2 | https://10.1.123.20:8443 | database         | x86_64       | default        |             | ONLINE | Fully operational |
   +--------+--------------------------+------------------+--------------+----------------+-------------+--------+-------------------+
   | micro3 | https://10.1.123.30:8443 | database         | x86_64       | default        |             | ONLINE | Fully operational |
   +--------+--------------------------+------------------+--------------+----------------+-------------+--------+-------------------+
   | micro4 | https://10.1.123.40:8443 | database-standby | x86_64       | default        |             | ONLINE | Fully operational |
   +--------+--------------------------+------------------+--------------+----------------+-------------+--------+-------------------+
   :input: microcloud cluster list
   +--------+------------------+----------+------------------------------------------------------------------+--------+
   |  NAME  |      ADDRESS     | ROLE     |                           FINGERPRINT                            | STATUS |
   +--------+------------------+----------+------------------------------------------------------------------+--------+
   | micro1 | 10.1.123.10:9443 | voter    | 47a74cb2ed8b844544ce71f45e96acb2c8021d4c1ffc2f1f449cdbf2f6898fd8 | ONLINE |
   +--------+------------------+----------+------------------------------------------------------------------+--------+
   | micro2 | 10.1.123.20:9443 | voter    | 56bee3adbd5e1de2186dd22788baffd5e1358e408ec3d9b713ed930741a339f2 | ONLINE |
   +--------+------------------+----------+------------------------------------------------------------------+--------+
   | micro3 | 10.1.123.30:9443 | voter    | aabdd5f64d4c2796a50d6ce9d91939f248bfeb27195426158dff05d660f93f86 | ONLINE |
   +--------+------------------+----------+------------------------------------------------------------------+--------+
   | micro4 | 10.1.123.40:9443 | stand-by | 649ec21815135104f1faa5fca099daddf995f554119c6e34706a2b31681ad1d7 | ONLINE |
   +--------+------------------+----------+------------------------------------------------------------------+--------+
   :input: microceph cluster list
   +--------+------------------+----------+------------------------------------------------------------------+--------+
   |  NAME  |      ADDRESS     | ROLE     |                           FINGERPRINT                            | STATUS |
   +--------+------------------+----------+------------------------------------------------------------------+--------+
   | micro1 | 10.1.123.10:7443 | voter    | a2b370cce1deb02437b583aa73be5e5c519aed75f02f4b98f6df150fd62c648a | ONLINE |
   +--------+------------------+----------+------------------------------------------------------------------+--------+
   | micro2 | 10.1.123.20:7443 | voter    | e37ea1acd14b984152cac4cb861cbe35ac438151233b9d0ee606c44c2e27d759 | ONLINE |
   +--------+------------------+----------+------------------------------------------------------------------+--------+
   | micro3 | 10.1.123.30:7443 | voter    | 152ccf372ecc93faffa8a6801cedd5eca49d977eea72e3f2239245cc22965399 | ONLINE |
   +--------+------------------+----------+------------------------------------------------------------------+--------+
   | micro4 | 10.1.123.40:7443 | stand-by | 9b75b396f6d59481b8c14221942d775cff4d27c5621b0b541eb5ba3245618093 | ONLINE |
   +--------+------------------+----------+------------------------------------------------------------------+--------+
   :input: microovn cluster list
   +--------+------------------+----------+------------------------------------------------------------------+--------+
   |  NAME  |      ADDRESS     | ROLE     |                           FINGERPRINT                            | STATUS |
   +--------+------------------+----------+------------------------------------------------------------------+--------+
   | micro1 | 10.1.123.10:6443 | voter    | a552d316c159a50a4e11253c36a1cd25a3902bee50e24ed1e073ee7728be0410 | ONLINE |
   +--------+------------------+----------+------------------------------------------------------------------+--------+
   | micro2 | 10.1.123.20:6443 | voter    | 2c779eb10409576a33fa01a29cede39abea61f7cd6a07837c369858b515ed02a | ONLINE |
   +--------+------------------+----------+------------------------------------------------------------------+--------+
   | micro3 | 10.1.123.30:6443 | voter    | 7f76cddfdbbe3d768c343b1a5f402842565c25d0e4e3ebbc8514263fc14ea28b | ONLINE |
   +--------+------------------+----------+------------------------------------------------------------------+--------+
   | micro4 | 10.1.123.40:6443 | stand-by | 5d62b2a63dec514c45c07b24ff93e2bd83ad8b9af4ab774aad3d2ac51ee102d5 | ONLINE |
   +--------+------------------+----------+------------------------------------------------------------------+--------+
   ```

1. Inspect the storage setup:

   ```{terminal}
   :input: lxc storage list
   :user: root
   :host: micro1
   :scroll:

   +-----------+--------+--------------------------------------------+---------+---------+
   |  NAME     | DRIVER |         DESCRIPTION                        | USED BY |  STATE  |
   +-----------+--------+--------------------------------------------+---------+---------+
   | local     | zfs    | Local storage on ZFS                       | 8       | CREATED |
   +-----------+--------+--------------------------------------------+---------+---------+
   | remote    | ceph   | Distributed storage on Ceph                | 1       | CREATED |
   +-----------+--------+--------------------------------------------+---------+---------+
   | remote-fs | cephfs | Distributed file-system storage using Ceph | 1       | CREATED |
   +-----------+--------+--------------------------------------------+---------+---------+
   :input: lxc storage info local
   info:
     description: Local storage on ZFS
     driver: zfs
     name: local
     space used: 747.00KiB
     total space: 9.20GiB
   used by:
     volumes:
     - backups (location "micro1")
     - backups (location "micro2")
     - backups (location "micro3")
     - backups (location "micro4")
     - images (location "micro1")
     - images (location "micro2")
     - images (location "micro3")
     - images (location "micro4")
   :input: lxc storage info remote
   info:
     description: Distributed storage on Ceph
     driver: ceph
     name: remote
     space used: 25.41KiB
     total space: 29.67GiB
   used by:
     profiles:
     - default
   :input: lxc storage info remote-fs
   info:
     description: Distributed file-system storage using CephFS
     driver: cephfs
     name: remote-fs
     space used: 0B
     total space: 29.67GiB
   used by: {}
   ```

1. Inspect the OVN network setup:

   ```{terminal}
   :input: lxc network list
   :user: root
   :host: micro1
   :scroll:

   +---------+----------+---------+-----------------+--------------------------+-------------+---------+---------+
   |  NAME   |   TYPE   | MANAGED |      IPV4       |           IPV6           | DESCRIPTION | USED BY |  STATE  |
   +---------+----------+---------+-----------------+--------------------------+-------------+---------+---------+
   | UPLINK  | physical | YES     |                 |                          |             | 1       | CREATED |
   +---------+----------+---------+-----------------+--------------------------+-------------+---------+---------+
   | br-int  | bridge   | NO      |                 |                          |             | 0       |         |
   +---------+----------+---------+-----------------+--------------------------+-------------+---------+---------+
   | default | ovn      | YES     | 198.51.100.1/24 | 2001:db8:d960:91cf::1/64 |             | 1       | CREATED |
   +---------+----------+---------+-----------------+--------------------------+-------------+---------+---------+
   | enp5s0  | physical | NO      |                 |                          |             | 0       |         |
   +---------+----------+---------+-----------------+--------------------------+-------------+---------+---------+
   | enp6s0  | physical | NO      |                 |                          |             | 1       |         |
   +---------+----------+---------+-----------------+--------------------------+-------------+---------+---------+
   | lxdovn1 | bridge   | NO      |                 |                          |             | 0       |         |
   +---------+----------+---------+-----------------+--------------------------+-------------+---------+---------+
   :input: lxc network show default
   config:
     bridge.mtu: "1442"
     ipv4.address: 198.51.100.1/24
     ipv4.nat: "true"
     ipv6.address: 2001:db8:d960:91cf::1/64
     ipv6.nat: "true"
     network: UPLINK
     volatile.network.ipv4.address: 192.0.2.100
     volatile.network.ipv6.address: 2001:db8:e647:610d:216:3eff:fe96:ed5c
   description: ""
   name: default
   type: ovn
   used_by:
   - /1.0/profiles/default
   managed: true
   status: Created
   locations:
   - micro1
   - micro3
   - micro2
   - micro4
   ```

1. Make sure that you can ping the virtual router within OVN.

   1. Within the output of the previous command (`lxc network show default`), find the value for `volatile.network.ipv4.address`. This is the virtual router's IPv4 address.

   1. Ping that IPv4 address.
   ```{terminal}
   :input: ping 192.0.2.100
   :user: root
   :host: micro1
   :scroll:

   PING 192.0.2.100 (192.0.2.100) 56(84) bytes of data.
   64 bytes from 192.0.2.100: icmp_seq=1 ttl=253 time=2.05 ms
   64 bytes from 192.0.2.100: icmp_seq=2 ttl=253 time=2.01 ms
   64 bytes from 192.0.2.100: icmp_seq=3 ttl=253 time=1.78 ms
   ^C
   --- 192.0.2.100 ping statistics ---
   4 packets transmitted, 3 received, 25% packet loss, time 3005ms
   rtt min/avg/max/mdev = 1.777/1.945/2.052/0.120 ms
   :input: ping6 -n 2001:db8:e647:610d:216:3eff:fe96:ed5c
   PING 2001:db8:e647:610d:216:3eff:fe96:ed5c(2001:db8:e647:610d:216:3eff:fe96:ed5c) 56 data bytes
   64 bytes from 2001:db8:e647:610d:216:3eff:fe96:ed5c: icmp_seq=1 ttl=253 time=1.61 ms
   64 bytes from 2001:db8:e647:610d:216:3eff:fe96:ed5c: icmp_seq=2 ttl=253 time=1.99 ms
   64 bytes from 2001:db8:e647:610d:216:3eff:fe96:ed5c: icmp_seq=3 ttl=253 time=15.7 ms
   ^C
   --- 2001:db8:e647:610d:216:3eff:fe96:ed5c ping statistics ---
   3 packets transmitted, 3 received, 0% packet loss, time 2004ms
   rtt min/avg/max/mdev = 1.606/6.432/15.704/6.558 ms
   ```

1. Inspect the default profile:

   ```{terminal}
   :input: lxc profile show default
   :user: root
   :host: micro1
   :scroll:

   config: {}
   description: ""
   devices:
     eth0:
       name: eth0
       network: default
       type: nic
     root:
       path: /
       pool: remote
       type: disk
   name: default
   used_by: []
   ```

## 8. Launch some instances

Now that your MicroCloud cluster is ready to use, let's launch a few instances:

1. Launch an Ubuntu container with the default settings:

       lxc launch ubuntu:22.04 u1

1. Launch another Ubuntu container, but use local storage instead of the default remote storage:

       lxc launch ubuntu:22.04 u2 --storage local

1. Launch an Ubuntu VM:

       lxc launch ubuntu:22.04 u3 --vm

1. Check the list of instances.
   Note that the instances are running on different cluster members.

   ```{terminal}
   :input: lxc list
   :user: root
   :host: micro1
   :scroll:

   +------+---------+---------------------+----------------------------------------------+-----------------+-----------+----------+
   | NAME |  STATE  |        IPV4         |                     IPV6                     |      TYPE       | SNAPSHOTS | LOCATION |
   +------+---------+---------------------+----------------------------------------------+-----------------+-----------+----------+
   | u1   | RUNNING | 198.51.100.2 (eth0) | 2001:db8:d960:91cf:216:3eff:fe4e:9642 (eth0) | CONTAINER       | 0         | micro1   |
   +------+---------+---------------------+----------------------------------------------+-----------------+-----------+----------+
   | u2   | RUNNING | 198.51.100.3 (eth0) | 2001:db8:d960:91cf:216:3eff:fe79:6765 (eth0) | CONTAINER       | 0         | micro3   |
   +------+---------+---------------------+----------------------------------------------+-----------------+-----------+----------+
   | u3   | RUNNING | 198.51.100.4 (eth0) | 2001:db8:d960:91cf:216:3eff:fe66:f24b (eth0) | VIRTUAL-MACHINE | 0         | micro2   |
   +------+---------+---------------------+----------------------------------------------+-----------------+-----------+----------+
   ```

1. Check the storage.
   Note that the instance volumes are located on the specified storage pools.

   ```{terminal}
   :input: lxc storage volume list remote
   :user: root
   :host: micro1
   :scroll:

   +-----------------+------------------------------------------------------------------+-------------+--------------+---------+----------+
   | TYPE | NAME | DESCRIPTION | CONTENT-TYPE | USED BY | LOCATION |
   +-----------------+------------------------------------------------------------------+-------------+--------------+---------+----------+
   | container       | u1                                                               |             | filesystem   | 1       |          |
   +-----------------+------------------------------------------------------------------+-------------+--------------+---------+----------+
   | image           | 17fbc145125c659b7ef926b2de5e5304370083e28846f084a0d514c7a96777bc |             | block        | 1       |          |
   +-----------------+------------------------------------------------------------------+-------------+--------------+---------+----------+
   | image           | 45613e262f8a5fc9467330f679862147c289516f045e3edc313e07ebcb0aab4a |             | filesystem   | 1       |          |
   +-----------------+------------------------------------------------------------------+-------------+--------------+---------+----------+
   | virtual-machine | u3                                                               |             | block        | 1       |          |
   +-----------------+------------------------------------------------------------------+-------------+--------------+---------+----------+
   :input: lxc storage volume list local
   +-----------+------------------------------------------------------------------+-------------+--------------+---------+----------+
   |   TYPE    |                               NAME                               | DESCRIPTION | CONTENT-TYPE | USED BY | LOCATION |
   +-----------+------------------------------------------------------------------+-------------+--------------+---------+----------+
   | container | u2                                                               |             | filesystem   | 1       | micro3   |
   +-----------+------------------------------------------------------------------+-------------+--------------+---------+----------+
   | custom    | backups                                                          |             | filesystem   | 1       | micro2   |
   +-----------+------------------------------------------------------------------+-------------+--------------+---------+----------+
   | custom    | backups                                                          |             | filesystem   | 1       | micro3   |
   +-----------+------------------------------------------------------------------+-------------+--------------+---------+----------+
   | custom    | backups                                                          |             | filesystem   | 1       | micro4   |
   +-----------+------------------------------------------------------------------+-------------+--------------+---------+----------+
   | custom    | backups                                                          |             | filesystem   | 1       | micro1   |
   +-----------+------------------------------------------------------------------+-------------+--------------+---------+----------+
   | custom    | images                                                           |             | filesystem   | 1       | micro2   |
   +-----------+------------------------------------------------------------------+-------------+--------------+---------+----------+
   | custom    | images                                                           |             | filesystem   | 1       | micro3   |
   +-----------+------------------------------------------------------------------+-------------+--------------+---------+----------+
   | custom    | images                                                           |             | filesystem   | 1       | micro4   |
   +-----------+------------------------------------------------------------------+-------------+--------------+---------+----------+
   | custom    | images                                                           |             | filesystem   | 1       | micro1   |
   +-----------+------------------------------------------------------------------+-------------+--------------+---------+----------+
   | image     | 45613e262f8a5fc9467330f679862147c289516f045e3edc313e07ebcb0aab4a |             | filesystem   | 1       | micro3   |
   +-----------+------------------------------------------------------------------+-------------+--------------+---------+----------+
   ```

## 9. Inspect your networking

The instances that you have launched are all on the same subnet.
You can, however, create a different network to isolate some instances from others.

1. Check the list of running instances:

   ```{terminal}
   :input: lxc list
   :user: root
   :host: micro1
   :scroll:

   +------+---------+---------------------+----------------------------------------------+-----------------+-----------+----------+
   | NAME |  STATE  |        IPV4         |                    IPV6                      |      TYPE       | SNAPSHOTS | LOCATION |
   +------+---------+---------------------+----------------------------------------------+-----------------+-----------+----------+
   | u1   | RUNNING | 198.51.100.2 (eth0) | 2001:db8:d960:91cf:216:3eff:fe4e:9642 (eth0) | CONTAINER       | 0         | micro1   |
   +------+---------+---------------------+----------------------------------------------+-----------------+-----------+----------+
   | u2   | RUNNING | 198.51.100.3 (eth0) | 2001:db8:d960:91cf:216:3eff:fe79:6765 (eth0) | CONTAINER       | 0         | micro3   |
   +------+---------+---------------------+----------------------------------------------+-----------------+-----------+----------+
   | u3   | RUNNING | 198.51.100.4 (eth0) | 2001:db8:d960:91cf:216:3eff:fe66:f24b (eth0) | VIRTUAL-MACHINE | 0         | micro2   |
   +------+---------+---------------------+----------------------------------------------+-----------------+-----------+----------+
   ```

1. Access the shell in `u1`:

       lxc exec u1 -- bash

1. Ping the IPv4 address of `u2`:

   ```{terminal}
   :input: ping 198.51.100.3
   :user: root
   :host: u1
   :scroll:

   PING 198.51.100.3 (198.51.100.3) 56(84) bytes of data.
   64 bytes from 198.51.100.3: icmp_seq=1 ttl=64 time=1.33 ms
   64 bytes from 198.51.100.3: icmp_seq=2 ttl=64 time=1.74 ms
   64 bytes from 198.51.100.3: icmp_seq=3 ttl=64 time=0.985 ms
   ^C
   --- 198.51.100.3 ping statistics ---
   3 packets transmitted, 3 received, 0% packet loss, time 2004ms
   rtt min/avg/max/mdev = 0.985/1.352/1.739/0.308 ms
   ```

1. Ping the IPv6 address of `u3`:

   ```{terminal}
   :input: ping6 -n 2001:db8:d960:91cf:216:3eff:fe66:f24b
   :user: root
   :host: u1
   :scroll:

   PING 2001:db8:d960:91cf:216:3eff:fe66:f24b(2001:db8:d960:91cf:216:3eff:fe66:f24b) 56 data bytes
   64 bytes from 2001:db8:d960:91cf:216:3eff:fe66:f24b: icmp_seq=1 ttl=64 time=16.8 ms
   64 bytes from 2001:db8:d960:91cf:216:3eff:fe66:f24b: icmp_seq=2 ttl=64 time=3.41 ms
   64 bytes from 2001:db8:d960:91cf:216:3eff:fe66:f24b: icmp_seq=3 ttl=64 time=3.86 ms
   ^C
   --- 2001:db8:d960:91cf:216:3eff:fe66:f24b ping statistics ---
   3 packets transmitted, 3 received, 0% packet loss, time 2004ms
   rtt min/avg/max/mdev = 3.407/8.012/16.774/6.197 ms
   ```

1. Confirm that the instance has connectivity to the outside world:

   ```{terminal}
   :input: ping www.example.com
   :user: root
   :host: u1
   :scroll:

   PING www.example.com (93.184.216.34) 56(84) bytes of data.
   64 bytes from 93.184.216.34 (93.184.216.34): icmp_seq=1 ttl=49 time=111 ms
   64 bytes from 93.184.216.34 (93.184.216.34): icmp_seq=2 ttl=49 time=95.2 ms
   64 bytes from 93.184.216.34 (93.184.216.34): icmp_seq=3 ttl=49 time=96.2 ms
   ^C
   --- www.example.com ping statistics ---
   3 packets transmitted, 3 received, 0% packet loss, time 2018ms
   rtt min/avg/max/mdev = 95.233/100.870/111.165/7.290 ms
   ```

1. Log out of the `u1` shell:

       exit

1. Create an OVN network with the default settings:

       lxc network create isolated --type=ovn

   There is only one `UPLINK` network, so the new network uses this one as its parent.

1. Show information about the new network:

   ```{terminal}
   :input: lxc network show isolated
   :user: root
   :host: micro1
   :scroll:

   config:
     bridge.mtu: "1442"
     ipv4.address: 198.51.100.201/24
     ipv4.nat: "true"
     ipv6.address: 2001:db8:452a:32b2::1/64
     ipv6.nat: "true"
     network: UPLINK
     volatile.network.ipv4.address: 192.0.2.101
     volatile.network.ipv6.address: 2001:db8:e647:610d:216:3eff:feef:6361
   description: ""
   name: isolated
   type: ovn
   used_by: []
   managed: true
   status: Created
   locations:
   - micro1
   - micro3
   - micro2
   - micro4
   ```

1. Check that you can ping the `volatile.network.ipv4.address`:

   ```{terminal}
   :input: ping 192.0.2.101
   :user: root
   :host: micro1
   :scroll:

   PING 192.0.2.101 (192.0.2.101) 56(84) bytes of data.
   64 bytes from 192.0.2.101: icmp_seq=1 ttl=253 time=1.25 ms
   64 bytes from 192.0.2.101: icmp_seq=2 ttl=253 time=1.04 ms
   64 bytes from 192.0.2.101: icmp_seq=3 ttl=253 time=1.68 ms
   ^C
   --- 192.0.2.101 ping statistics ---
   3 packets transmitted, 3 received, 0% packet loss, time 2002ms
   rtt min/avg/max/mdev = 1.042/1.321/1.676/0.264 ms
   ```

1. Launch an Ubuntu container that uses the new network:

       lxc launch ubuntu:22.04 u4 --network isolated

1. Access the shell in `u4`:

       lxc exec u4 -- bash

1. Confirm that the instance has connectivity to the outside world:

   ```{terminal}
   :input: ping www.example.com
   :user: root
   :host: u4
   :scroll:

   PING www.example.com (93.184.216.34) 56(84) bytes of data.
   64 bytes from 93.184.216.34 (93.184.216.34): icmp_seq=1 ttl=49 time=95.6 ms
   64 bytes from 93.184.216.34 (93.184.216.34): icmp_seq=2 ttl=49 time=118 ms
   64 bytes from 93.184.216.34 (93.184.216.34): icmp_seq=3 ttl=49 time=94.6 ms
   ^C
   --- www.example.com ping statistics ---
   3 packets transmitted, 3 received, 0% packet loss, time 2004ms
   rtt min/avg/max/mdev = 94.573/102.587/117.633/10.646 ms
   ```

1. Ping the IPv4 address of `u2`:

   ```{terminal}
   :input: ping 198.51.100.3
   :user: root
   :host: u4
   :scroll:

   PING 198.51.100.3 (198.51.100.3) 56(84) bytes of data.
   ^C
   --- 198.51.100.3 ping statistics ---
   14 packets transmitted, 0 received, 100% packet loss, time 13301ms
   ```

   The ping fails; `u2` is not reachable because it is on a different OVN subnet.

## 10. Access the UI

Instead of managing your instances and your LXD setup from the command line, you can also use the LXD UI.
See {ref}`lxd:access-ui` for more information.

1. Check the LXD cluster list to determine the IP addresses of the cluster members:

   ```{terminal}
   :input: lxc cluster list
   :user: root
   :host: micro1
   :scroll:

   +--------+--------------------------+------------------+--------------+----------------+-------------+--------+-------------------+
   |  NAME  |            URL           |      ROLES       | ARCHITECTURE | FAILURE DOMAIN | DESCRIPTION | STATE  |      MESSAGE      |
   +--------+--------------------------+------------------+--------------+----------------+-------------+--------+-------------------+
   | micro1 | https://10.1.123.10:8443 | database-leader  | x86_64       | default        |             | ONLINE | Fully operational |
   |        |                          | database         |              |                |             |        |                   |
   +--------+--------------------------+------------------+--------------+----------------+-------------+--------+-------------------+
   | micro2 | https://10.1.123.20:8443 | database         | x86_64       | default        |             | ONLINE | Fully operational |
   +--------+--------------------------+------------------+--------------+----------------+-------------+--------+-------------------+
   | micro3 | https://10.1.123.30:8443 | database         | x86_64       | default        |             | ONLINE | Fully operational |
   +--------+--------------------------+------------------+--------------+----------------+-------------+--------+-------------------+
   | micro4 | https://10.1.123.40:8443 | database-standby | x86_64       | default        |             | ONLINE | Fully operational |
   +--------+--------------------------+------------------+--------------+----------------+-------------+--------+-------------------+
   ```

1. In your web browser, navigate to the URL of one of the cluster members.
   For example, for `micro1`, navigate to `https://10.1.123.10:8443`.

1. By default, MicroCloud uses a self-signed certificate, which will cause a security warning in your browser.
   Use your browser’s mechanism to continue despite the security warning.

   ```{figure} /images/ui_security_warning.png
   :alt: Example for a security warning in Chrome

   Example for a security warning in Chrome
   ```

1. You should now see the LXD UI, prompting you to set up a certificate.

   ```{figure} /images/ui_certificate_selection.png
   :alt: Certificate selection in the LXD UI

   Certificate selection in the LXD UI
   ```

   ```{note}
   Since LXD 5.21, the LXD UI is enabled by default.

   If you don't see the certificate screen, you might have an earlier version of LXD. Run `snap info lxd` to check.
   If you have an earlier version, run the following commands on the cluster member that you're trying to access (for example, `micro1`) to enable the UI:

       snap set lxd ui.enable=true
       systemctl reload snap.lxd.daemon
   ```
1. Follow the instructions in the UI to set up the certificates.

    ```{tip}
    If you create a new certificate, you must transfer it to one of the cluster members to add it to the trust store.

    To do this, use the {ref}`file push command <lxd:instances-access-files-push>`.
    For example:

        lxc file push lxd-ui.crt micro1/root/lxd-ui.crt

    You can then access the shell on that cluster member and add the certificate to the trust store:

        lxc exec micro1 -- bash
        lxc config trust add lxd-ui.crt
    ```

1. You can now browse the UI and inspect, for example, the instances you created and the networks and storage that MicroCloud set up.

   ```{figure} /images/ui_instances.png
   :alt: Instances view in the LXD UI

   Instances view in the LXD UI
   ```

## Next steps

Now that your MicroCloud is up and running, you can start using it!

If you're already familiar with LXD, see {ref}`howto-commands` for a reference of the most common commands.

If you're new to LXD, check out the {ref}`LXD tutorials <lxd:tutorials>` to familiarize yourself with what you can do in LXD:

- {ref}`lxd:tutorial-ui` guides you through common operations in LXD, using the UI.
- {ref}`lxd:first-steps` goes through the same functionality, but using the CLI.

In both tutorials, you can skip the first section about installing and initializing LXD, because LXD is already operational as part of your MicroCloud setup.
