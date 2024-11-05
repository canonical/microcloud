<p align="left">
    <img alt="MicroCloud logo" width="10%" src="doc/images/microcloud_logo_dark.svg#gh-dark-mode-only">
    <img alt="MicroCloud logo" width="10%" src="doc/images/microcloud_logo_light.svg#gh-light-mode-only">
</p>

# **MicroCloud**

**MicroCloud** MicroCloud allows you to deploy your own fully functional cloud in minutes.

It’s a snap package that can automatically configure LXD, Ceph, and OVN across a set of servers.
It can leverage multicast to automatically detect other servers on the network, making it possible to set up a complete cluster by running a single command on each of the machines.

MicroCloud creates a small footprint cluster of compute nodes with distributed storage and secure networking, optimized for repeatable, reliable remote deployments. MicroCloud is aimed at edge computing, and anyone in need of a small-scale private cloud.

## **Requirements?**

MicroCloud requires a minimum of three machines.
It supports up to 50 machines.

To use local storage, each machine requires a local disk.
To use distributed storage, at least three additional disks (not only partitions) for use by Ceph are required, and these disks must be on at least three different machines.

Once the simple initialisation is complete, users can launch, run and manage their workloads using system containers or VMs, and otherwise utilise regular LXD functionality.

## **How to get started**

To get started, install the LXD, MicroCeph, MicroOVN and MicroCloud snaps. You can install them all at once with the following command:

```sh
snap install lxd microceph microovn microcloud
```

Then start the bootstrapping process with the following command:

```sh
microcloud init
```

In case you want to setup a multi machine MicroCloud, run the following command on all the other machines:

```sh
microcloud join
```

Following the simple CLI prompts, a working MicroCloud will be ready within minutes.

<!-- include start about -->

The MicroCloud snap drives three other snaps ([LXD](https://canonical-microcloud.readthedocs-hosted.com/en/latest/lxd/), [MicroCeph](https://canonical-microcloud.readthedocs-hosted.com/en/latest/microceph/), and [MicroOVN](https://canonical-microcloud.readthedocs-hosted.com/en/latest/microovn/)), enabling automated deployment of a highly available LXD cluster for compute, with Ceph as the storage driver and OVN as the managed network.

During initialisation, MicroCloud scrapes the other servers for details and then prompts you to add disks to Ceph and configure the networking setup.

At the end of this, you’ll have an OVN cluster, a Ceph cluster, and a LXD cluster. LXD itself will have been configured with both networking and storage suitable for use in a cluster.

<!-- include end about -->

## **What about networking?**

By default, MicroCloud uses MicroOVN for networking, which is a minimal wrapper around OVN (Open Virtual Network).
If you decide to not use MicroOVN, MicroCloud falls back on the Ubuntu fan for basic networking.

You can optionally add the following dedicated networks:
  - a network for Ceph management traffic (also called public traffic)
  - a network for internal traffic (also called cluster traffic)
  - a network for OVN underlay traffic

## **What's next?**

This is just the beginning of MicroCloud. We’re very excited about what’s coming up next!

### **RESOURCES:**

- Documentation: https://canonical-microcloud.readthedocs-hosted.com/
- Find the package at the Snap Store:

 [![Snapcraft logo](https://dashboard.snapcraft.io/site_media/appmedia/2018/04/Snapcraft-logo-bird.png)](https://snapcraft.io/microcloud)

- Snap package sources: [microcloud-pkg-snap](https://github.com/canonical/microcloud-pkg-snap)
