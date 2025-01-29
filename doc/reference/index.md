(reference)=
# Reference

The reference material in this section provides technical descriptions of MicroCloud.

(reference-requirements)=
## Requirements

(hardware-requirements)=
### Hardware requirements

MicroCloud supports up to 50 machines as members of the same cluster.

- For testing and development, a single machine (physical or virtual) can be used to create a cluster. 

- For production environments, a minimum of 3 physical machines is required as cluster members. We do not recommend using virtual machines as cluster members in production.

- For critical deployments, we recommend a minimum of 4 cluster members. While 3 members are sufficient for high availability, an extra member provides redundancy for running critical applications.


```{tip}
If you want to add further members to a cluster after initialisation, use the {command}`microcloud add` command.
```

You can mix different processor architectures within the same MicroCloud cluster. 

Each cluster member must have at least 8 GiB of RAM (more depending on the connected disks). We recommend at least 32 GiB of RAM for production environments.

### Storage requirements

All storage disks should be free of existing partitions or file systems.

Also see Ceph's {ref}`ceph:hardware-recommendations`.

#### Local storage
Local storage is typically sufficient for testing and development, as it is fast and convenient. To use local storage, each cluster member requires a local disk.


#### Distributed storage
You can set up distributed storage on a test cluster with a single member, but it won’t have the recommended replication configuration which ensures high availability.

#### High availability

For high availability, and the ability to recover a cluster should something go wrong, use distributed storage with at least three additional disks for use by Ceph. These disks must be on at least three different cluster members.

### Production environments

For production environments, we recommend at least 3 NVMe disks per cluster member:
- 1 for OS
- 1 for local storage
- 1 for distributed storage

### Full disk encryption

If you intend to use full disk encryption on any cluster member, the `dm-crypt` kernel module must be available, and the snap `dm-crypt` plug must be connected to MicroCeph. The `dm-crypt` module is available by default in Ubuntu 24.04 and higher.

For further information, see the Prerequisites section of this page: {doc}`microceph:explanation/full-disk-encryption`. Note that the command shown on that page to connect the snap `dm-crypt` plug can only be performed once MicroCeph is installed. The MicroCloud installation steps include installing MicroCeph; thus, {ref}`install MicroCloud first<howto-install>`, then connect the plug. 

### Networking requirements

For networking, MicroCloud requires at least two dedicated network interfaces: one for intra-cluster communication and one for external connectivity.

In production environments, we recommend dual-port network cards with a minimum 10 GiB capacity, or higher if low latency is essential.

If you want to partially or fully disaggregate the Ceph networks and the OVN underlay network, you need more dedicated interfaces. For details, see: {ref}`howto-ceph-networking`.

To allow for external connectivity, MicroCloud requires an uplink network that supports broadcast and multicast. See {ref}`explanation-networking` for more information.

The intra-cluster interface must have IPs assigned, whereas the external connectivity interface must not have any IPs assigned.

The IP addresses of the cluster members must not change after installation, so DHCP is not supported.

### Software requirements

MicroCloud requires snapd version 2.59 or newer.

We recommend an LTS version of Ubuntu 22.04 or newer. Production deployments subscribed to Ubuntu Pro are required to use an LTS version. 

If you intend to use ZFS storage, use a non-HWE (Hardware Enabled) variant of Ubuntu 22.04.

Also see LXD's {ref}`lxd:requirements` and Ceph's {doc}`ceph:start/os-recommendations`.

(snaps)=
## Snaps

To run MicroCloud, you must install the following snaps:

- [MicroCloud snap](https://snapcraft.io/microcloud)
- [LXD snap](https://snapcraft.io/lxd)
- [MicroCeph snap](https://snapcraft.io/microceph)
- [MicroOVN snap](https://snapcraft.io/microovn)

See {ref}`howto-install` for installation instructions.
