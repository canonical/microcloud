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

MicroOVN requires an uplink network that is an actual L2 subnet (which is usually not the case in a virtual cloud environment).
In addition, MicroOVN requires its own dedicated network interface, for example, a dedicated physical network interface, a VLAN, or a virtual function on an :abbr:`SR-IOV (Single root I/O virtualisation)`-capable network interface.

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
