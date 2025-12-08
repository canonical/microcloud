(howto-initialize)=
# How to initialize MicroCloud

The {ref}`initialization process <explanation-initialization>` bootstraps the MicroCloud cluster.
You run the initialization on one of the machines, and it configures the required services on all of the machines that have been joined.

## Pre-initialization requirements

- Complete the steps in {ref}`howto-install` before initialization.
- If you intend to use full disk encryption (FDE) on any cluster member, that member must meet the prerequisites listed on this page: {doc}`microceph:explanation/security/full-disk-encryption`.
  - Follow only the instructions in the Prerequisites section on that page. Skip its Usage section; the MicroCloud initialization process handles the disk encryption.

(howto-initialize-interactive)=
## Interactive configuration

If you run the initialization process in interactive mode (the default), you are prompted for information about your machines and how you want to set them up.
The questions that you are asked might differ depending on your setup. For example, if you do not have the MicroOVN snap installed, you will not be prompted to configure your network; if your machines don't have local disks, you will not be prompted to set up storage.

The following instructions show the full initialization process.

```{tip}
During initialization, MicroCloud displays tables of entities to choose from.

To select specific entities, use the {kbd}`Up` and {kbd}`Down` keys to choose a table row and select it with the {kbd}`Space` key.
To select all rows, use the {kbd}`Right` key.
You can filter the table rows by typing one or more characters.

When you have selected the required entities, hit {kbd}`Enter` to confirm.
```

Complete the following steps to initialize MicroCloud:

1. On one of the machines, enter the following command:

       sudo microcloud init

1. Select whether you want to set up more than one machine.

   This allows you to create a MicroCloud using a single cluster member.
   It will skip the {ref}`trust-establishment-session` if no more machines should be part of the MicroCloud.

   Additional machines can always be added at a later point in time.
   See {ref}`howto-member-add` for more information.
1. Select the IP address that you want to use for MicroCloud's internal traffic (see {ref}`reference-requirements-network-interfaces-intracluster`).
   MicroCloud automatically detects the available addresses (IPv4 and IPv6) on the existing network interfaces and displays them in a table.

   You must select exactly one address.
1. On all the other machines, enter the following command and repeat the address selection:

       sudo microcloud join

   It will automatically detect the machine acting as the initiator.
   See {ref}`trust-establishment-session` for more information and  {ref}`automatic-server-detection` in case the network doesn't support multicast.
1. Select the machines that you want to add to the MicroCloud cluster.

   MicroCloud displays all machines that have reached out during the trust establishment session.
   Make sure that all machines that you select have the required snaps installed.
1. Select whether you want to set up local storage.

   ```{note}
   - To set up local storage, each machine must have a local disk.
   - The disks must not contain any partitions.
   - A disk used for local storage will not be available for distributed storage.
   ```

   If you choose `yes`, configure the local storage:

   1. Select the disks that you want to use for local storage.

      You must select exactly one disk from each machine.
   1. Select whether you want to wipe any of the disks.
      Wiping a disk will destroy all data on it.
1. Select whether you want to set up distributed storage (using MicroCeph).

   ```{note}
   - You can set up distributed storage on a single cluster member.
   - High availability requires a minimum of 3 cluster members, with 3 separate disks across 3 different cluster members.
   - The disks must not contain any partitions.
   - A disk that was previously selected for local storage will not be shown for distributed storage.
   ```

   If you choose `yes`, configure the distributed storage:

   1. Select the disks that you want to use for distributed storage.

      You must select at least one disk.
   1. Select whether you want to wipe any of the disks.
      Wiping a disk will destroy all data on it.

   1. Select whether you want to encrypt any of the disks.
      Encrypting a disk will store the encryption keys in the Ceph key ring inside the Ceph configuration folder.

      ```{warning}
      Cluster members with disks to be encrypted require a kernel with `dm-crypt` enabled. The snap `dm-crypt` plug must also be connected. See the Prerequisites section of this page for more information: {doc}`microceph:explanation/security/full-disk-encryption`.

      If you have not enabled and connected `dm-crypt` on any cluster member that you want to encrypt, do so now before you continue.

      ```

   1. You can choose to optionally set up a CephFS distributed file system.
