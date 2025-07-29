(howto-snap)=
# How to manage the snaps

MicroCloud is distributed as a [snap](https://snapcraft.io/docs).
The benefit of packaging MicroCloud as a snap is that it makes it possible to include the required dependencies, and that it allows MicroCloud to be installed on many different Linux distributions.
The snap ensures that MicroCloud runs in a consistent environment.

Because MicroCloud uses a set of {ref}`other snaps <reference-requirements-software-snaps>`, you must make sure to have suitable versions of these snaps installed on all machines of your MicroCloud cluster.
The installed snap versions must be compatible with one another, and for each of the snaps, the same version must be installed on all machines.

## Choose the right channel and track

Snaps come with different channels that define which release of a snap is installed and tracked for updates.
See [Channels and tracks](https://snapcraft.io/docs/channels) in the snap documentation for detailed information.

MicroCloud currently provides the `2` LTS and `3` development track. The `1` track reached {abbr}`EOL (End of Life)` at the end of April 2025.

```{tip}
In general, you should use the default channels for all snaps required to run MicroCloud.

See {ref}`howto-support` for a list of supported channels that are orchestrated to work together.
```

When installing a snap, specify the channel as follows:

    sudo snap install <snap_name> --channel=<channel>

For example:

    sudo snap install microcloud --channel=2/stable

To see all available channels of a snap, run the following command:

    snap info <snap_name>

(howto-snap-control-updates)=
## Control updates

By default, snaps are updated automatically.
In the case of MicroCloud, this can be problematic because the related snaps must always use compatible versions, and because all machines of a cluster must use the same version of each snap.

Therefore, you should manually apply your updates and make sure that all cluster members are in sync regarding the snap versions that they use.

(howto-snap-hold-updates)=
### Hold updates

You can hold snap updates for a specific time or forever, for all snaps or for specific snaps.

Which strategy to choose depends on your use case.
If you want to fully control updates to your MicroCloud setup, you should put a hold on all related snaps until you decide to update them.

Enter the following command to indefinitely hold all updates to the snaps needed for MicroCloud:

    sudo snap refresh --hold lxd microceph microovn microcloud

See [Hold refreshes](https://snapcraft.io/docs/managing-updates#heading--hold) in the snap documentation for detailed information about holding snap updates.

(howto-snap-cluster)=
### Keep cluster members in sync

Snap updates are delivered as [progressive releases](https://snapcraft.io/docs/progressive-releases), which means that updated snap versions are made available to different machines at different times.
This method can cause a problem for cluster updates if some cluster members are refreshed to a version that is not available to other cluster members yet.

To avoid this problem, use the `--cohort="+"` flag when refreshing your snaps:

    sudo snap refresh <snap> --cohort="+"

This flag ensures that all machines in a cluster see the same snap revision and are therefore not affected by a progressive rollout.

## Use an Enterprise Store Proxy

If you manage a large MicroCloud deployment and you need absolute control over when updates are applied, consider installing an Enterprise Store Proxy.

The Enterprise Store Proxy is a separate application that sits between the snap client command on your machines and the snap store.
You can configure the Enterprise Store Proxy to make only specific snap revisions available for installation.

See the [Enterprise Store Proxy documentation](https://documentation.ubuntu.com/enterprise-store/) for information about how to install and register the Enterprise Store Proxy.

After setting it up, configure the snap clients on all cluster members to use the proxy.
See [Configuring devices](https://documentation.ubuntu.com/enterprise-store/main/how-to/devices/) for instructions.

You can then configure the Enterprise Store Proxy to override the revisions for the snaps that are needed for MicroCloud:

    sudo snap-proxy override lxd <channel>=<revision>
    sudo snap-proxy override microceph <channel>=<revision>
    sudo snap-proxy override microovn <channel>=<revision>
    sudo snap-proxy override microcloud <channel>=<revision>
