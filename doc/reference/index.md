.. _reference:

Reference
=========

The reference material in this section provides technical descriptions of MicroCloud.

Requirements
------------

.. image:: https://res.cloudinary.com/canonical/image/fetch/f_auto,q_auto,fl_sanitize,w_236,h_214/https://assets.ubuntu.com/v1/904e5156-LXD+illustration+2.svg
   :alt: LXD requirements illustration
   :align: center

.. _hardware-requirements:

Hardware requirements
~~~~~~~~~~~~~~~~~~~~~

MicroCloud requires a minimum of three machines.
It supports up to 50 machines.

Each machine must have at least 8 GiB of RAM (more depending on the connected disks).
You can mix different processor architectures within the same MicroCloud cluster.

If you want to add further machines after the initial initialisation, you can use the :command:`microcloud add` command.

To use local storage, each machine requires a local disk.
To use distributed storage, at least three additional disks (not only partitions) for use by Ceph are required, and these disks must be on at least three different machines.

Also see Ceph's :ref:`ceph:hardware-recommendations`.

Networking requirements
~~~~~~~~~~~~~~~~~~~~~~~

For networking, MicroCloud requires two dedicated network interfaces: one for intra-cluster communication and one for external connectivity.
To allow for external connectivity, MicroCloud requires an uplink network that supports broadcast and multicast.
See :ref:`explanation-networking` for more information.

The IP addresses of the machines must not change after installation, so DHCP is not supported.

Software requirements
~~~~~~~~~~~~~~~~~~~~~

MicroCloud requires snapd version 2.59 or newer.

Also see LXD's :ref:`lxd:requirements` and Ceph's :doc:`ceph:start/os-recommendations`.

.. _snaps:

Snaps
-----

To run MicroCloud, you must install the following snaps:

- `MicroCloud snap`_
- `LXD snap`_
- `MicroCeph snap`_
- `MicroOVN snap`_
