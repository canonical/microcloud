# **MicroCloud**

**MicroCloud** MicroCloud allows you to deploy your own fully functional cloud in minutes.

It’s a snap package that can automatically configure LXD, Ceph, and OVN across a set of servers.
It relies on mDNS to automatically detect other servers on the network, making it possible to set up a complete cluster by running a single command on one of the machines.

MicroCloud creates a small footprint cluster of compute nodes with distributed storage and secure networking, optimized for repeatable, reliable remote deployments. MicroCloud is aimed at edge computing, and anyone in need of a small-scale private cloud.

## **Requirements?**

MicroCloud requires a minimum of three machines.
It supports up to 50 machines.

To use local storage, each machine requires a local disk.
To use distributed storage, at least three additional disks (not only partitions) for use by Ceph are required, and these disks must be on at least three different machines.

<p align="center" width="100%">
    <img alt="MicroCloud logo" width="33%" src="https://res.cloudinary.com/canonical/image/fetch/f_auto,q_auto,fl_sanitize,w_236,h_214/https://assets.ubuntu.com/v1/904e5156-LXD+illustration+2.svg">
</p>

Once the simple initialisation is complete, users can launch, run and manage their workloads using system containers or VMs, and otherwise utilise regular LXD functionality.

## **How to get started**

To get started, install the LXD, MicroCeph, MicroOVN and MicroCloud snaps. You can install them all at once with the following command:

```
snap install lxd microceph microovn microcloud
```

Then start the bootstrapping process with the following command:

```
microcloud init
```

Following the simple CLI prompts, a working MicroCloud will be ready within minutes.

The MicroCloud snap drives three other snaps ([LXD](https://documentation.ubuntu.com/lxd), [MicroCeph](https://canonical-microceph.readthedocs-hosted.com/), and [MicroOVN](https://canonical-microovn.readthedocs-hosted.com/)), enabling automated deployment of a highly available LXD cluster for compute with Ceph as the storage driver and OVN as the managed network.

During initialisation, MicroCloud detects the other servers and then prompts you to add disks to Ceph and configure the networking setup.

At the end of this, you’ll have an OVN cluster, a Ceph cluster, and a LXD cluster. LXD itself will have been configured with both networking and storage suitable for use in a cluster.

## **What about networking?**

By default, MicroCloud uses MicroOVN for networking, which is a minimal wrapper around OVN (Open Virtual Network).
If you decide to not use MicroOVN, MicroCloud falls back on the Ubuntu fan for basic networking.

## **What's next?**

This is just the beginning of MicroCloud. We’re very excited about what’s coming up next!

### **RESOURCES:**

- Introduction: https://discuss.linuxcontainers.org/t/introducing-microcloud/15871
- Find the package at the Snap Store:

 [![Snapcraft logo](https://dashboard.snapcraft.io/site_media/appmedia/2018/04/Snapcraft-logo-bird.png)](https://snapcraft.io/microcloud)
