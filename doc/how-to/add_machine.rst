.. _howto-add:

====================
How to add a machine
====================

If you want to add a machine to the MicroCloud cluster after the initialisation, use the :command:`microcloud add` command::

  sudo microcloud add

Answer the prompts to add the machine.
Alternatively, you can add the ``--auto`` flag to accept the default configuration instead of an interactive setup.
You can also add the ``--wipe`` flag to automatically wipe any disks you add to the cluster.
