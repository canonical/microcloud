(howto-install)=
# How to install MicroCloud

(pre-deployment-requirements)=
## Pre-deployment requirements

`````{tabs}
````{group-tab} General

The requirements in this section apply to all MicroCloud deployments.

A physical or virtual machine intended for use as a MicroCloud cluster member must meet the following prerequisites:

- Software:
  - Ubuntu 22.04 or newer (LTS version recommended)
    - If you intend to use ZFS storage, use a non-HWE (Hardware Enabled) variant of Ubuntu 22.04
  - snapd 2.59 or newer

- Networking:
  - Fixed IP addresses (DHCP not supported)
  - At least two network interfaces per cluster member: one for intra-cluster communication and one for external connectivity to the uplink network
    - Partially or fully disaggregated networking setups require more interfaces; see: {ref}`howto-ceph-networking`
    - To use a {ref}`dedicated underlay network for OVN traffic <exp-networking-ovn-underlay>`, an additional interface per cluster member is required
  - Uplink network must support both broadcast and multicast
  - Intra-cluster interface must have IPs assigned; external connectivity interface (to uplink) must not have any IPs assigned

- Storage:
  - Disks should be free of existing partitions or file systems
  - For local storage, each cluster member must have at least one local disk
  - If you intend to use full disk encryption on a cluster member, it must have `snapd` version `2.59.1` or newer installed and the `dm-crypt` kernel module available
    - To check if the module exists, run:

    ```
    sudo modinfo dm-crypt
    ```
````

````{group-tab} Testing or development environments

```{important}
These requirements are in addition to those listed in the General tab.
```

- Physical or virtual machines can be used
- Minimum cluster size:
  - 1 member
- Memory:
  - Minimum 8 GiB RAM per cluster member
- Networking:
  - It is possible to use a single network interface per cluster member. However, such a configuration is neither supported nor recommended. For details, see: {ref}`reference-requirements-network-interface-single`.
- Storage:
  - If high availability is required, use distributed storage with:
    - a minimum of 3 cluster members
    - a minimum of 3 separate disks located across 3 different members
  - Otherwise, local storage is sufficient
````

````{group-tab} Production environments

```{important}
These requirements are in addition to those listed in the General tab.
```

- Physical machines only (no VMs)
- Minimum cluster size:
  - 3 members
  - For critical deployments, we recommend a minimum of 4 members
- Memory:
  - Minimum 32 GiB RAM per cluster member
- Software:
  - For production deployments subscribed to Ubuntu Pro, each cluster member must use a LTS version of Ubuntu
- Networking:
  - For each cluster member, we recommend dual-port network cards with a minimum 10 GiB capacity, or higher if low latency is essential
- Storage:
  - For each cluster member, we recommend at least 3 NVMe disks:
    - 1 for OS
    - 1 for local storage
    - 1 for distributed storage
````
`````

For detailed information, see: {ref}`reference-requirements`.

## Installation

```{youtube} https://www.youtube.com/watch?v=M0y0hQ16YuE
```

To install MicroCloud, install all required {ref}`snaps <reference-requirements-software-snaps>` on all machines that you want to include in your cluster. You can {ref}`optionally specify a channel <howto-install-specify-channel>` for each snap, but generally, you can leave out the channel to use the current recommended default. 

To do so, enter the following commands on all machines:

```bash
sudo snap install lxd --cohort="+"
sudo snap install microceph --cohort="+"
sudo snap install microovn --cohort="+"
sudo snap install microcloud --cohort="+"
```

The `--cohort` flag ensures that versions remain {ref}`synchronized during later updates <howto-update-sync>`.

Following installation, make sure to {ref}`hold updates <howto-update-hold>`.

### Previously installed snaps

If a required snap is already installed on your machine, you will receive a message to that effect. In this case, check the version for the installed snap:

```bash
snap list <snap>
```

View the {ref}`matrix of compatible versions <ref-releases-matrix>` to determine whether you need to upgrade the snap to a different channel. Follow either the update or upgrade instructions below.

#### Update

If the installed snap is using a channel corresponding to a release that is compatible with the other snaps, update to the most recent stable version of the snap without changing the channel:

```bash
sudo snap refresh <snap> --cohort="+"
```

#### Upgrade

If you need to upgrade the channel, run:

```bash
sudo snap refresh <snap> --cohort="+" --channel=<target channel>
```

Example:

```bash
sudo snap refresh microcloud --cohort="+" --channel=2/stable
```

(howto-install-specify-channel)=
### Optionally specify a channel

Channels correspond to different {ref}`releases <ref-releases-snaps>`. When unspecified, MicroCloud and its components' snaps use their respective recommended default channels.

To specify a different channel, add the `--channel` flag at installation:

```bash
sudo snap install <snap> --cohort="+" --channel=<target channel>
```

For example, to use the `3/edge` channel for the MicroCloud snap, run:

```bash
sudo snap install microcloud --cohort="+" --channel=3/edge
```

For details about the MicroCloud snap channels, see: {ref}`ref-snaps-microcloud-channels`.

(howto-install-hold-updates)=
## Hold updates

When a new release is published to a snap channel, installed snaps following that channel update automatically by default. This is undesired behavior for MicroCloud and its components, and you should override this default behavior by holding updates. See: {ref}`howto-update-hold`.
