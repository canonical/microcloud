(howto-install)=
# How to install MicroCloud

(pre-deployment-requirements)=
## Pre-deployment requirements

````{tabs}
```{group-tab} General

The requirements in this section apply to all MicroCloud deployments.

A physical or virtual machine intended for use as a MicroCloud cluster member must meet the following prerequisites:

- Software:
  - Ubuntu 22.04 or newer (LTS version recommended)
    - If you intend to use ZFS storage, use a non-HWE (Hardware Enabled) variant of Ubuntu 22.04
  - snapd 2.59 or newer

- Networking:
  - Fixed IP addresses (DHCP not supported)
  - Two network interfaces per cluster member, one for intra-cluster communication and one for external connectivity to the uplink network
    - Partially or fully disaggregated networking setups require more interfaces; see: {ref}`howto-ceph-networking`
  - Uplink network must support both broadcast and multicast
  - Intra-cluster interface must have IPs assigned; external connectivity interface must not have any IPs assigned

- Storage:
  - Disks should be free of existing partitions or file systems
  - For local storage, each cluster member must have at least one local disk
  - If you intend to use full disk encryption on a cluster member, it must have `snapd` version `2.59.1` or newer installed and the `dm-crypt` kernel module available
    - To check if the module exists, run:

    ```
    sudo modinfo dm-crypt
    ```
```

```{group-tab} Testing or development environments

- Physical or virtual machines can be used
- Minimum cluster size:
  - 1 member
- Memory:
  - Minimum 8 GiB RAM per cluster member
- Storage:
  - If high availability is required, use distributed storage with:
    - a minimum of 3 cluster members
    - a minimum of 3 separate disks located across 3 different members
  - Otherwise, local storage is sufficient
```

```{group-tab} Production environments

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
```
````

For detailed information, see: {ref}`reference-requirements`.

## Installation

```{youtube} https://www.youtube.com/watch?v=M0y0hQ16YuE
```

To install MicroCloud, install all required {ref}`snaps` on all machines that you want to include in your cluster.

To do so, enter the following commands on all machines:

    sudo snap install lxd --channel=5.21/stable --cohort="+"
    sudo snap install microceph --channel=squid/stable --cohort="+"
    sudo snap install microovn --channel=24.03/stable --cohort="+"
    sudo snap install microcloud --channel=2/stable --cohort="+"

```{note}
Make sure to install the same version of the snaps on all machines.
See {ref}`howto-snap` for more information.

If you don't want to use MicroCloud's full functionality, you can install only some of the snaps.
However, this is not recommended.
```

After installing the snaps make sure to hold any automatic updates to keep the used snap versions across MicroCloud in sync.
See {ref}`howto-snap-hold-updates` for more information.
