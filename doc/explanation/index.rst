Explanation
=============

The explanatory guides in this section introduce you to the concepts used in MicroCloud and help you understand how things fit together.

Configuration
-------------

.. include:: ../../README.md
   :parser: myst_parser.sphinx_
   :start-after: be ready within minutes.
   :end-before: ## **What about networking?**

Automatic server detection
--------------------------

MicroCloud uses :abbr:`mDNS (multicast DNS)` to automatically detect other servers on the network.
This method works in physical networks, but it is usually not supported in a cloud environment.

By default, the scan is limited to the local subnet.

MicroCloud will display all servers that it detects, and you can then select the ones you want to add to the MicroCloud cluster.

LXD cluster
-----------

MicroCloud sets up a LXD cluster.
You can use the :command:`microcloud cluster` command to show information about the cluster members, or to remove specific members.

Apart from that, you can use LXD commands to manage the cluster.
See :ref:`lxd:clustering` in the LXD documentation for more information.

Networking
----------

.. include:: ../../README.md
   :parser: myst_parser.sphinx_
   :start-after: ## **What about networking?**
   :end-before: ## **What's next?**

See :ref:`lxd:network-ovn` in the LXD documentation for more information.

Roadmap
-------

.. include:: ../../README.md
   :parser: myst_parser.sphinx_
   :start-after: ## **What's next?**
   :end-before: ### **RESOURCES:**
