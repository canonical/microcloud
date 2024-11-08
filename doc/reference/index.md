(reference)=
# Reference

The reference material in this section provides technical descriptions of MicroCloud.

## Requirements

(hardware-requirements)=
### Hardware requirements

MicroCloud requires a minimum of three machines.
It supports up to 50 machines.

Each machine must have at least 8 GiB of RAM (more depending on the connected disks).
You can mix different processor architectures within the same MicroCloud cluster.

If you want to add further machines after the initial initialisation, you can use the {command}`microcloud add` command.

To use local storage, each machine requires a local disk.
To use distributed storage, at least three additional disks (not only partitions) for use by Ceph are required, and these disks must be on at least three different machines.

Also see Ceph's {ref}`ceph:hardware-recommendations`.

### Networking requirements

For networking, MicroCloud requires at least two dedicated network interfaces: one for intra-cluster communication and one for external connectivity. If you want to segregate the Ceph networks and the OVN underlay network, you might need more dedicated interfaces.

To allow for external connectivity, MicroCloud requires an uplink network that supports broadcast and multicast. See {ref}`explanation-networking` for more information.

The IP addresses of the machines must not change after installation, so DHCP is not supported.

### Software requirements

MicroCloud requires snapd version 2.59 or newer.

Also see LXD's {ref}`lxd:requirements` and Ceph's {doc}`ceph:start/os-recommendations`.

(snaps)=
## Snaps

To run MicroCloud, you must install the following snaps:

- [MicroCloud snap](https://snapcraft.io/microcloud)
- [LXD snap](https://snapcraft.io/lxd)
- [MicroCeph snap](https://snapcraft.io/microceph)
- [MicroOVN snap](https://snapcraft.io/microovn)
