:discourse: lxc:15871
:relatedlinks: https://snapcraft.io/microcloud

.. _home:

MicroCloud
==========

MicroCloud is the easiest way to get a fully highly available LXD cluster up and running.

sdfdsfsd master The MicroCloud snap automatically configures `LXD`_, `Ceph`_, and `OVN`_ across a set of servers.
MicroCloud relies on mDNS to automatically detect other servers on the network, making it possible to set up a complete cluster by running a single command on one of the machines.

This way, MicroCloud creates a small footprint cluster of compute nodes with distributed storage and secure networking, optimised for repeatable, reliable remote deployments.

MicroCloud is aimed at edge computing, and anyone in need of a small-scale private cloud.

---------

In this documentation
---------------------

..  grid:: 1 1 2 2

   ..  grid-item:: :doc:`Tutorial <tutorial/index>`

       **Start here**: a hands-on introduction to MicroCloud for new users

   ..  grid-item:: :doc:`How-to guides <how-to/index>`

      **Step-by-step guides** covering key operations and common tasks

.. grid:: 1 1 2 2
   :reverse:

   .. grid-item:: :doc:`Reference <reference/index>`

      **Technical information** - specifications, APIs, architecture

   .. grid-item:: :doc:`Explanation <explanation/index>`

      **Discussion and clarification** of key topics

---------

Project and community
---------------------

MicroCloud is a member of the Ubuntu family. It’s an open source project that warmly welcomes community projects, contributions, suggestions, fixes and constructive feedback.

- `MicroCloud snap`_
- `Contribute <GitHub_>`_
- `Get support`_
- `Announcement`_
- `Thinking about using MicroCloud for your next project? Get in touch! <website_>`_


.. toctree::
   :hidden:
   :maxdepth: 2

   tutorial/index
   how-to/index
   reference/index
   explanation/index
