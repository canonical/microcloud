.. _howto-install:

How to install MicroCloud
=========================

To install MicroCloud, install all required :ref:`snaps` on all machines that you want to include in your cluster.

To do so, enter the following command on all machines::

  sudo snap install lxd microceph microovn microcloud --cohort="+"

.. note::
   Make sure to install the same version of the snaps on all machines.
   See :ref:`howto-snap` for more information.

   If you don't want to use MicroCloud's full functionality, you can install only some of the snaps.
   However, this is not recommended.
