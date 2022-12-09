# **MicroCloud**

<!--Need to Center This Image-->
<p align="center" width="100%">
    <img width="33%" src="https://camo.githubusercontent.com/3abbd2b61fcb74805b2b48e41e2fead897322c39dab533de0231e13ac18e968b/68747470733a2f2f6c696e7578636f6e7461696e6572732e6f72672f7374617469632f696d672f636f6e7461696e6572732e706e67"> 
</p>

## **What is MicroCloud?**

**MicroCloud** is the easiest way to get a **fully highly available LXD cluster up and running.**

It’s a snap package which can automatically configure both LXD and Ceph across a set of servers.
It relies on mDNS to automatically detect other servers on the network, making it possible to setup a complete cluster by running a single command on one of the machines.

MicroCloud creates a small footprint cluster of compute nodes with distributed storage and secure networking, optimised for repeatable, reliable remote deployments. MicroCloud is aimed at edge computing, and anyone in need of a small-scale private cloud.

## **Requirements?**

A minimum of **3 systems and at least 3 additional disks** for use by Ceph, are required.

<p align="center" width="100%">
    <img width="33%" src="https://res.cloudinary.com/canonical/image/fetch/f_auto,q_auto,fl_sanitize,w_236,h_214/https://assets.ubuntu.com/v1/904e5156-LXD+illustration+2.svg">
</p>

Once the simple initialization is completed, users can launch, run and manage their workloads using system containers or VMs, and otherwise utilize regular LXD functionalities.

## **How to get Started**

To get started, LXD, MicroCeph and Micro Cloud snaps are needed. Users can install them all at once with the command:

```
snap install lxd microceph microcloud
```

The bootstrapping process starts with running

```
microcloud init
```

Following the simple CLI prompts, a working Micro cloud will be ready within minutes.

Micro Cloud snap drives two other snaps (LXD and MicroCeph) enabling automated deployment of a highly available LXD cluster for compute with Ceph as a storage backend.

After the first initialization steps MicroCloud will detect the other servers, setup a cluster and finally prompt you to add disks to Ceph.

At the end of this, you’ll have both a Ceph and LXD cluster and LXD itself will have been configured with both networking and storage suitable for use in a cluster.

## **What about Networking?**

For networking, MicroCloud will use a default network bridge. **MicroOVN** is in development and will be added once completed.

## **Whats Next?**

This is just the beginning of MicroCloud. We’re very excited about what’s coming up next, starting with the addition of OVN to the mix, providing distributed networking alongside the distributed storage provided by Ceph.

### **RESOURCES:**
 - Introduction: https://discuss.linuxcontainers.org/t/introducing-microcloud/15871
 - Find the Package at the Snap Store:
 
 [![](https://dashboard.snapcraft.io/site_media/appmedia/2018/04/Snapcraft-logo-bird.png)](https://snapcraft.io/microcloud)