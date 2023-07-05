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

.. include:: ../../README.md
   :parser: myst_parser.sphinx_
   :start-after: ## **What about networking?**
   :end-before: ## **What's next?**

MicroOVN requires an uplink network that is an actual L2 subnet (which is usually not the case in a virtual cloud environment).
In addition, MicroOVN requires its own dedicated network interface, for example, a dedicated physical network interface, a VLAN, or a virtual function on an :abbr:`SR-IOV (Single root I/O virtualisation)`-capable network interface.

See :ref:`lxd:network-ovn` in the LXD documentation for more information.

Storage
-------

You have two options for storage in MicroCloud: local storage or distributed storage.

Local storage is faster, but less flexible and not fail-safe.
To use local storage, each machine in the cluster requires a local disk.
Disk sizes can vary.

For distributed storage, MicroCloud uses MicroCeph, which is a lightweight way of deploying a Ceph cluster.
To use distributed storage, you must have at least three disks (attached to at least three different machines).
