.. _howto-initialise:

How to initialise MicroCloud
============================

The :ref:`initialisation process <explanation-initialisation>` bootstraps the MicroCloud cluster.
You run the initialisation on one of the machines, and it configures the required services on all machines.

During the initialisation process, you are prompted for information about your machines and how you want to set them up.
The questions that you are asked might differ depending on your setup; for example, if you do not have the MicroOVN snap installed, you will not be prompted to configure your network, and if your machines don't have local disks, you will not be prompted to set up local storage.

The following instructions show the full initialisation process.

.. tip::
   During initialisation, MicroCloud displays tables of entities to choose from.

   To select specific entities, use the :kbd:`Up` and :kbd:`Down` keys to choose a table row and select it with the :kbd:`Space` key.
   To select all rows, use the :kbd:`Right` key.
   You can filter the table rows by typing one or more characters.

   When you have selected the required entities, hit :kbd:`Enter` to confirm.

Complete the following steps to initialise MicroCloud:

1. On one of the machines, enter the following command::

     sudo microcloud init

#. Select the IP address that you want to use for MicroCloud's internal traffic.
   MicroCloud automatically detects the available addresses (IPv4 and IPv6) on the existing network interfaces and displays them in a table.

   You must select exactly one address.
#. Decide if you want to limit the search for other machines.

   If you accept the default (``yes``), MicroCloud will automatically detect machines in the local subnet.
   Otherwise, it will detect all available machines, which might include duplicates (if machines are available both on IPv4 and on IPv6).

   See :ref:`automatic-server-detection` for more information.
#. Select the machines that you want to add to the MicroCloud cluster.

   MicroCloud displays all machines that it detects.
   Make sure that all machines that you select have the required snaps installed.
#. Select whether you want to set up local storage.

   .. note::
      To set up local storage, each machine must have a local disk.

   If you choose ``yes``, configure the local storage:

   1. Select the disks that you want to use for local storage.

      You must select exactly one disk from each machine.
   #. Select whether you want to wipe any of the disks.
      Wiping a disk will destroy all data on it.
#. Select whether you want to set up distributed storage (using MicroCeph).

   .. note::
      To set up distributed storage, you need at least three additional disks.

   If you choose ``yes``, configure the distributed storage:

   1. Select the disks that you want to use for distributed storage.

      You must select at least three disks.
   #. Select whether you want to wipe any of the disks.
      Wiping a disk will destroy all data on it.
#. Select whether you want to set up distributed networking (using MicroOVN).

   If you choose ``yes``, configure the distributed networking:

   1. Select the network interfaces that you want to use.

      You must select one network interface per machine.
   #. If you want to use IPv4, specify the IPv4 gateway on the uplink network (in CIDR notation) and the first and last IPv4 address in the range that you want to use with LXD.
   #. If you want to use IPv6, specify the IPv6 gateway on the uplink network (in CIDR notation).
#. MicroCloud now starts to bootstrap the cluster.
   Monitor the output to see whether all steps complete successfully.
   See :ref:`bootstrapping-process` for more information.

   Once the initialisation process is complete, you can start using MicroCloud.

Here's an example of the full initialisation process:

