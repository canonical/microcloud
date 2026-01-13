---
myst:
  html_meta:
    description: Step-by-step tutorial to learn the basics of how to install, initialize, and use MicroCloud using a physical machine as a single cluster member.
---

(tutorial-single)=
# MicroCloud tutorial using a single physical cluster member

This tutorial guides you in the installation, initialization, and basic usage of MicroCloud on a single physical machine.

First, review the requirements listed below. Once you can meet the requirements, follow the instructions to install MicroCloud and its components.

After that, we'll lead you step by step through the MicroCloud initialization process. You will learn how to configure storage and networking, including the uplink and internal networks.

Once you have initialized MicroCloud, you'll try out its basic usage. You'll learn how to find information about the cluster, including its storage and networking details. You'll then start a few instances and confirm that they can communicate with each other. Finally, you'll learn how to access the UI for cluster management through a graphical interface.

(tutorial-single-requirements)=
## Requirements

If you cannot meet the requirements below, try the {ref}`multi-member cluster tutorial<tutorial-multi>`, which provides step-by-step instructions within a confined environment with fewer prerequisites.

(tutorial-single-requirements-general)=
### General

- Use a physical machine for the tutorial with at least 2 GiB of RAM available. This is less than the recommended hardware requirement, but it should be sufficient for this tutorial.
- Ensure that the machine meets the requirements listed in the **General** tab in the {ref}`pre-deployment-requirements`.
- If Docker is installed, uninstall or disable it. Docker can cause networking issues when installed alongside LXD. If you must leave Docker enabled, visit this page for other options: {ref}`network-lxd-docker`.
- MicroCloud cannot be initialized on a machine that already has LXD initialized. If you have an initialized installation of LXD, you must remove it completely. If LXD was installed using its snap, use this command to purge it from your system:

  ```bash
  sudo snap remove --purge lxd
  ```

- If LXD is installed but not initialized, you do not need to remove it. However, ensure that it is running the most recent LTS version. See the LXD documentation: {ref}`lxd:howto-snap`.
- If MicroCeph or MicroOVN are already installed, ensure that they are also running their most recent LTS versions. To ensure that the versions are compatible for MicroCloud, refer to: {ref}`ref-releases-matrix`.
- Due to variation in physical machine setups, it is beyond the scope of this tutorial to instruct you on how to set up your network interfaces and storage disks. Thus, this tutorial requires a higher level of knowledge of server management on your part. If you require step-by-step instructions, follow the {ref}`multi-member cluster tutorial<tutorial-multi>` instead.

(tutorial-single-requirements-storage)=
### Storage requirements

MicroCloud supports both local and remote storage. For remote storage (also called distributed storage), you need one additional physical disk attached to the machine, such as an external SSD. This disk must be free of partitions and file systems, and able to be wiped. At least 10 GiB of storage space is recommended.

To configure local storage as well, you'll need a second disk attached to that machine that meets the same requirements. If you only have one additional disk to use, you must use it for remote storage.

(tutorial-single-requirements-network)=
### Network requirements

Two network interfaces must be configured on your machine, such as with a dual-port NIC:

- One interface is used for an uplink network that provides external connectivity to cluster members. The network interface for the uplink network must support both broadcast and multicast, and it must not have any IP addresses bound directly to it.
- The other interface is used for internal (or intra-cluster) communication, meaning communication between MicroCloud cluster members. It must have assigned IPs.

  Even though we are setting up a cluster with a single member, this network is still required by MicroCloud. This is because it uses an address on this network to bind its services, as well as those of its components.

During the MicroCloud initialization process, you'll be asked for the IPv4 and IPv6 gateway addresses for the uplink network. Be prepared to provide at least one; you can optionally provide both. If you provide the IPv4 gateway address, you'll need to provide the IPv4 subnet ranges as well.

(tutorial-single-install)=
## Install MicroCloud and its components

Install the required snaps:

```bash
sudo snap install lxd microceph microovn microcloud --cohort="+"
```

```{admonition} About the cohort flag
:class: note
The `--cohort="+"` flag in the command ensures that the same version of the snap is installed on all cluster members.
See {ref}`howto-update-sync` for more information.
```

(tutorial-single-hold-updates)=
## Hold updates on the snaps

All MicroCloud cluster members run the same versions of each of its component snaps. Since snaps auto-update by default, this can cause issues if one cluster member updates before another one.

