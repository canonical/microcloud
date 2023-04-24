.. _reference:

Reference
=========

The reference material in this section provides technical descriptions of MicroCloud.

Requirements
------------

.. include:: ../../README.md
   :parser: myst_parser.sphinx_
   :start-after: Requirements?**
   :end-before: Once the simple

You can mix different processor architectures within the same MicroCloud cluster.

If you want to add further machines after the initial initialisation, you can use the :command:`microcloud add` command.

For networking, MicroCloud requires a dedicated network interface and an uplink network that is an actual L2 subnet.
See :ref:`explanation-networking` for more information.

.. _snaps:

Snaps
-----

To run MicroCloud, you must install the following snaps:

- `MicroCloud snap`_
- `LXD snap`_
- `MicroCeph snap`_
- `MicroOVN snap`_
