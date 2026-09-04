:relatedlinks: [MicroCloud:&#32;your&#32;open&#32;source&#32;cloud&#32;platform](https://canonical.com/microcloud)

.. meta::
   :description: MicroCloud is a lightweight, open source cloud platform built on LXD, MicroCeph, and MicroOVN, ideal for private cloud, edge computing, and test labs.

.. _home:

MicroCloud
==========

Deploy a low-touch, open source cloud platform in minutes with MicroCloud.

MicroCloud creates a lightweight cluster of machines that operates as a scalable private cloud.
It combines LXD for virtualization, MicroCeph for distributed storage, and MicroOVN for networking—all automatically configured by the `MicroCloud snap <https://snapcraft.io/microcloud>`__ for :ref:`reproducible, scalable deployments <exp-microcloud-scale>`.

With MicroCloud, you can eliminate the complexity of manual setup and quickly benefit from :ref:`high availability <exp-microcloud-ha>`, :ref:`streamlined security updates <ref-releases-snaps>`, and :ref:`fine-grained access control for multi-tenancy <exp-microcloud-access-control>`.
Cluster members can run :ref:`full virtual machines or lightweight system containers <exp-microcloud-vms-containers>` with bare-metal performance. Manage it through your choice of client interfaces, including a :ref:`graphical UI <howto-ui>` and :ref:`CLI <ref-commands>`.

MicroCloud is designed for small-scale private clouds and hybrid cloud extensions. Its efficiency and simplicity also make it an excellent choice for edge computing, test labs, and other resource-constrained use cases.

.. figure:: /images/microcloud_basic_architecture.svg
   :alt: A diagram of basic MicroCloud setup architecture
   :align: center
   :width: 75%

----

In this documentation
---------------------

.. domain::

   .. slice:: Get started

      :doc:`About MicroCloud <explanation/microcloud>`
      :doc:`Tutorial with multiple virtualized cluster members <tutorial/multi-member>`
      :doc:`Tutorial with a single physical cluster member <tutorial/single-member>`

   .. slice:: Clusters and cluster members

      :doc:`Add members <how-to/member_add>`
      :doc:`Remove members <how-to/member_remove>`
      :doc:`Shut down members <how-to/member_shutdown>`

   .. slice:: Multiple clusters

      :doc:`Manage multiple clusters with the Cluster Manager <how-to/cluster_manager>`
      :doc:`Cluster Manager architecture <reference/cluster-manager-architecture>`
      :doc:`Cluster Manager API reference <reference/cluster-manager-api>`

   .. slice:: Storage and networking

      :doc:`Local and distributed storage <explanation/storage>`
      :doc:`MicroCloud's approach to networking <explanation/networking>`
      :doc:`Configure a dedicated Ceph network <how-to/ovn_underlay>`
      :doc:`Configure an OVN underlay <how-to/ceph_networking>`
      :doc:`Add a service <how-to/add_service>`

   .. slice:: Security

      :doc:`Overview <explanation/security>` slice
      :doc:`Initialization process <explanation/initialization>`

   .. slice:: Setup

      :doc:`Requirements <reference/requirements>` slice
      :doc:`Installation <how-to/install>`
      :doc:`Initialization <how-to/initialize>`
      :doc:`Preseed file fields for non-interactive configuration <reference/preseed>`
      :doc:`Automate initialization with Terraform <how-to/terraform_automation>`
      :doc:`Common CLI commands <reference/commands>`
      :doc:`Access the UI <how-to/ui>`

   .. slice:: Maintenance

      :doc:`Recover a cluster <how-to/recover>`
      :doc:`Update and upgrade <how-to/update_upgrade>`
      :doc:`Manage the snaps <how-to/snaps>`
      :doc:`Releases and snaps <reference/releases-snaps>`
      :doc:`Release notes <reference/release-notes/index>`
      :doc:`Decommission a MicroCloud <how-to/decommission>`

----

About the integrated documentation sets
---------------------------------------

The three components of MicroCloud (:doc:`lxd:index`, :doc:`microceph:index`, and :doc:`microovn:index`) each offer their own documentation sets, available at their respective standalone documentation sites.

For convenience, this site provides not only MicroCloud's documentation but also an integrated view of all four documentation sets.
You can easily switch between sets using the links in the site header, allowing you to explore all the related documentation without leaving this site.

.. note::

   The components' documentation sets are written for a general audience that might not be using MicroCloud.
   Thus, not all the information in these sets are relevant to MicroCloud users.
   For example, since MicroCloud automates the installation of its components, you can ignore the manual installation instructions in the components' documentation.

   Also, while each component's documentation includes instructions for removing cluster members, you should not remove members from only one component.
   Use MicroCloud instead to remove cluster members (see :ref:`howto-member-remove`).

----

How this documentation is organized
-----------------------------------

This documentation uses the `Diátaxis documentation structure <https://diataxis.fr/>`__.

..
  Turn spell check off temporarily until this issue is resolved:
  https://github.com/canonical/documentation-style-guide/issues/207
  (Remove both `vale off` and `vale on` lines.)

.. vale off

- The :ref:`tutorials` introduce you to MicroCloud concepts and usage.
- The :ref:`howto` provide detailed setup and usage instructions, including how to access the UI and manage cluster members.
- The :ref:`reference` guides provide technical details, release notes, and common CLI commands.
- The :ref:`explanation` section includes topic overviews and detailed explanations of key concepts, such as local versus distributed storage.

.. vale on

----

Project and community
---------------------

MicroCloud is a member of the `Canonical <https://canonical.com>`__ family.
It’s an open source project that warmly welcomes community contributions, suggestions, fixes, and constructive feedback.

Get involved
------------

- :ref:`Support <howto-support>`
- `Discussion forum <https://discourse.ubuntu.com/c/project/lxd/microcloud/145>`__
- :ref:`Contribute <howto-contribute>`

Releases
--------

- :ref:`ref-release-notes`

Governance and policies
-----------------------

- `Code of conduct <https://ubuntu.com/community/docs/ethos/code-of-conduct>`__

Commercial support
------------------

Thinking about using MicroCloud for your next project?
`Get in touch <https://canonical.com/microcloud/contact-us>`__!

.. toctree::
   :hidden:
   :maxdepth: 2

   self
   /tutorial/index
   /how-to/index
   /reference/index
   /explanation/index
