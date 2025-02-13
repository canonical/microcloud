---
relatedlinks: https://documentation.ubuntu.com/lxd/
---

(explanation-microcloud)=
# About MicroCloud

The MicroCloud snap drives three other snaps ({doc}`lxd:index`, {doc}`microceph:index`, and {doc}`microovn:index`), enabling automated deployment of a highly available LXD cluster for compute, with Ceph as the storage driver and OVN as the managed network.

During initialisation, MicroCloud scrapes the other servers for details and then prompts you to add disks to Ceph and configure the networking setup.

At the end of this, you’ll have an OVN cluster, a Ceph cluster, and a LXD cluster. LXD itself will have been configured with both networking and storage suitable for use in a cluster.


## LXD cluster

MicroCloud sets up a LXD cluster. You can use the {command}`microcloud cluster` command to show information about the cluster members, or to remove specific members.

Apart from that, you can use LXD commands to manage the cluster. In the LXD documentation, see {ref}`lxd:clustering` for how-to guides on cluster management, or {ref}`lxd:exp-clusters` for an explanation of LXD clusters.

(explanation-networking)=
## Networking

By default, MicroCloud uses MicroOVN for networking, which is a minimal wrapper around OVN (Open Virtual Network).

- For an overview of MicroCloud networking with OVN, see: {ref}`exp-networking`.
- For networking requirements, see: {ref}`network-requirements`.
- To learn how to use a dedicated underlay network, see: {ref}`howto-ovn-underlay`.

## Storage

You have two options for storage in MicroCloud: local storage or distributed storage.

Local storage is faster, but less flexible and not fail-safe.
To use local storage, each machine in the cluster requires a local disk.
Disk sizes can vary.

For distributed storage, MicroCloud uses MicroCeph, which is a lightweight way of deploying a Ceph cluster.
To use distributed storage, you must have at least three disks (attached to at least three different machines).

## Troubleshooting

MicroCloud does not manage the services that it deploys.
After the deployment process, the individual services are operating independently.
If anything goes wrong, each service is responsible for handling recovery.

So, for example, if {command}`lxc cluster list` shows that a LXD cluster member is offline, follow the usual steps for recovering an offline cluster member (in the simplest case, restart the LXD snap on the machine).
The same applies to MicroOVN and MicroCeph.