Thus, whenever you install MicroCloud and its components, pause automatic updates of the snaps so that you can ensure they are all updated together during a maintenance window that you control:

```bash
sudo snap refresh lxd microceph microovn microcloud --hold
```

For more information, see {ref}`howto-update-hold`.

(tutorial-single-init)=
## Initialize MicroCloud

The initialization process sets up LXD, MicroCeph, and MicroOVN to work together as a MicroCloud. When you initialize MicroCloud, you can optionally set up the MicroCloud cluster using a join mechanism to add multiple machines as cluster members. In this tutorial, we will set up a single cluster member during initialization; you can optionally add more cluster members afterward. The initialization also sets up MicroCloud storage and network configurations.

For a detailed look at the initialization process, see: {ref}`explanation-initialization`.

```{tip}
In this tutorial, we initialize MicroCloud interactively. Later, you might want to look into using a preseed file for {ref}`howto-initialize-preseed` to automate deployment with a pre-defined configuration.
```

Start the interactive initialization process:

```bash
sudo microcloud init
```

MicroCloud will ask if you want to set up more than one cluster member. Enter `no`:

```{terminal}
:input: sudo microcloud init
:user: ubuntu
:host: micro1
Waiting for services to start ...
Do you want to set up more than one cluster member? (yes/no) [default=yes]: no
```

(tutorial-single-init-network-internal)=
### Configure the internal network

Next, MicroCloud needs to configure the network interface to use for internal traffic. If there is only one interface suitable for internal traffic, MicroCloud detects its address automatically and shows output similar to the following:

```{terminal}
 Using address 192.0.2.10 for MicroCloud
```

If there are multiple suitable interfaces, MicroCloud instead shows a list of options and asks you to make a selection. Select the IP address of the network interface you want to use for this from the listed options.

(tutorial-single-init-storage)=
### Configure storage

Next, MicroCloud will ask about local storage:

```{terminal}
Would you like to set up local storage? (yes/no) [default=yes]:
```

If you only have one disk available for storage, then enter `no`, as you will need it for distributed storage. In this case, ignore any instructions related to local storage in the remainder of this tutorial.

If you have more than one disk, then press {kbd}`Enter` to accept the default of `yes`, then choose the disk you want to use for local storage.

Next, MicroCloud will ask you to select which disks to wipe. If the disk you chose is not empty, then select it to be wiped. Otherwise, press {kbd}`Enter` to skip wiping the disk.

Next, MicroCloud will ask about distributed storage:

```{terminal}
Would you like to set up distributed storage? (yes/no) [default=yes]:
```

Press {kbd}`Enter` to accept the default of `yes`, then choose the disk you want to use for distributed storage.

MicroCloud will again ask you to select which disks to wipe. If the disk you chose is not empty, then select it to be wiped. Otherwise, press {kbd}`Enter` to skip wiping the disk.

You'll see the following error message, and you'll be asked if you want to change the disk selection. Do not accept the default this time. Enter `no`:

```{terminal}
! Warning: Disk configuration does not meet recommendations for fault tolerance. At least 3 systems must supply disks (1 currently supplying). Continuing with this configuration will inhibit MicroCloud's ability to retain data on system failure
Change disk selection? (yes/no) [default=yes]: no
```

A working distributed storage setup requires a multi-member cluster with at least three cluster members providing storage disks for fault tolerance. Since this is only a test setup, you can disregard this error and continue with the disk you selected instead of changing it.

Next, MicroCloud will ask about disk encryption:

```{terminal}
Do you want to encrypt the selected disks? (yes/no) [default=no]:
```

Press {kbd}`Enter` to accept the default of `no`. You do not need to encrypt them for this tutorial.

(tutorial-single-init-storage-cephfs)=
#### Configure CephFS remote storage

The next set of questions configures {ref}`CephFS remote storage <lxd:storage-cephfs>`, which creates a distributed file system over a MicroCeph storage cluster.

First, MicroCloud will ask if you want to set up this feature:

```{terminal}
Would you like to set up CephFS remote storage? (yes/no) [default=yes]:
```

Press {kbd}`Enter` to accept the default of `yes`.

```{admonition} If you did not set up local storage
:class: note

If you did not set up local storage _and_ you set up CephFS storage, there will be an additional step for you at the end of the initialization process.
```

