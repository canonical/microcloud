
(reference-requirements)=
# Requirements

(reference-requirements-hardware)=
## Hardware requirements

MicroCloud supports up to 50 machines as members of the same cluster.

- For testing and development, you can use physical or virtual machines as cluster members. A single machine can be used to create a single-member cluster.

- For production environments, a minimum of 3 physical machines is required as cluster members. We do not recommend using virtual machines as cluster members in production.

- For critical deployments, we recommend a minimum of 4 cluster members. While 3 members are sufficient for high availability, an extra member provides redundancy for running critical applications.

```{tip}
If you want to add further members to a cluster after initialization, use the {command}`microcloud add` command.
```

(reference-requirements-processor)=
## Processor architectures

You can mix different processor architectures within the same MicroCloud cluster.

(reference-requirements-memory)=
## Memory requirements

We recommend at least:

- 8 GiB of RAM for test and development environments
- 32 GiB of RAM for production environments

For detailed RAM requirements related to distributed storage, see the RAM and Memory sections of {ref}`Ceph's hardware requirements <ceph:hardware-recommendations>`.

(reference-requirements-storage)=
## Storage requirements

All storage disks should be free of existing partitions or file systems.

Also see Ceph's {ref}`ceph:hardware-recommendations`.

(reference-requirements-storage-local)=
### Local storage

Local storage is fast and convenient. It is commonly used in production and is typically sufficient on its own for testing and development. To use local storage, each cluster member requires at least one local disk.

(reference-requirements-storage-distributed)=
### Distributed storage

You can set up distributed storage on a test cluster with a single member, but such a cluster cannot support the recommended replication configuration which ensures high availability.

(reference-requirements-storage-ha)=
### High availability

For high availability, and the ability to {ref}`recover a cluster <howto-recover>` should something go wrong, use distributed storage with at least three additional disks for use by {doc}`ceph:index`. These disks must be located across at least three different cluster members.

(reference-requirements-storage-production)=
## Production environments

For production environments, we recommend at least 3 NVMe disks per cluster member:
- 1 for OS
- 1 for local storage
- 1 for distributed storage

(reference-requirements-storage-fde)=
## Full disk encryption

If you intend to use full disk encryption on any cluster member, the `dm-crypt` kernel module must be available, and the snap `dm-crypt` plug must be connected to MicroCeph. The `dm-crypt` module is available by default in Ubuntu 24.04 and higher.

For further information, see the Prerequisites section of this page: {doc}`microceph:explanation/security/full-disk-encryption`. Note that the command shown on that page to connect the snap `dm-crypt` plug can only be performed once MicroCeph is installed. The MicroCloud installation steps include installing MicroCeph; thus, {ref}`install MicroCloud first<howto-install>`, then connect the plug.

(reference-requirements-network)=
## Networking requirements

(reference-requirements-network-interfaces)=
### Required network interfaces

For networking, each machine in the MicroCloud cluster requires at least two dedicated network interfaces: one for intra-cluster communication and one for external connectivity. These can be connected to the same network, or different networks.

In production environments, we recommend dual-port network cards with a minimum 10 GiB capacity, or higher if low latency is essential.

(reference-requirements-network-interfaces-intracluster)=
Network interface for intra-cluster traffic
:  MicroCloud requires one network interface that is pre-configured with an IP address (IPv4 or IPv6) that is within the same subnet as the IPs of the other cluster members. The network that it is connected to must support multicast.

   This network interface can be, for example, a dedicated physical network interface, a VLAN, or a virtual function on an {abbr}`SR-IOV (Single root I/O virtualization)`-capable network interface.
   It serves as the dedicated network interface for MicroCloud and is used for multicast discovery (during setup) and all internal traffic between the MicroCloud, OVN, and Ceph members.

(reference-requirements-network-interfaces-uplink)=
Network interface to connect to the uplink network
:  MicroCloud requires one network interface for connecting OVN to the uplink network.
   This network interface must either be an unused interface that does not have an IP address configured, or a bridge.

   MicroCloud configures this interface as an uplink interface that provides external connectivity to the MicroCloud cluster.

   You can specify a different interface to be used as the uplink interface for each cluster member.
   MicroCloud requires that all uplink interfaces are connected to the uplink network, using the gateway and IP address range information that you provide during the MicroCloud initialization process.

(reference-requirements-network-nics-optional)=
### Optional network interfaces

MicroCloud requires additional network interfaces to implement either of the following optional setups:

- Dedicated networks for Ceph: see {ref}`howto-ceph-networking`
- Dedicated underlay network for OVN traffic: see {ref}`howto-ovn-underlay`

These interfaces can be physical {abbr}`NICs (Network Interface Cards)`, or VLAN interfaces on top of one or more NICs (which can optionally be bonded for failover).

(reference-requirements-network-uplink)=
### Uplink network

To configure external connectivity, MicroCloud requires an uplink network that supports broadcast and multicast. For more information about the uplink network, see the LXD documentation on the {ref}`network-ovn`.

(reference-requirements-network-ip)=
### Cluster member IP addresses

The IP addresses of the cluster members must not change after installation, so they must be configured as static addresses.

(reference-requirements-software)=
## Software requirements

MicroCloud requires snapd version 2.59 or newer.

We recommend an LTS version of Ubuntu 22.04 or newer. Production deployments subscribed to Ubuntu Pro are required to use an LTS version.

If you intend to use ZFS storage, use a non-HWE (Hardware Enabled) variant of Ubuntu 22.04.

Also see LXD's {doc}`lxd:requirements` and Ceph's {doc}`ceph:start/os-recommendations`.

(reference-requirements-software-snaps)=
### Snaps

To run MicroCloud, you must install the following snaps:

- [MicroCloud snap](https://snapcraft.io/microcloud)
- [LXD snap](https://snapcraft.io/lxd)
- [MicroCeph snap](https://snapcraft.io/microceph)
- [MicroOVN snap](https://snapcraft.io/microovn)

See {ref}`howto-install` for installation instructions.
