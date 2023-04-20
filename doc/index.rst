.. _home:

.. image:: https://camo.githubusercontent.com/3abbd2b61fcb74805b2b48e41e2fead897322c39dab533de0231e13ac18e968b/68747470733a2f2f6c696e7578636f6e7461696e6572732e6f72672f7374617469632f696d672f636f6e7461696e6572732e706e67
   :alt: LXD logo
   :align: center

MicroCloud
======================================================

MicroCloud is the easiest way to get a fully highly available LXD cluster up and running.

The MicroCloud snap automatically configures `LXD <https://linuxcontainers.org/>`_, `Ceph <https://ceph.io/en/>`_, and `OVN <https://www.ovn.org/>`_ across a set of servers.
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

* `Snap <https://snapcraft.io/microcloud>`_
* `Contribute <https://github.com/canonical/microcloud>`_
* `Get support <https://discuss.linuxcontainers.org/tag/microcloud>`_
* `Announcement <https://discuss.linuxcontainers.org/t/introducing-microcloud/15871>`_
* `Thinking about using MicroCloud for your next project? Get in touch! <https://microcloud.is>`_


.. toctree::
   :hidden:
   :maxdepth: 2

   tutorial/index
   how-to/index
   reference/index
   explanation/index