1. Select either an IPv4 or IPv6 CIDR subnet for the Ceph internal traffic. You can leave it empty to use the default value, which is the MicroCloud internal network (see {ref}`howto-ceph-networking` for how to configure it).
1. Select either an IPv4 or IPv6 CIDR subnet for the Ceph public traffic. You can leave it empty to use the default value, which is the MicroCloud internal network if you chose this as default for the Ceph internal network question, or the Ceph internal network if you chose to set a custom network other than the MicroCloud internal network (see {ref}`howto-ceph-networking` for how to configure it).
1. Select whether you want to set up distributed networking (using MicroOVN).

   If you choose `yes`, configure the distributed networking:

   1. Select the network interfaces that you want to use (see {ref}`reference-requirements-network-interfaces-uplink`).

      You must select one network interface per machine.
   1. If you want to use IPv4, specify the IPv4 gateway on the uplink network (in CIDR notation) and the first and last IPv4 address in the range that you want to use with LXD.
   1. If you want to use IPv6, specify the IPv6 gateway on the uplink network (in CIDR notation).
   1. If you chose to set up distributed networking, you can optionally set up an underlay network for the distributed networking (for an explanation of the benefits, see {ref}`exp-networking-ovn-underlay`):

      If you choose ``yes``, configure the underlay network:

      1. Select the network interfaces that you want to use.

         You must select one network interface with an IP address per machine.
1. MicroCloud now starts to bootstrap the cluster.
   Monitor the output to see whether all steps complete successfully.
   See {ref}`bootstrapping-process` for more information.

   Once the initialization process is complete, you can start using MicroCloud.

See an example of the full initialization process in the {ref}`Get started with MicroCloud <initialization-process>` tutorial.

Once you have initialized MicroCloud, you can interact with it through CLI commands, API requests, or its {ref}`graphical UI <howto-ui>`.

### Excluding MicroCeph or MicroOVN from MicroCloud

If the MicroOVN or MicroCeph snap is not installed on the system that runs {command}`microcloud init`, you will be prompted with the following question:

    MicroCeph not found. Continue anyway? (yes/no) [default=yes]:

    MicroOVN not found. Continue anyway? (yes/no) [default=yes]:

If you choose `yes`,  only existing services will be configured on all systems.
If you choose `no`, the setup will be canceled.

All other systems must have at least the same set of snaps installed as the system that runs {command}`microcloud init`, otherwise they will not be available to select from the list of systems.
Any questions associated to these systems will be skipped. For example, if MicroCeph is not installed, you will not be prompted for distributed storage configuration.

### Reusing an existing MicroCeph or MicroOVN with MicroCloud

If some of the systems are already part of a MicroCeph or MicroOVN cluster, you can choose to reuse this cluster when initializing MicroCloud when prompted with the following question:

    "micro01" is already part of a MicroCeph cluster. Do you want to add this cluster to MicroCloud? (add/skip) [default=add]:

    "micro01" is already part of a MicroOVN cluster. Do you want to add this cluster to MicroCloud? (add/skip) [default=add]:

If you choose `add`, MicroCloud will add the remaining systems selected for initialization to the pre-existing cluster.
If you choose `skip`, the respective service will not be set up at all.

If more than one MicroCeph or MicroOVN cluster exists among the systems, the MicroCloud initialization will be canceled.

(howto-initialize-preseed)=
## Non-interactive configuration

If you want to automate the initialization process, you can provide a preseed configuration in YAML format to the {command}`microcloud preseed` command:

    cat <preseed_file> | microcloud preseed

Make sure to distribute and run the same preseed configuration on all systems that should be part of the MicroCloud.

The preseed YAML file must use the following syntax:

```{literalinclude} preseed.yaml
:language: YAML
:emphasize-lines: 1-4,7-10,13-14,17-19,22,25-27,30-35,63-66,72,79-87
```

### Minimal preseed using multicast discovery

You can use the following minimal preseed file to initialize a MicroCloud across three machines.
In this case `micro01` takes over the role of the initiator.
Multicast discovery is used to find the other machines on the network.

On each of the machines `eth1` is used as uplink for the OVN network.
The uplink network `192.0.2.0/24` is configured with `192.0.2.1` as the gateway.
The range `192.0.2.100-192.0.2.254` is used for the assignment of NAT addresses to the OVN overlay networks.

For local storage the disk `/dev/sdb` is occupied.
In case of remote storage `/dev/sdc` will be used by MicroCeph:

```yaml
lookup_subnet: 10.0.0.0/24
initiator: micro01
session_passphrase: foo
systems:
- name: micro01
  ovn_uplink_interface: eth1
  storage:
    local:
      path: /dev/sdb
    ceph:
      - path: /dev/sdc
- name: micro02
  ovn_uplink_interface: eth1
  storage:
    local:
      path: /dev/sdb
    ceph:
      - path: /dev/sdc
- name: micro03
  ovn_uplink_interface: eth1
  storage:
    local:
      path: /dev/sdb
    ceph:
      - path: /dev/sdc
ovn:
  ipv4_gateway: 192.0.2.1/24
  ipv4_range: 192.0.2.100-192.0.2.254
```