Next, you'll be prompted to select the CIDR subnet for Ceph internal and public traffic:

```{terminal}
What subnet (IPv4/IPv6 CIDR) would you like your Ceph internal traffic on? [default=192.0.2.0/24]:
What subnet (either IPv4 or IPv6 CIDR notation) would you like your Ceph public traffic on? [default=192.0.2.0/24]:
```

By default, MicroCloud uses your internal network for both. Press {kbd}`Enter` twice to accept the default for both questions.

```{admonition} Using other networks for Ceph
:class: note
MicroCloud and MicroCeph support using separate networks for Ceph internal and public traffic if needed. We don't need this for the purposes of this tutorial, but if you'd like to know more, see: {ref}`howto-ceph-networking`.
```

(tutorial-single-init-network-uplink)=
### Configure the uplink network

Next, you'll set up the uplink network that provides external connectivity from your cluster members to other networks, such as the internet. This uplink network is configured with MicroOVN, a minimal wrapper around the OVN (Open Virtual Network) project. For more information about OVN networking, see {ref}`exp-networking`.

MicroCloud will ask:

```{terminal}
Configure distributed networking? (yes/no) [default=yes]:
```

Press {kbd}`Enter` for `yes`.

You'll then be prompted to select the network interface to use for external connectivity (also known as the uplink network). Select the interface that has been configured according to the {ref}`tutorial-single-requirements-network` provided earlier in this tutorial.

Next, MicroCloud will ask you to enter the IPv4 gateway address for your uplink network, in CIDR format as shown in the example below. Be sure to replace the example IPv4 gateway for your own uplink network.

```{terminal}
Specify the IPv4 gateway (CIDR) on the uplink network: 198.51.100.1/24
```

Next, you'll be asked to enter the first and last IPv4 addresses to use from the uplink network's subnet. Replace the addresses in the example below with your own:

```{terminal}
Specify the first IPv4 address in the range to use on the uplink network: 198.51.100.100
Specify the last IPv4 address in the range to use on the uplink network: 198.51.100.200
```

MicroCloud will ask for an IPv6 gateway as well, in CIDR format:

```{terminal}
Specify the IPv6 gateway (CIDR) on the uplink network (empty to skip IPv6):
```

Enter the address. If you do not have an IPv6 address, press {kbd}`Enter` to skip this question.

Finally, MicroCloud will ask for the DNS addresses for domain name resolution. Example:

```{terminal}
Specify the DNS addresses (comma-separated IPv4 / IPv6 addresses) for the distributed network (default: 198.51.100.1,2001:db8:100::1):
```

By default, MicroCloud will suggest that you use the IPv4 and IPv6 gateways for this. However, these gateways will only perform DNS if you have configured them to run a DNS forwarder. If so, or if you are not concerned about resolving domain names at this time, you can press {kbd}`Enter` to accept the defaults.

Otherwise, you can optionally enter the address of an external trusted DNS resolver, such as `1.1.1.1` (Cloudflare) or `8.8.8.8` (Google). If you do not enter an address that can resolve DNS, your MicroCloud cluster will still function in all other ways.

(tutorial-single-init-complete)=
### Complete the initialization

Finally, MicroCloud will ask about underlay networking:

```{terminal}
Configure dedicated underlay networking? (yes/no) [default=no]:
```

This refers to an OVN underlay network. Press {kbd}`Enter` to accept the default `no` value.

```{admonition} Using a dedicated underlay network
:class: note
MicroCloud and MicroOVN support using a dedicated underlay network for OVN traffic. We don't need this for the purposes of this tutorial, but if you'd like to know more, see: {ref}`howto-ovn-underlay`.
```

You should then see the following output:

```{terminal}
Initializing new services ...
 Local MicroCloud is ready
 Local MicroOVN is ready
 Local MicroCeph is ready
 Local LXD is ready
Awaiting cluster formation ...
Configuring cluster-wide devices ...
MicroCloud is ready
```

```{admonition} Possible final step
:class: note
If you initialized MicroCloud without local storage _and_ with CephFS storage, visit the {ref}`howto-initialize-images-backups` how-to guide to complete your initialization, then continue this tutorial.
```

(tutorial-single-inspect)=
##  Inspect your MicroCloud setup

Now that MicroCloud is initialized, we can view information about the cluster, including its storage and networking.

