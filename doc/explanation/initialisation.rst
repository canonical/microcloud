.. _explanation-initialisation:

About the initialisation process
================================

See :ref:`howto-initialise` for instructions on how to set up MicroCloud.

.. _automatic-server-detection:

Automatic server detection
--------------------------

MicroCloud uses :abbr:`mDNS (multicast DNS)` to automatically detect other servers on the network.
This method works in physical networks, but it is usually not supported in a cloud environment.

By default, the scan is limited to the local subnet.

MicroCloud will display all servers that it detects, and you can then select the ones you want to add to the MicroCloud cluster.

.. _bootstrapping-process:

Bootstrapping process
---------------------

After you provide the required information to :ref:`initialise MicroCloud <howto-initialise>`, MicroCloud starts bootstrapping the cluster.

The bootstrapping process consists of the following steps:

1. MicroCloud initialises the first server (the one where you run the :command:`microcloud init` command) and creates the MicroCloud cluster.
#. MicroCloud creates the LXD cluster, the OVN cluster, and the Ceph cluster.
#. MicroCloud issues join tokens for the other servers that are to be added to the cluster.
#. MicroCloud sends the join tokens over the network to the other servers.
#. The other servers join the MicroCloud cluster, the OVN cluster, and the Ceph cluster.

   This step of forming the cluster can take several minutes, mainly because of the initialisation of MicroCeph and adding disks to the Ceph cluster.
#. When the cluster is formed, MicroCloud configures LXD.
   It sets up networking and storage pools and configures the default profile to use the created OVN network and the distributed storage (if available).

After the initialisation is complete, you can look at the LXD configuration to confirm the setup.

.. terminal::
   :input: lxc cluster list
   :host: micro01
   :scroll:

   +---------+------------------------------------+-----------------+--------------+----------------+-------------+--------+-------------------+
   |  NAME   |                 URL                |      ROLES      | ARCHITECTURE | FAILURE DOMAIN | DESCRIPTION | STATE  |      MESSAGE      |
   +---------+------------------------------------+-----------------+--------------+----------------+-------------+--------+-------------------+
   | micro01 | https://[2001:db8:d:100::169]:8443 | database-leader | aarch64      | default        |             | ONLINE | Fully operational |
   |         |                                    | database        |              |                |             |        |                   |
   +---------+------------------------------------+-----------------+--------------+----------------+-------------+--------+-------------------+
   | micro02 | https://[2001:db8:d:100::170]:8443 | database        | aarch64      | default        |             | ONLINE | Fully operational |
   +---------+------------------------------------+-----------------+--------------+----------------+-------------+--------+-------------------+
   | micro03 | https://[2001:db8:d:100::171]:8443 | database        | aarch64      | default        |             | ONLINE | Fully operational |
   +---------+------------------------------------+-----------------+--------------+----------------+-------------+--------+-------------------+
   :input: lxc storage list
   +--------+--------+-----------------------------+---------+---------+
   |  NAME  | DRIVER |         DESCRIPTION         | USED BY |  STATE  |
   +--------+--------+-----------------------------+---------+---------+
   | local  | zfs    | Local storage on ZFS        | 10      | CREATED |
   +--------+--------+-----------------------------+---------+---------+
   | remote | ceph   | Distributed storage on Ceph | 7       | CREATED |
   +--------+--------+-----------------------------+---------+---------+
   :input: lxc network list
   +----------+----------+---------+-----------------+---------------------------+-------------+---------+---------+
   |   NAME   |   TYPE   | MANAGED |      IPV4       |           IPV6            | DESCRIPTION | USED BY |  STATE  |
   +----------+----------+---------+-----------------+---------------------------+-------------+---------+---------+
   | UPLINK   | physical | YES     |                 |                           |             | 2       | CREATED |
   +----------+----------+---------+-----------------+---------------------------+-------------+---------+---------+
   | default  | ovn      | YES     | 10.123.123.1/24 | fd42:1234:1234:1234::1/64 |             | 5       | CREATED |
   +----------+----------+---------+-----------------+---------------------------+-------------+---------+---------+
   | eth0     | physical | NO      |                 |                           |             | 0       |         |
   +----------+----------+---------+-----------------+---------------------------+-------------+---------+---------+
   | eth0.200 | vlan     | NO      |                 |                           |             | 1       |         |
   +----------+----------+---------+-----------------+---------------------------+-------------+---------+---------+
