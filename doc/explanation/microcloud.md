---
myst:
  html_meta:
    description: An overview of MicroCloud including its component snaps of LXD, MicroCeph, and MicroOVN, and its major features.
---

(exp-microcloud)=
(explanation-microcloud)=
# About MicroCloud

The MicroCloud snap drives three other snaps ({doc}`lxd:index`, {doc}`microceph:index`, and {doc}`microovn:index`), enabling automated deployment of a highly available LXD cluster for compute, with Ceph as the storage driver and OVN as the managed network.

During initialization, MicroCloud scrapes the other servers for details and then prompts you to add disks to Ceph and configure the networking setup.

At the end of this, you’ll have an OVN cluster, a Ceph cluster, and a LXD cluster. LXD itself will have been configured with both networking and storage suitable for use in a cluster.

(exp-microcloud-lxd)=
## LXD cluster

MicroCloud sets up a LXD cluster. You can use the {command}`microcloud cluster` command to show information about the cluster members, or to remove specific members.

Apart from that, you can use LXD commands to manage the cluster. In the LXD documentation, see {ref}`lxd:clustering` for how-to guides on cluster management, or {ref}`lxd:exp-clusters` for an explanation of LXD clusters.

(exp-microcloud-microovn)=
## MicroOVN networking

By default, MicroCloud uses MicroOVN for networking, which is a minimal wrapper around OVN (Open Virtual Network).

- For an overview of MicroCloud networking with OVN, see: {ref}`exp-networking`.
- For networking requirements, see: {ref}`reference-requirements-network`.
- To learn how to use a dedicated underlay network, see: {ref}`howto-ovn-underlay`.

(exp-microcloud-storage)=
## Storage

You have two options for storage in MicroCloud: local storage or distributed storage.

Local storage is faster, but less flexible and not fail-safe.
To use local storage, each machine in the cluster requires a local disk.
Disk sizes can vary.

```{note}
MicroCloud enables cluster-wide live migration on LXD.
When using local storage, consider the constraints listed in {ref}`lxd:storage-attach-volume`.
```

For distributed storage, MicroCloud uses MicroCeph, which is a lightweight way of deploying a Ceph cluster.
To use distributed storage, you must have at least three disks (attached to at least three different machines).

(exp-microcloud-ui)=
## The MicroCloud UI

You can securely access a browser-based graphical UI for managing your MicroCloud deployment.

For details, see: {ref}`howto-ui`.

```{admonition} Other client interfaces
:class: tip
You can also manage MicroCloud through the {ref}`command line <howto-commands>` or LXD's {ref}`lxd:rest-api`.
```

(exp-microcloud-scale)=
## Replicable at scale

MicroCloud is designed to be replicable at scale, enabling you to create consistent environments across multiple sites or clusters. All configuration performed during initialization can be captured in a {ref}`preseed file <howto-initialize-preseed>` to reproduce the deployment. This allows you to deploy identical MicroCloud clusters with minimal manual input.

Once deployed, each MicroCloud component (LXD, MicroCeph, MicroOVN) is designed to scale horizontally, meaning you can add more machines to increase capacity, performance, and redundancy. When new cluster members are added, these components automatically integrate them as control plane, storage, and networking peers without requiring manual reconfiguration. This includes {doc}`automatic failure domain adjustment <microceph:explanation/cluster-scaling>` for MicroCeph.

Furthermore, MicroCloud's snap-based updates help keep deployments consistent at scale. By updating cluster members to the latest version available on the LTS snap channel, you can ensure that all machines are using the same version with the latest security updates and bugfixes. See: {ref}`ref-releases-snaps`.

(exp-microcloud-ha)=
## High availability

MicroCloud achieves high availability (HA) through its distributed architecture: LXD for the control plane and workload management, MicroCeph for replicated, self-healing storage, and MicroOVN for redundant and distributed networking.

LXD provides control plane HA by allowing each cluster member to manage the cluster. If one member goes down, another can serve requests in its place. For data plane HA, LXD also provides automatic {ref}`cluster healing <lxd:cluster-healing>`. For more information, refer to the LXD documentation on {ref}`lxd:clusters-high-availability`.

Using distributed storage with MicroCeph means that data is replicated across the cluster, so even if one member goes offline, its data remains available on others. Ceph's {doc}`Controlled Replication Under Scalable Hashing (CRUSH) algorithm <ceph:rados/operations/crush-map>` automatically redistributes data when parts of the system fail, maintaining availability. Also see: the {ref}`MicroCloud storage requirements for high availability <reference-requirements-storage-ha>` and the {doc}`MicroCeph documentation on its failure domain management <microceph:explanation/cluster-scaling>`.

MicroOVN brings a distributed overlay network, meaning that switching and routing functions are not centralized on any single cluster member. Each member hosts its own virtual switch, avoiding a single point of failure for internal, intra-cluster traffic: every member can continue forwarding packets even if others are offline. External connectivity relies on a virtual router that is active on one member at a time; if that member fails, another takes over to keep uplink connectivity available. For more information, see: {ref}`exp-networking-ovn`.

(exp-microcloud-access-control)=
## Fine-grained access control and multi-tenancy

MicroCloud supports fine-grained access control through its underlying use of LXD's {ref}`authorization <lxd:authorization>` features. LXD defines entitlements (API-level actions) and {ref}`lxd:permissions` (granting an entitlement on a specific resource), and associates them with groups rather than individual identities.

For example, a group can be granted permission to view a particular instance without the ability to modify it, or permitted to manage specific resources without being allowed to edit cluster-level settings. This enables multiple groups to safely share one MicroCloud cluster, each with their own explicitly defined access policies.

For more information, refer to the LXD documentation: {ref}`lxd:authorization`.

(exp-microcloud-vms-containers)=
## System containers and virtual machines

Through LXD, MicroCloud can run both lightweight system containers and full virtual machines (VMs) for workload processing. System containers provide an environment that behaves like a complete operating system (including the ability to run multiple processes) while sharing the host's kernel, making them smaller and more resource-efficient. VMs include their own kernel, offering stronger isolation and the flexibility to run a different operating system from the host.

For more information, refer to the LXD documentation: {ref}`lxd:containers-and-vms`.

(exp-microcloud-troubleshooting)=
## Troubleshooting

MicroCloud does not manage the services that it deploys.
After the deployment process, the individual services are operating independently.
If anything goes wrong, each service is responsible for handling recovery.

So, for example, if {command}`lxc cluster list` shows that a LXD cluster member is offline, follow the usual steps for recovering an offline cluster member (in the simplest case, restart the LXD snap on the machine).
The same applies to MicroOVN and MicroCeph.