(tutorial-single-inspect-list)=
### List cluster information

Since MicroCloud and its components all have their own CLI tools, you can list cluster information through each of those tools:

```bash
lxc cluster list
sudo microcloud cluster list
sudo microceph cluster list
sudo microovn cluster list
```

Try them out, one at a time, and observe what information is provided by each tool. You should see outputs similar to what is shown below. Note how each cluster uses the same IP address for its internal network, but different ports.


```{terminal}
:input: lxc cluster list
:user: ubuntu
:host: micro1
:scroll:

To start your first container, try: lxc launch ubuntu:24.04
Or for a virtual machine: lxc launch ubuntu:24.04 --vm
+--------+--------------------------+-----------------+--------------+----------------+-------------+--------+-------------------+
|  NAME  |           URL            |      ROLES      | ARCHITECTURE | FAILURE DOMAIN | DESCRIPTION | STATE  |      MESSAGE      |
+--------+--------------------------+-----------------+--------------+----------------+-------------+--------+-------------------+
| micro1 | https://192.0.2.10:8443  | database-leader | x86_64      | default        |             | ONLINE | Fully operational |
|        |                          | database        |              |                |             |        |                   |
+--------+--------------------------+-----------------+--------------+----------------+-------------+--------+-------------------+

:input: sudo microcloud cluster list
┌────────┬──────────────────┬───────┬──────────────────────────────────────────────────────────────────┬────────┐
│  NAME  │     ADDRESS      │ ROLE  │                           FINGERPRINT                            │ STATUS │
├────────┼──────────────────┼───────┼──────────────────────────────────────────────────────────────────┼────────┤
│ micro1 │ 192.0.2.10:9443  │ voter │ d665dbe177f6da7f3fa8df2469956b61b6b5ec28b3f1967ac2270cfe8ee2f3ad │ ONLINE │
└────────┴──────────────────┴───────┴──────────────────────────────────────────────────────────────────┴────────┘

:input: sudo microceph cluster list
+--------+------------------+-------+------------------------------------------------------------------+--------+
|  NAME  |     ADDRESS      | ROLE  |                           FINGERPRINT                            | STATUS |
+--------+------------------+-------+------------------------------------------------------------------+--------+
| micro1 | 192.0.2.10:7443 | voter | dd4c358ee0f298775c7a71b903925bc6ce2ba8110662aa0ff8521cc680fd63df | ONLINE |
+--------+------------------+-------+------------------------------------------------------------------+--------+

:input: sudo microovn cluster list
+--------+------------------+-------+------------------------------------------------------------------+--------+
|  NAME  |     ADDRESS      | ROLE  |                           FINGERPRINT                            | STATUS |
+--------+------------------+-------+------------------------------------------------------------------+--------+
| micro1 | 192.0.2.10:6443 | voter | fba074b4f3959eb8b16b8785c2ae17c45ed800c06d1fca5c02e7a0e980c271ff | ONLINE |
+--------+------------------+-------+------------------------------------------------------------------+--------+
```

Continuing on, we will generally use the `lxc` CLI tool to interact with the MicroCloud cluster. Note that any time you use this CLI, you will likely see a notice similar to this:

```bash
To start your first container, try: lxc launch ubuntu:24.04
Or for a virtual machine: lxc launch ubuntu:24.04 --vm
```

Disregard this notice for now. After we finish inspecting the cluster, we'll move on to launching containers and VMs.

(tutorial-single-inspect-storage)=
### Inspect storage

Let's inspect the storage that we have set up. First, list all the storage pools:

```bash
lxc storage list
```

You should see a pool for each storage option that you set up during MicroCloud initialization. If you did not configure local storage, you can expect that the list will not contain a local storage pool.

```{terminal}
:input: lxc storage list
:user: ubuntu
:host: micro1
+-----------+--------+----------------------------------------------+---------+---------+
|   NAME    | DRIVER |                 DESCRIPTION                  | USED BY |  STATE  |
+-----------+--------+----------------------------------------------+---------+---------+
| local     | zfs    | Local storage on ZFS                         | 2       | CREATED |
+-----------+--------+----------------------------------------------+---------+---------+
| remote    | ceph   | Distributed storage on Ceph                  | 1       | CREATED |
+-----------+--------+----------------------------------------------+---------+---------+
| remote-fs | cephfs | Distributed file-system storage using CephFS | 0       | CREATED |
+-----------+--------+----------------------------------------------+---------+---------+
```