.. terminal::
   :input: sudo microcloud init
   :host: micro01
   :scroll:

   Select an address for MicroCloud's internal traffic:
   Space to select; enter to confirm; type to filter results.
   Up/down to move; right to select all; left to select none.
          +----------------------+-------+
          |       ADDRESS        | IFACE |
          +----------------------+-------+
     [ ]  | 203.0.113.169        | eth0  |
   > [X]  | 2001:db8:d:100::169  | eth0  |
          +----------------------+-------+

    Using address "2001:db8:d:100::169" for MicroCloud

   Limit search for other MicroCloud servers to 2001:db8:d:100::169/64? (yes/no) [default=yes]: yes
   Scanning for eligible servers ...
   Space to select; enter to confirm; type to filter results.
   Up/down to move; right to select all; left to select none.
          +---------+-------+----------------------+
          |  NAME   | IFACE |         ADDR         |
          +---------+-------+----------------------+
   > [x]  | micro03 | eth0  | 2001:db8:d:100::171  |
     [x]  | micro02 | eth0  | 2001:db8:d:100::170  |
          +---------+-------+----------------------+

    Selected "micro03" at "2001:db8:d:100::171"
    Selected "micro02" at "2001:db8:d:100::170"

   Would you like to set up local storage? (yes/no) [default=yes]: yes
   Select exactly one disk from each cluster member:
   Space to select; enter to confirm; type to filter results.
   Up/down to move; right to select all; left to select none.
          +----------+---------------------------+-----------+------+-------------------------------------------+
          | LOCATION |           MODEL           | CAPACITY  | TYPE |                   PATH                    |
          +----------+---------------------------+-----------+------+-------------------------------------------+
     [ ]  | micro01  | Samsung SSD 870 EVO 500GB | 465.76GiB | sata | /dev/disk/by-id/wwn-0x5002538fc2b211bd    |
     [x]  | micro01  | Samsung SSD 980 250GB     | 232.89GiB | nvme | /dev/disk/by-id/nvme-eui.002538dc21405ac8 |
     [ ]  | micro02  | Samsung SSD 870 EVO 500GB | 465.76GiB | sata | /dev/disk/by-id/wwn-0x5002538fc2b2109c    |
     [x]  | micro02  | Samsung SSD 980 250GB     | 232.89GiB | nvme | /dev/disk/by-id/nvme-eui.002538dc21405ad7 |
     [ ]  | micro03  | Samsung SSD 870 EVO 500GB | 465.76GiB | sata | /dev/disk/by-id/wwn-0x5002538fc2b2162d    |
   > [x]  | micro03  | Samsung SSD 980 250GB     | 232.89GiB | nvme | /dev/disk/by-id/nvme-eui.002538dc21405aca |
          +----------+---------------------------+-----------+------+-------------------------------------------+

   Select which disks to wipe:
   Space to select; enter to confirm; type to filter results.
   Up/down to move; right to select all; left to select none.
          +----------+---------------------------+-----------+------+-------------------------------------------+
          | LOCATION |           MODEL           | CAPACITY  | TYPE |                   PATH                    |
          +----------+---------------------------+-----------+------+-------------------------------------------+
   > [x]  | micro01  | Samsung SSD 980 250GB     | 232.89GiB | nvme | /dev/disk/by-id/nvme-eui.002538dc21405ac8 |
     [x]  | micro02  | Samsung SSD 980 250GB     | 232.89GiB | nvme | /dev/disk/by-id/nvme-eui.002538dc21405ad7 |
     [x]  | micro03  | Samsung SSD 980 250GB     | 232.89GiB | nvme | /dev/disk/by-id/nvme-eui.002538dc21405aca |
          +----------+---------------------------+-----------+------+-------------------------------------------+

    Using "/dev/disk/by-id/nvme-eui.002538dc21405ac8" on "micro01" for local storage pool
    Using "/dev/disk/by-id/nvme-eui.002538dc21405ad7" on "micro02" for local storage pool
    Using "/dev/disk/by-id/nvme-eui.002538dc21405aca" on "micro03" for local storage pool

   Would you like to set up distributed storage? (yes/no) [default=yes]: yes
   Select from the available unpartitioned disks:
   Space to select; enter to confirm; type to filter results.
   Up/down to move; right to select all; left to select none.
          +----------+---------------------------+-----------+------+----------------------------------------+
          | LOCATION |           MODEL           | CAPACITY  | TYPE |                  PATH                  |
          +----------+---------------------------+-----------+------+----------------------------------------+
   > [x]  | micro01  | Samsung SSD 870 EVO 500GB | 465.76GiB | sata | /dev/disk/by-id/wwn-0x5002538fc2b211bd |
     [x]  | micro02  | Samsung SSD 870 EVO 500GB | 465.76GiB | sata | /dev/disk/by-id/wwn-0x5002538fc2b2109c |
     [x]  | micro03  | Samsung SSD 870 EVO 500GB | 465.76GiB | sata | /dev/disk/by-id/wwn-0x5002538fc2b2162d |
          +----------+---------------------------+-----------+------+----------------------------------------+

   Select which disks to wipe:
   Space to select; enter to confirm; type to filter results.
   Up/down to move; right to select all; left to select none.
          +----------+---------------------------+-----------+------+----------------------------------------+
          | LOCATION |           MODEL           | CAPACITY  | TYPE |                  PATH                  |
          +----------+---------------------------+-----------+------+----------------------------------------+
   > [x]  | micro01  | Samsung SSD 870 EVO 500GB | 465.76GiB | sata | /dev/disk/by-id/wwn-0x5002538fc2b211bd |
     [x]  | micro02  | Samsung SSD 870 EVO 500GB | 465.76GiB | sata | /dev/disk/by-id/wwn-0x5002538fc2b2109c |
     [x]  | micro03  | Samsung SSD 870 EVO 500GB | 465.76GiB | sata | /dev/disk/by-id/wwn-0x5002538fc2b2162d |
          +----------+---------------------------+-----------+------+----------------------------------------+

    Using 1 disk(s) on "micro02" for remote storage pool
    Using 1 disk(s) on "micro03" for remote storage pool
    Using 1 disk(s) on "micro01" for remote storage pool

   Configure distributed networking? (yes/no) [default=yes]:  yes
   Select exactly one network interface from each cluster member:
   Space to select; enter to confirm; type to filter results.
   Up/down to move; right to select all; left to select none.
          +----------+----------+------+
          | LOCATION |  IFACE   | TYPE |
          +----------+----------+------+
   > [x]  | micro02  | eth0.200 | vlan |
     [x]  | micro03  | eth0.200 | vlan |
     [x]  | micro01  | eth0.200 | vlan |
          +----------+----------+------+

    Using "eth0.200" on "micro03" for OVN uplink
    Using "eth0.200" on "micro01" for OVN uplink
    Using "eth0.200" on "micro02" for OVN uplink

   Specify the IPv4 gateway (CIDR) on the uplink network (empty to skip IPv4): 192.0.2.1/24
   Specify the first IPv4 address in the range to use with LXD: 192.0.2.100
   Specify the last IPv4 address in the range to use with LXD: 192.0.2.254
   Specify the IPv6 gateway (CIDR) on the uplink network (empty to skip IPv6): 2001:db8:d:200::1/64

   Initializing a new cluster
    Local MicroCloud is ready
    Local LXD is ready
    Local MicroOVN is ready
    Local MicroCeph is ready
   Awaiting cluster formation ...
    Peer "micro02" has joined the cluster
    Peer "micro03" has joined the cluster
   Cluster initialization is complete
   MicroCloud is ready
