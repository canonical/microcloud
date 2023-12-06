.. _howto-initialise:

How to initialise MicroCloud
============================

The :ref:`initialisation process <explanation-initialisation>` bootstraps the MicroCloud cluster.
You run the initialisation on one of the machines, and it configures the required services on all machines.

.. _howto-initialise-interactive:

Interactive configuration
-------------------------

If you run the initialisation process in interactive mode (the default), you are prompted for information about your machines and how you want to set them up.
The questions that you are asked might differ depending on your setup; for example, if you do not have the MicroOVN snap installed, you will not be prompted to configure your network, and if your machines don't have local disks, you will not be prompted to set up local storage.

The following instructions show the full initialisation process.

.. tip::
   During initialisation, MicroCloud displays tables of entities to choose from.

   To select specific entities, use the :kbd:`Up` and :kbd:`Down` keys to choose a table row and select it with the :kbd:`Space` key.
   To select all rows, use the :kbd:`Right` key.
   You can filter the table rows by typing one or more characters.

   When you have selected the required entities, hit :kbd:`Enter` to confirm.

Complete the following steps to initialise MicroCloud:

1. On one of the machines, enter the following command::

     sudo microcloud init

#. Select the IP address that you want to use for MicroCloud's internal traffic (see :ref:`microcloud-networking-intracluster`).
   MicroCloud automatically detects the available addresses (IPv4 and IPv6) on the existing network interfaces and displays them in a table.

   You must select exactly one address.
#. Decide if you want to limit the search for other machines.

   If you accept the default (``yes``), MicroCloud will automatically detect machines in the local subnet.
   Otherwise, it will detect all available machines, which might include duplicates (if machines are available both on IPv4 and on IPv6).

   See :ref:`automatic-server-detection` for more information.
#. Select the machines that you want to add to the MicroCloud cluster.

   MicroCloud displays all machines that it detects.
   Make sure that all machines that you select have the required snaps installed.
#. Select whether you want to set up local storage.

   .. note::
      To set up local storage, each machine must have a local disk.
      The disks must not contain any partitions.

   If you choose ``yes``, configure the local storage:

   1. Select the disks that you want to use for local storage.

      You must select exactly one disk from each machine.
   #. Select whether you want to wipe any of the disks.
      Wiping a disk will destroy all data on it.
#. Select whether you want to set up distributed storage (using MicroCeph).

   .. note::
      To set up distributed storage, you need at least three additional disks on at least three different machines.
      The disks must not contain any partitions.

   If you choose ``yes``, configure the distributed storage:

   1. Select the disks that you want to use for distributed storage.

      You must select at least three disks.
   #. Select whether you want to wipe any of the disks.
      Wiping a disk will destroy all data on it.
#. Select whether you want to set up distributed networking (using MicroOVN).

   If you choose ``yes``, configure the distributed networking:

   1. Select the network interfaces that you want to use (see :ref:`microcloud-networking-uplink`).

      You must select one network interface per machine.
   #. If you want to use IPv4, specify the IPv4 gateway on the uplink network (in CIDR notation) and the first and last IPv4 address in the range that you want to use with LXD.
   #. If you want to use IPv6, specify the IPv6 gateway on the uplink network (in CIDR notation).
#. MicroCloud now starts to bootstrap the cluster.
   Monitor the output to see whether all steps complete successfully.
   See :ref:`bootstrapping-process` for more information.

   Once the initialisation process is complete, you can start using MicroCloud.

See an example of the full initialisation process in the :ref:`Get started with MicroCloud <initialisation-process>` tutorial.

.. _howto-initialise-preseed:

Non-interactive configuration
-----------------------------

If you want to automate the initialisation process, you can provide a preseed configuration in YAML format to the :command:`microcloud init` command::

  cat <preseed_file> | microcloud init --preseed

The preseed YAML file must use the following syntax:

.. literalinclude:: preseed.yaml
   :language: YAML
   :emphasize-lines: 1,4-7,27,33-41
