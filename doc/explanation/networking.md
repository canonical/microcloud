---
relatedlinks: https://docs.ovn.org,https://ubuntu.com/blog/data-centre-networking-what-is-ovn,https://en.wikipedia.org/wiki/Virtual_private_cloud
---

(exp-networking)=
# MicroCloud's networking approach

This page describes a high-level overview of MicroCloud's networking approach, particularly its use of [Open Virtual Network (OVN)](https://www.ovn.org/). It also compares and contrasts MicroCloud networking to [Virtual Private Clouds (VPCs)](https://en.wikipedia.org/wiki/Virtual_private_cloud).

(exp-networking-ovn)=
## OVN and MicroCloud

OVN is an open source software-defined networking (SDN) solution built on [Open vSwitch (OVS)](https://www.openvswitch.org/). 

MicroCloud's implementation of OVN minimizes manual configuration, reducing a normally complex setup process to mere minutes. During {ref}`initialization <howto-initialize>`, MicroCloud defaults to configuring distributed networking with {doc}`microovn:index`, which is a minimal wrapper around OVN. 

Along with enabling communication between MicroCloud cluster members, OVN also manages key network components such as overlays, virtual routers and switches, NAT, and more. 

(exp-networking-ovn-features)=
### Key features and benefits

- **Logical networks**: OVN networks are logical networks implemented in software, which means they operate independently of physical network infrastructure and allow for segmentation within the same cluster. For example, you can use separate networks for frontend and backend components.

- **Distributed virtual routers and logical switches**: Data plane operations are distributed across cluster members rather than run by a centralized controller, allowing network traffic to be processed at the edge, closer to where workloads run.

- **Consistent networking across locations**: Since OVN logical networks are software-defined, the same configurations can be consistently deployed across multiple clusters.

- **Dynamic address management**: Manages DHCP and DNS within the cluster.

- **Access control lists**: Define security policies for network access.

- **Resiliency**: OVN virtual routers are highly available; they can automatically fail over to another cluster member as needed.

(exp-networking-ovn-architecture)=
### Networking architecture

OVN requires the use of cloud management software (CMS) to connect OVN logical networks to a physical network. Since MicroCloud is built on {ref}`LXD clusters <lxd:exp-clusters>`, LXD acts as the CMS for MicroCloud, enabling the connection of an OVN network to an existing managed {ref}`lxd:network-bridge` or {ref}`lxd:network-physical` for external connectivity.

The following figure shows the OVN network traffic flow in a MicroCloud/LXD cluster:

```{figure} /images/ovn_networking_1.svg
:width: 100%

OVN networking (single network)
```

In MicroCloud, each OVN network spans across all cluster members. Intra-cluster traffic (communication between instances on members within the same cluster, also known as OVN east/west traffic) travels through an OVN tunnel over a designated {abbr}`NIC (Network Interface Card)` (`eth1` in the figure above).

For external connectivity (called OVN north/south traffic), the OVN network connects to an uplink network via a virtual router on a designated NIC (`eth0` in the figure above). The virtual router is active on only one cluster member at a time but can migrate between cluster members as needed. This ensures uplink connectivity in case the member with the active router becomes unavailable.

An instance (container or virtual machine) within a cluster member connects to the OVN network via its virtual NIC. Through the OVN network, it can communicate with the uplink network.

The strengths of using OVN become apparent when considering a networking architecture with more than one OVN network:

```{figure} /images/ovn_networking_2.svg
:width: 100%

OVN networking (two networks)
```

Both OVN networks depicted are completely independent. Both networks are available on all cluster members (with each virtual router active on one random cluster member). Any instance can use either of the networks, and the traffic on one network is completely isolated from the other network.

See the {ref}`LXD documentation on OVN networks <lxd:network-ovn>` for more information.

(exp-networking-ovn-underlay)=
### Dedicated underlay network

During {ref}`MicroCloud initialization <howto-initialize>`, you can choose to {ref}`configure a dedicated underlay network for OVN traffic <howto-ovn-underlay>`. This requires an additional network interface on each cluster member.

A dedicated underlay network serves as the physical infrastructure over which the virtual (overlay) network is constructed. While optional, it offers several potential benefits:

- **Traffic isolation**: Keeps overlay traffic separate from management and other traffic.
- **Reduced congestion**: Dedicating physical resources minimizes network bottlenecks.
- **Optimized performance**: Enables predictable latency and bandwidth for sensitive applications.
- **Scalable design**: Allows the overlay network to scale independently of other networks.

### Alternatives

If you decide to not use MicroOVN, MicroCloud falls back on the [Ubuntu fan](https://wiki.ubuntu.com/FanNetworking) for basic networking. MicroCloud will still be usable, but you will see some limitations, including:

- When you move an instance from one cluster member to another, its IP address changes.
- Egress traffic leaves from the local cluster member (while OVN provides shared egress).
  As a result of this, network forwarding works at a basic level only, and external addresses must be forwarded to a specific cluster member and don't fail over.
- There is no support for hardware acceleration, load balancers, or ACL functionality within the local network.

(exp-networking-vpcs)=
## Comparison to public cloud VPCs

For users familiar with public cloud networking, understanding the similarities and differences between MicroCloud and Virtual Private Clouds (VPCs) can provide helpful context.

### Similarities

- **Private networking**: Like a VPC, MicroCloud provides a private, isolated network that allows instances to communicate privately while controlling external access.

- **Subnet-like structure**: MicroCloud uses OVN to define logical networks, similar to how VPCs use subnets.

- **Routing and NAT**: Both MicroCloud and VPCs enable internal routing between instances and support Network Address Translation (NAT) for outbound internet access.

### Differences

- **Built for on-premises or self-hosted use**: MicroCloud is designed for on-premises or self-hosted environments, whereas VPCs are tightly integrated into managed cloud ecosystems.

- **Self-contained networking environment**: Unlike public cloud VPCs, which operate within a provider’s infrastructure and managed networking components, MicroCloud runs on user-managed hardware and environments. Its networking is entirely self-contained, and users can configure external connectivity as needed.

- **Use of OVN instead of vendor-specific SDN solutions**: Instead of relying on vendor-specific networking solutions, MicroCloud uses OVN to manage internal communication and overlays.
