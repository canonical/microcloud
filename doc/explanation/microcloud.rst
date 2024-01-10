:relatedlinks: https://documentation.ubuntu.com/lxd/

.. _explanation-microcloud:

About MicroCloud
================

.. include:: ../../README.md
   :parser: myst_parser.sphinx_
   :start-after: be ready within minutes.
   :end-before: ## **What about networking?**

LXD cluster
-----------

MicroCloud sets up a LXD cluster.
You can use the :command:`microcloud cluster` command to show information about the cluster members, or to remove specific members.

Apart from that, you can use LXD commands to manage the cluster.
See :ref:`lxd:clustering` in the LXD documentation for more information.

.. _explanation-networking:

Networking
----------

By default, MicroCloud uses MicroOVN for networking, which is a minimal wrapper around OVN (Open Virtual Network).

For external connectivity, MicroOVN requires an uplink network.
This uplink network must support broadcast and multicast traffic (so that IP adverts and packets can flow between the OVN virtual router and the uplink network).
Proper Ethernet networks generally fulfil these requirements, but virtual cloud environments often don't.

Each machine in the MicroCloud cluster should have at least two available network interfaces (which can be connected to the same network or to different networks):

.. _microcloud-networking-intracluster:

Network interface for intra-cluster traffic
  MicroCloud requires one network interface that is pre-configured with an IP address that is within the same subnet as the IPs of the other cluster members.
  The network that it is connected to must support multicast.

  This network interface can be, for example, a dedicated physical network interface, a VLAN, or a virtual function on an :abbr:`SR-IOV (Single root I/O virtualisation)`-capable network interface.
  It serves as the dedicated network interface for MicroOVN and is used for multicast discovery (during setup) and all internal traffic between the MicroCloud, OVN, and Ceph members.

.. _microcloud-networking-uplink:

Network interface to connect to the uplink network
  MicroCloud requires one network interface for connecting OVN to the uplink network.
  This network interface must either be an unused interface that does not have an IP address configured, or a bridge.

  MicroCloud configures this interface as an uplink interface that provides external connectivity to the MicroCloud cluster.

  You can specify a different interface to be used as the uplink interface for each cluster member.
  MicroCloud requires that all uplink interfaces are connected to the uplink network, using the gateway and IP address range information that you provide during the MicroCloud initialisation process.

If you have a network interface that is configured as a Linux bridge, you can use it for both network interfaces.

See :ref:`lxd:network-ovn` in the LXD documentation for more information.

If you decide to not use MicroOVN, MicroCloud falls back on the Ubuntu fan for basic networking.
MicroCloud will still be usable, but you will see some limitations:

- When you move an instance from one cluster member to another, its IP address changes.
- Egress traffic leaves from the local cluster member (while OVN provides shared egress).
  As a result of this, network forwarding works at a basic level only, and external addresses must be forwarded to a specific cluster member and don't fail over.
- There is no support for hardware acceleration, load balancers, or ACL functionality within the local network.

Storage
-------

You have two options for storage in MicroCloud: local storage or distributed storage.

Local storage is faster, but less flexible and not fail-safe.
To use local storage, each machine in the cluster requires a local disk.
Disk sizes can vary.

For distributed storage, MicroCloud uses MicroCeph, which is a lightweight way of deploying a Ceph cluster.
To use distributed storage, you must have at least three disks (attached to at least three different machines).

Troubleshooting
---------------

MicroCloud does not manage the services that it deploys.
After the deployment process, the individual services are operating independently.
If anything goes wrong, each service is responsible for handling recovery.

So, for example, if :command:`lxc cluster list` shows that a LXD cluster member is offline, follow the usual steps for recovering an offline cluster member (in the simplest case, restart the LXD snap on the machine).
The same applies to MicroOVN and MicroCeph.
