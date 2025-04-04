(howto-add-service)=
# How to add a service

If you set up the MicroCloud without MicroOVN or MicroCeph initially, you can add those services with the {command}`microcloud service add` command:

    sudo microcloud service add

If MicroCloud detects a service is installed but not set up, it will ask to configure the service.

To add MicroCeph:

   ```{note}
   To set up distributed storage, you need at least three additional disks on at least three different machines.
   The disks must not contain any partitions.
   ```

1. Select `yes` to set up distributed storage.

   1. Select the disks that you want to use for distributed storage.
      You must select at least three disks.

   1. Select whether you want to wipe any of the disks.
      Wiping a disk will destroy all data on it.

   1. You can choose to optionally encrypt the chosen disks.

   1. You can choose to optionally set up a CephFS distributed file system.

   1. Select either an IPv4 or IPv6 CIDR subnet for the Ceph internal traffic. You can leave it empty to use the default value, which is the MicroCloud internal network (see {ref}`howto-ceph-networking` for how to configure it).

To add MicroOVN:

1. Select `yes` to set up distributed networking.

   1. Select the network interfaces that you want to use (see {ref}`reference-requirements-network-interfaces-uplink`).
      You must select one network interface per machine.

   1. If you want to use IPv4, specify the IPv4 gateway on the uplink network (in CIDR notation) and the first and last IPv4 address in the range that you want to use with LXD.

   1. If you want to use IPv6, specify the IPv6 gateway on the uplink network (in CIDR notation).

MicroCloud now starts to bootstrap the cluster for only the new services.

Monitor the output to see whether all steps complete successfully.

See {ref}`bootstrapping-process` for more information.
