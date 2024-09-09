(howto-initialise)=
# How to initialise MicroCloud

The {ref}`initialisation process <explanation-initialisation>` bootstraps the MicroCloud cluster.
You run the initialisation on one of the machines, and it configures the required services on all of the machines that have been joined.

(howto-initialise-interactive)=
## Interactive configuration

If you run the initialisation process in interactive mode (the default), you are prompted for information about your machines and how you want to set them up.
The questions that you are asked might differ depending on your setup; for example, if you do not have the MicroOVN snap installed, you will not be prompted to configure your network, and if your machines don't have local disks, you will not be prompted to set up local storage.

The following instructions show the full initialisation process.

```{tip}
During initialisation, MicroCloud displays tables of entities to choose from.

To select specific entities, use the {kbd}`Up` and {kbd}`Down` keys to choose a table row and select it with the {kbd}`Space` key.
To select all rows, use the {kbd}`Right` key.
You can filter the table rows by typing one or more characters.

When you have selected the required entities, hit {kbd}`Enter` to confirm.
```

Complete the following steps to initialise MicroCloud:

1. On one of the machines, enter the following command:

       sudo microcloud init

1. Select whether you want to set up more than one machine.

   This allows you to create a MicroCloud using a single node.
   It will skip the {ref}`trust-establishment-session` if no more machines should be part of the MicroCloud.

   Additional machines can always be added at a later point in time.
   See {ref}`howto-add` for more information.
1. Select the IP address that you want to use for MicroCloud's internal traffic (see {ref}`microcloud-networking-intracluster`).
   MicroCloud automatically detects the available addresses (IPv4 and IPv6) on the existing network interfaces and displays them in a table.

   You must select exactly one address.
1. On all the other machines, enter the following command and repeat the address selection:

       sudo microcloud join

   It will automatically detect the machine acting as the initiator.
   See {ref}`trust-establishment-session` for more information and  {ref}`automatic-server-detection` in case the network doesn't support mDNS.
1. Select the machines that you want to add to the MicroCloud cluster.

   MicroCloud displays all machines that have reached out during the trust establishment session.
   Make sure that all machines that you select have the required snaps installed.
1. Select whether you want to set up local storage.

   ```{note}
   To set up local storage, each machine must have a local disk.
   The disks must not contain any partitions.
   ```

   If you choose `yes`, configure the local storage:

   1. Select the disks that you want to use for local storage.

      You must select exactly one disk from each machine.
   1. Select whether you want to wipe any of the disks.
      Wiping a disk will destroy all data on it.
1. Select whether you want to set up distributed storage (using MicroCeph).

   ```{note}
   To set up distributed storage, you need at least three additional disks on at least three different machines.
   The disks must not contain any partitions.
   ```

   If you choose `yes`, configure the distributed storage:

   1. Select the disks that you want to use for distributed storage.

      You must select at least three disks.
   1. Select whether you want to wipe any of the disks.
      Wiping a disk will destroy all data on it.

   1. Select whether you want to encrypt any of the disks.
      Encrypting a disk will store the encryption keys in the Ceph key ring inside the Ceph configuration folder.

      ```{note}
      Encryption requires a kernel with `dm_crypt` enabled.
      See {doc}`how full disk encryption works <microceph:explanation/full-disk-encryption>` in the MicroCeph documentation for more information.
      ```

   1. You can choose to optionally set up a CephFS distributed file system.
1. Select either an IPv4 or IPv6 CIDR subnet for the Ceph internal traffic. You can leave it empty to use the default value, which is the MicroCloud internal network (see {ref}`howto-ceph-networking` for how to configure it).
1. Select whether you want to set up distributed networking (using MicroOVN).

   If you choose `yes`, configure the distributed networking:

   1. Select the network interfaces that you want to use (see {ref}`microcloud-networking-uplink`).

      You must select one network interface per machine.
   1. If you want to use IPv4, specify the IPv4 gateway on the uplink network (in CIDR notation) and the first and last IPv4 address in the range that you want to use with LXD.
   1. If you want to use IPv6, specify the IPv6 gateway on the uplink network (in CIDR notation).
   1. If you chose to set up distributed networking, you can choose to setup an underlay network for the distributed networking:

      If you choose ``yes``, configure the underlay network:

      1. Select the network interfaces that you want to use (see {ref}`microcloud-networking-underlay`).

         You must select one network interface with an IP address per machine.
1. MicroCloud now starts to bootstrap the cluster.
   Monitor the output to see whether all steps complete successfully.
   See {ref}`bootstrapping-process` for more information.

   Once the initialisation process is complete, you can start using MicroCloud.

See an example of the full initialisation process in the {ref}`Get started with MicroCloud <initialisation-process>` tutorial.

### Excluding MicroCeph or MicroOVN from MicroCloud

If the MicroOVN or MicroCeph snap is not installed on the system that runs {command}`microcloud init`, you will be prompted with the following question:

    MicroCeph not found. Continue anyway? (yes/no) [default=yes]:

    MicroOVN not found. Continue anyway? (yes/no) [default=yes]:

If you choose `yes`,  only existing services will be configured on all systems.
If you choose `no`, the setup will be cancelled.

All other systems must have at least the same set of snaps installed as the system that runs {command}`microcloud init`, otherwise they will not be available to select from the list of systems.
Any questions associated to these systems will be skipped. For example, if MicroCeph is not installed, you will not be prompted for distributed storage configuration.

### Reusing an existing MicroCeph or MicroOVN with MicroCloud

If some of the systems are already part of a MicroCeph or MicroOVN cluster, you can choose to reuse this cluster when initialising MicroCloud when prompted with the following question:

    "micro01" is already part of a MicroCeph cluster. Do you want to add this cluster to MicroCloud? (add/skip) [default=add]:

    "micro01" is already part of a MicroOVN cluster. Do you want to add this cluster to MicroCloud? (add/skip) [default=add]:

If you choose `add`, MicroCloud will add the remaining systems selected for initialisation to the pre-existing cluster.
If you choose `skip`, the respective service will not be set up at all.

If more than one MicroCeph or MicroOVN cluster exists among the systems, the MicroCloud initialisation will be cancelled.

(howto-initialise-preseed)=
## Non-interactive configuration

If you want to automate the initialisation process, you can provide a preseed configuration in YAML format to the {command}`microcloud preseed` command:

    cat <preseed_file> | microcloud preseed

Make sure to distribute and run the same preseed configuration on all systems that should be part of the MicroCloud.

The preseed YAML file must use the following syntax:

```{literalinclude} preseed.yaml
:language: YAML
:emphasize-lines: 1-3,6-8,11-12,19,29-33
```