Use the following command to view details on any of your storage pools:

```{terminal}
:input: lxc storage info <storage pool name>
:user: ubuntu
:host: micro1
```

Example:

```bash
lxc storage info remote
```

This displays information such as the total and available storage, as well as the instances using the pool, if any.

(tutorial-single-inspect-networking)=
### Inspect the OVN networking setup

Run the following command, then note which network is listed as an OVN network.

```bash
lxc network list
```

You'll see output similar to this, along with any additional network interfaces on your machine:

```{terminal}
:input: lxc network list
:user: ubuntu
:host: micro1
:scroll:

+---------+----------+---------+----------------+---------------------------+---------------------+---------+---------+
|  NAME   |   TYPE   | MANAGED |     IPV4       |           IPV6            |     DESCRIPTION     | USED BY |  STATE  |
+---------+----------+---------+----------------+---------------------------+---------------------+---------+---------+
| UPLINK  | physical | YES     |                |                           |                     | 1       | CREATED |
+---------+----------+---------+----------------+---------------------------+---------------------+---------+---------+
| br-int  | bridge   | NO      |                |                           |                     | 0       |         |
+---------+----------+---------+----------------+---------------------------+---------------------+---------+---------+
| default | ovn      | YES     | 203.0.113.1/24 | fd42:6194:adfb:d034::1/64 | Default OVN network | 1       | CREATED |
+---------+----------+---------+---------------+---------------------------+---------------------+---------+---------+
```

Typically, the OVN network used by MicroCloud is named `default`. Let's take a look at some information about this network:

```bash
lxc network show default
```

You should see output similar to this:

```{terminal}
:input: lxc network show default
:user: ubuntu
:host: micro1

name: default
description: Default OVN network
type: ovn
managed: true
status: Created
config:
  bridge.mtu: "1442"
  ipv4.address: 203.0.113.1/24
  ipv4.nat: "true"
  ipv6.address: 2001:db8:113:1::1/64
  ipv6.nat: "true"
  network: UPLINK
  volatile.network.ipv4.address: 198.51.100.100
used_by:
- /1.0/profiles/default
locations:
- micro1
project: default
```

The OVN network spans across all MicroCloud cluster members and handles internal traffic. It also provides external connectivity to MicroCloud instances by connecting to the uplink network via a virtual router. This virtual router is active on only one cluster member at a time. When there are multiple cluster members, if the cluster member with the virtual router goes offline, the virtual router can migrate to a different cluster member to ensure uplink connectivity. For details, see: {ref}`exp-networking-ovn-architecture`.

Within the output of the previous command (`lxc network show default`), find the value for `volatile.network.ipv4.address`. It should match the first IPv4 address in the subnet range you provided for the uplink network during configuration. This is the IP address for the virtual router.

Confirm that you can ping the virtual router:

```{terminal}
:user: ubuntu
:host: micro1
:input: ping -c 4 <IPv4 address of virtual router>
```

(tutorial-single-launch-instances)=
## Launch some instances

Now that your MicroCloud is ready to use, let's launch a few instances on it.

Launching an instance means to create and start a {ref}`containers or VMs<lxd:containers-and-vms>` from a LXD image. We'll launch a container using distributed storage (the default), a container using local storage, and a virtual machine.

First, launch an Ubuntu container with the default settings:

```bash
lxc launch ubuntu:24.04 u1
```

If you configured local storage during MicroCloud initialization, try launching another Ubuntu container using local storage:

```bash
lxc launch ubuntu:24.04 u2 --storage local
```

Notice that it takes much less time to launch the second container, because the Ubuntu image is temporarily cached from when you launched the first.

Next, launch an Ubuntu VM:

```bash
lxc launch ubuntu:24.04 u3 --vm
```

Expect the VM image to take longer to launch than the second container. This is because container and VM images are different, and the VM image is not yet cached.

Check the list of instances:

```bash
lxc list
```

You should see output similar to this:

```{terminal}
:user: ubuntu
:host: micro1
:input: lxc list
:scroll:

+------+---------+---------------------+-------------------------------------------------+-----------------+-----------+----------+
| NAME |  STATE  |        IPV4         |                      IPV6                       |      TYPE       | SNAPSHOTS | LOCATION |
+------+---------+---------------------+-------------------------------------------------+-----------------+-----------+----------+
| u1   | RUNNING | 203.0.113.2 (eth0)   | 2001:0db8:0113:0000:0000:0000:0000:0002 (eth0) | CONTAINER       | 0         | micro1   |
+------+---------+---------------------+-------------------------------------------------+-----------------+-----------+----------+
| u2   | RUNNING | 203.0.113.3 (eth0)   | 2001:0db8:0113:0000:0000:0000:0000:0003 (eth0) | CONTAINER       | 0         | micro1   |
+------+---------+---------------------+-------------------------------------------------+-----------------+-----------+----------+
| u3   | RUNNING | 203.0.113.4 (enp5s0) | 2001:0db8:0113:0000:0000:0000:0000:0004 (eth0) | VIRTUAL-MACHINE | 0         | micro1   |
+------+---------+---------------------+-------------------------------------------------+-----------------+-----------+----------+
```

Run the following commands to list your storage volumes:

```bash
lxc storage volume list remote
lxc storage volume list local
```

Expect output similar to this:

```{terminal}
:user: ubuntu
:host: micro1
:input: lxc storage volume list remote
:scroll:
+-----------------+------------------------------------------------------------------+-------------+--------------+---------+----------+
|      TYPE       |                               NAME                               | DESCRIPTION | CONTENT-TYPE | USED BY | LOCATION |
+-----------------+------------------------------------------------------------------+-------------+--------------+---------+----------+
| container       | u1                                                               |             | filesystem   | 1       |          |
+-----------------+------------------------------------------------------------------+-------------+--------------+---------+----------+
| image           | 729e8a878cb625654b67b7068cc754327411a9bdfffda253ceaf2a34fad4cde2 |             | block        | 1       |          |
+-----------------+------------------------------------------------------------------+-------------+--------------+---------+----------+
| image           | 349269fb8e20f8e13c99833ef673e895b13834a98b6d3ba996fc8bab8e23e1dd |             | filesystem   | 1       |          |
+-----------------+------------------------------------------------------------------+-------------+--------------+---------+----------+
| virtual-machine | u3                                                               |             | block        | 1       |          |
+-----------------+------------------------------------------------------------------+-------------+--------------+---------+----------+
:input: lxc storage volume list local
+-----------+------------------------------------------------------------------+-------------+--------------+---------+----------+
|   TYPE    |                               NAME                               | DESCRIPTION | CONTENT-TYPE | USED BY | LOCATION |
+-----------+------------------------------------------------------------------+-------------+--------------+---------+----------+
| container | u2                                                               |             | filesystem   | 1       | micro1   |
+-----------+------------------------------------------------------------------+-------------+--------------+---------+----------+
| custom    | backups                                                          |             | filesystem   | 1       | micro1   |
+-----------+------------------------------------------------------------------+-------------+--------------+---------+----------+
| custom    | images                                                           |             | filesystem   | 1       | micro1   |
+-----------+------------------------------------------------------------------+-------------+--------------+---------+----------+
| image     | 349269fb8e20f8e13c99833ef673e895b13834a98b6d3ba996fc8bab8e23e1dd |             | filesystem   | 1       | micro1   |
+-----------+------------------------------------------------------------------+-------------+--------------+---------+----------+
```

Verify for yourself that the volumes are located in the storage pools you specified. The volumes belonging to the `u1` container and the `u3` VM should appear in the list of remote storage volumes, whereas the volumes belonging to the `u2` container (if you launched it) should appear in the list of local volumes.

(tutorial-single-instances-networking)=
## Inspect your networking

We will begin by testing the connectivity between instances on your cluster member. After that, we will create another network to demonstrate how we can isolate instances.

(tutorial-single-instances-connectivity)=
### Test connectivity

We'll shell into one of the instances, then ping other instances from it to test the connectivity.

You'll need the internal addresses of the instances, shown in the output from this command:

```bash
lxc list
```

Use the following command to access the shell in the `u1` instance:

```bash
lxc shell u1
```

From within that shell, ping the IPv4 address of your `u2` instance. If you did not configure local storage and thus do not have a `u2` instance, ping the IPv4 address of your `u3` instance instead.

```{terminal}
:user: root
:host: u1
:input: ping -c 4 <IPv4 address of your u2 or u3 instance>
```

