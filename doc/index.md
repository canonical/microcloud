---
myst:
  html_meta:
    description: MicroCloud is a lightweight, open source cloud platform built on LXD, MicroCeph, and MicroOVN, ideal for private cloud, edge computing, and test labs.
discourse: lxc:[Introducing&#32;MicroCloud](15871)
relatedlinks: "[Install&#32;MicroCloud&#32;on&#32;Linux&#32;|&#32;Snap&#32;Store](https://snapcraft.io/microcloud)"
---

(home)=
# MicroCloud

Deploy a low-touch, open source cloud platform in minutes with MicroCloud.

MicroCloud creates a lightweight cluster of machines that operates as a scalable private cloud. It combines LXD for virtualization, MicroCeph for distributed storage, and MicroOVN for networking—all automatically configured by the [MicroCloud snap](https://snapcraft.io/microcloud) for {ref}`reproducible, scalable deployments <exp-microcloud-scale>`.

With MicroCloud, you can eliminate the complexity of manual setup and quickly benefit from {ref}`high availability <exp-microcloud-ha>`, {ref}`streamlined security updates <ref-releases-snaps>`, and {ref}`fine-grained access control for multi-tenancy <exp-microcloud-access-control>`. Cluster members can run {ref}`full virtual machines or lightweight system containers <exp-microcloud-vms-containers>` with bare-metal performance. Manage it through your choice of client interfaces, including a {ref}`graphical UI <howto-ui>` and {ref}`CLI <ref-commands>`.

MicroCloud is designed for small-scale private clouds and hybrid cloud extensions. Its efficiency and simplicity also make it an excellent choice for edge computing, test labs, and other resource-constrained use cases.

```{figure} /images/microcloud_basic_architecture.svg
:alt: A diagram of basic MicroCloud setup architecture
:align: center
:width: 75%
```

---

## In this documentation

| | |
|---|---|
| Start here | {ref}`Tutorial using multiple virtualized cluster members <tutorial-multi>` • {ref}`Tutorial using a single physical cluster member <tutorial-single>` • {ref}`MicroCloud overview <exp-microcloud>` |
| Storage and networks | {ref}`Understand local vs. distributed storage <exp-storage>` • {ref}`Understand MicroCloud's networking approach <exp-networking>` • {ref}`Configure an OVN underlay network <howto-ovn-underlay>` • {ref}`Configure a dedicated Ceph network <howto-ceph-networking>` • {ref}`Add a service <howto-add-service>` |
| Cluster management | {ref}`Add <howto-member-add>`, {ref}`remove <howto-member-remove>`, and {ref}`shut down <howto-member-shutdown>` cluster members • {ref}`Access the web UI <howto-ui>` • {ref}`Common CLI commands reference <ref-commands>` |
| Setup and maintenance | {ref}`Installation <howto-install>` • {ref}`Initialization <howto-initialize>` • {ref}`Automate initialization with Terraform <howto-terraform-automation>` • {ref}`Update and upgrade <howto-update-upgrade>` • {ref}`Recover a cluster <howto-recover>` • {ref}`security` |
| Releases and requirements | {ref}`Supported and compatible releases <ref-releases-matrix>` •  {ref}`Snaps and releases reference <ref-releases-snaps>` • {ref}`ref-release-notes` • {ref}`Setup requirements <reference-requirements>` |

---

## About the integrated documentation sets

The three components of MicroCloud ({doc}`lxd:index`, {doc}`microceph:index`, and {doc}`microovn:index`) each offer their own documentation sets, available at their respective standalone documentation sites.

For convenience, this site provides not only MicroCloud's documentation but also an integrated view of all four documentation sets. You can easily switch between sets using the links in the site header, allowing you to explore all the related documentation without leaving this site.

```{note}
The components' documentation sets are written for a general audience that might not be using MicroCloud. Thus, not all the information in these sets are relevant to MicroCloud users. For example, since MicroCloud automates the installation of its components, you can ignore the manual installation instructions in the components' documentation.

Also, while each component's documentation includes instructions for removing cluster members, you should not remove members from only one component. Use MicroCloud instead to remove cluster members (see {ref}`howto-member-remove`).
```

---

## Project and community

MicroCloud is a member of the [Canonical](https://canonical.com) family. It’s an open source project that warmly welcomes community contributions, suggestions, fixes, and constructive feedback.

### Get involved

- {ref}`Support <howto-support>`
- [Discussion forum](https://discourse.ubuntu.com/c/lxd/microcloud/145)
- {ref}`Contribute <howto-contribute>`

### Releases

- {ref}`ref-release-notes`

### Governance and policies

- [Code of conduct](https://ubuntu.com/community/docs/ethos/code-of-conduct)

### Commercial support

Thinking about using MicroCloud for your next project? [Get in touch](https://canonical.com/microcloud/contact-us)!




```{toctree}
:hidden:
:maxdepth: 2

self
Tutorials </tutorial/index>
/how-to/index
/reference/index
/explanation/index
