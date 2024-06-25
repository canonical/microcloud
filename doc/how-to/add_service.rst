.. _howto-add-service:

How to add a new service
========================

If you set up the MicroCloud without MicroOVN or MicroCeph initially, you can add those services with the command :command:`microcloud service add`::

  sudo microcloud service add

If MicroCloud detects a service is installed but not set up, it will ask to configure the service.

#. Select whether you want to set up distributed storage (if adding MicroCeph to the MicroCloud).

   .. note::
      To set up distributed storage, you need at least three additional disks on at least three different machines.
      The disks must not contain any partitions.

   If you choose ``yes``, configure the distributed storage:

   1. Select the disks that you want to use for distributed storage.

      You must select at least three disks.
   #. Select whether you want to wipe any of the disks.
      Wiping a disk will destroy all data on it.

   #. You can choose to optionally set up a CephFS distributed file system.

#. Select either an IPv4 or IPv6 CIDR subnet for the Ceph internal traffic. You can leave it empty to use the default value, which is the MicroCloud internal network (see :ref:`howto-ceph-networking` for how to configure it).

#. Select whether you want to set up distributed networking (if adding MicroOVN to the MicroCloud).

   If you choose ``yes``, configure the distributed networking:

   1. Select the network interfaces that you want to use (see :ref:`microcloud-networking-uplink`).

      You must select one network interface per machine.
   #. If you want to use IPv4, specify the IPv4 gateway on the uplink network (in CIDR notation) and the first and last IPv4 address in the range that you want to use with LXD.
   #. If you want to use IPv6, specify the IPv6 gateway on the uplink network (in CIDR notation).
#. MicroCloud now starts to bootstrap the cluster for only the new services.
   Monitor the output to see whether all steps complete successfully.
   See :ref:`bootstrapping-process` for more information.