Next, ping the IPv6 address of your `u3` instance:

```{terminal}
:user: root
:host: u1
:input: ping -c 4 <IPv6 address of your u3 instance>
```

Check if the `u1` instance also has connectivity to the outside world by pinging an external IP. The example below pings the public Google DNS server IP address:

```bash
ping -c 4 8.8.8.8
```

If you have enabled DNS, you can also test that it works by pinging a domain name. The example below pings the Google DNS server's domain name:

```bash
ping -c 4 dns.google
```

Finally, use the following command to log out of the `u1` shell, returning to the MicroCloud host machine:

```bash
exit
```

(tutorial-single-isolate)=
### Create another network for instance isolation

The instances that you have launched are all on the same subnet. You can, however, create a different network to isolate some instances from others.

Create an OVN network with the default settings:

```bash
lxc network create isolated --type=ovn
```

Take a look at the details for this new network:

```bash
lxc network show isolated
```

There is only one `UPLINK` network, so the new network uses this one as well. Notice that the `volatile.network.ipv4.address` for this network is different from the `default` OVN network's, but within the same subnet. Also compare the `ipv4.address` in the `config` options for each, and note that they are on different subnets.

Check that you can ping the `volatile.network.ipv4.address` for the new `isolated` network:

```{terminal}
:user: ubuntu
:host: micro1
:input: ping -c 4 <volatile.network.ipv4.address of 'isolated'>
```

Launch a new Ubuntu container that uses the new network:

```bash
lxc launch ubuntu:24.04 u4 --network isolated
```

Access the shell in the `u4` container:

```bash
lxc shell u4
```

Confirm that the instance has connectivity to the outside world by pinging the Google DNS server's IP address, and if you have DNS enabled, its domain name as well:

```bash
ping -c 4 8.8.8.8
ping -c dns.google
```

Next, try to ping the IPv4 address of one of the other instances, such as `u1`:

```{terminal}
:user: ubuntu
:host: micro1
:input: ping -c 4 <IPv4 address of another instance>
```

This ping should fail. Other instances should not be reachable because they are on a different OVN subnet.

```{admonition} OVN peer routing
:class: tip
If you want to enable direct connectivity for instances on different OVN subnets, see: {ref}`lxd:network-ovn-peers`.
```

Exit the `u4` container:

```bash
exit
```

(tutorial-single-ui)=
## Access the UI

Instead of managing your instances and your LXD setup from the command line, you can also use the LXD UI. See {ref}`lxd:access-ui` for more information.

Check the LXD cluster list to determine the URL of the cluster member.

```bash
lxc cluster list
```

Example:

```{terminal}
:user: ubuntu
:host: micro1
:input: lxc cluster list
:scroll:

+--------+--------------------------+-----------------+--------------+----------------+-------------+--------+-------------------+
|  NAME  |           URL            |      ROLES      | ARCHITECTURE | FAILURE DOMAIN | DESCRIPTION | STATE  |      MESSAGE      |
+--------+--------------------------+-----------------+--------------+----------------+-------------+--------+-------------------+
| micro1 | https://192.0.2.10:8443  | database-leader | x86_64       | default        |             | ONLINE | Fully operational |
|        |                          | database        |              |                |             |        |                   |
+--------+--------------------------+-----------------+--------------+----------------+-------------+--------+-------------------+

```

Open the URL of your cluster member in your browser.

By default, MicroCloud uses a self-signed certificate, which will cause a security warning to appear. Use your browser’s mechanism to continue despite the security warning.

```{figure} /images/ui_security_warning.png
:alt: Example for a security warning in Chrome
```
You should now see the LXD UI prompting you to set up a certificate. Follow the instructions in the UI to set up the certificates. When done, you'll be able to browse the UI and inspect the networks, storage, and instances that were created during this tutorial.

```{figure} /images/ui_instances.png
:alt: Instances view in the LXD UI
```

(tutorial-single-next)=
## Next steps

To learn how to add more physical machines as cluster members to your MicroCloud, see: {ref}`howto-member-add`.

See {ref}`howto-commands` for a reference of the most common commands.

If you're new to LXD, check out the {ref}`LXD tutorials <lxd:first-steps>` to familiarize yourself with what you can do in LXD. You can skip the sections for installing and initializing LXD, because LXD is already operational as part of your MicroCloud setup.
