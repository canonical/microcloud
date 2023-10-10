.. _howto-snap:

How to manage the snaps
=======================

MicroCloud is distributed as a `snap`_.
The benefit of packaging MicroCloud as a snap is that it makes it possible to include the required dependencies, and that it allows MicroCloud to be installed on many different Linux distributions.
The snap ensures that MicroCloud runs in a consistent environment.

Because MicroCloud uses a set of :ref:`other snaps <snaps>`, you must make sure to have suitable versions of these snaps installed on all machines of your MicroCloud cluster.
The installed snap versions must be compatible with one another, and for each of the snaps, the same version must be installed on all machines.

Choose the right channel and track
----------------------------------

Snaps come with different channels that define which release of a snap is installed and tracked for updates.
See `Channels and tracks`_ in the snap documentation for detailed information.

MicroCloud currently provides only the ``latest`` track.

.. tip::
   In general, you should use the ``latest/stable`` channel for all snaps required to run MicroCloud.

   However, since MicroCloud is not ready for production yet, you might want to use the ``latest/edge`` channel of the MicroCloud snap to benefit from the latest bug fixes.

When installing a snap, specify the channel as follows::

  sudo snap install <snap_name> --channel=<channel>

For example::

  sudo snap install microcloud --channel=latest/stable

To see all available channels of a snap, run the following command::

  snap info <snap_name>

Control updates
---------------

By default, snaps are updated automatically.
In the case of MicroCloud, this can be problematic because the related snaps must always use compatible versions, and because all machines of a cluster must use the same version of each snap.

Therefore, you should schedule your updates and make sure that all cluster members are in sync regarding the snap versions that they use.

Schedule updates
~~~~~~~~~~~~~~~~

There are two methods for scheduling when your snaps should be updated:

- You can hold snap updates for a specific time, either for specific snaps or for all snaps on your system.
  After the duration of the hold, or when you remove the hold, your snaps are automatically refreshed.
- You can specify a system-wide refresh window, so that snaps are automatically refreshed only within this time frame.
  Such a refresh window applies to all snaps.

Hold updates
^^^^^^^^^^^^

You can hold snap updates for a specific time or forever, for all snaps or for specific snaps.

Which strategy to choose depends on your use case.
If you want to fully control updates to your MicroCloud setup, you should put a hold on all related snaps until you decide to update them.

Enter the following command to indefinitely hold all updates to the snaps needed for MicroCloud::

  sudo snap refresh --hold lxd microceph microovn microcloud

When you choose to update your installation, use the following commands to remove the hold, update the snaps, and hold the updates again::

  sudo snap refresh --unhold lxd microceph microovn microcloud
  sudo snap refresh lxd microceph microovn microcloud --cohort="+"
  sudo snap refresh --hold lxd microceph microovn microcloud

See `Hold refreshes`_ in the snap documentation for detailed information about holding snap updates.

Specify a refresh window
^^^^^^^^^^^^^^^^^^^^^^^^

Depending on your setup, you might want your snaps to update regularly, but only at specific times that don't disturb normal operation.

You can achieve this by specifying a refresh timer.
This option defines a refresh window for all snaps that are installed on the system.

For example, to configure your system to update snaps only between 8:00 am and 9:00 am on Mondays, set the following option::

  sudo snap set system refresh.timer=mon,8:00-9:00

You can use a similar mechanism (setting ``refresh.hold``) to hold snap updates as well.
However, in this case the snaps will be refreshed after 90 days, irrespective of the value of ``refresh.hold``.

See `Control updates with system options`_ in the snap documentation for detailed information.

Keep cluster members in sync
~~~~~~~~~~~~~~~~~~~~~~~~~~~~

The cluster members that are part of the MicroCloud deployment must always run the same version of the snaps.
This means that when the snaps on one of the cluster members are refreshed, they must also be refreshed on all other cluster members before MicroCloud is operational again.

Snaps updates are delivered as `progressive releases`_, which means that updated snap versions are made available to different machines at different times.
This method can cause a problem for cluster updates if some cluster members are refreshed to a version that is not available to other cluster members yet.

To avoid this problem, use the ``--cohort="+"`` flag when refreshing your snaps::

  sudo snap refresh lxd microceph microovn microcloud --cohort="+"

This flag ensures that all machines in a cluster see the same snap revision and are therefore not affected by a progressive rollout.

Use a Snap Store Proxy
----------------------

If you manage a large MicroCloud deployment and you need absolute control over when updates are applied, consider installing a Snap Store Proxy.

The Snap Store Proxy is a separate application that sits between the snap client command on your machines and the snap store.
You can configure the Snap Store Proxy to make only specific snap revisions available for installation.

See the `Snap Store Proxy documentation`_ for information about how to install and register the Snap Store Proxy.

After setting it up, configure the snap clients on all cluster members to use the proxy.
See `Configuring snap devices`_ for instructions.

You can then configure the Snap Store Proxy to override the revisions for the snaps that are needed for MicroCloud::

  sudo snap-proxy override lxd <channel>=<revision>
  sudo snap-proxy override microceph <channel>=<revision>
  sudo snap-proxy override microovn <channel>=<revision>
  sudo snap-proxy override microcloud <channel>=<revision>
