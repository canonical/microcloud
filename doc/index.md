---
discourse: lxc:[Introducing&#32;MicroCloud](15871)
relatedlinks: "[Install&#32;MicroCloud&#32;on&#32;Linux&#32;|&#32;Snap&#32;Store](https://snapcraft.io/microcloud)"
---

(home)=
# MicroCloud

Deploy a scalable, low-touch cloud platform in minutes with MicroCloud.

MicroCloud creates a lightweight cluster of machines that operates as an open source private cloud. It combines LXD for virtualization, MicroCeph for distributed storage, and MicroOVN for networking—all automatically configured by the [MicroCloud snap](https://snapcraft.io/microcloud) for reproducible, reliable deployments.

With MicroCloud, you can eliminate the complexity of manual setup and quickly benefit from high availability, automatic security updates, and the advanced features of its components such as self-healing clusters and fine-grained access control. Cluster members can run full virtual machines or lightweight system containers with bare-metal performance.

MicroCloud is designed for small-scale private clouds and hybrid cloud extensions. Its efficiency and simplicity also make it an excellent choice for edge computing, test labs, and other resource-constrained use cases.

```{figure} /images/microcloud_basic_architecture.svg
:alt: A diagram of basic MicroCloud setup architecture
:align: center
:width: 75%
```

---

## In the MicroCloud documentation

````{grid} 1 1 2 2

```{grid-item} [Tutorial](/tutorial/get_started)

**Start here**: a hands-on [introduction to MicroCloud](/tutorial/get_started) for new users
```

```{grid-item} [How-to guides](/how-to/index)

**Step-by-step guides** covering key operations and common tasks such as {ref}`installing MicroCloud <howto-install>` and {ref}`adding <howto-member-add>` and {ref}`removing <howto-member-remove>` cluster members
```

````

````{grid} 1 1 2 2
:reverse:

```{grid-item} [Reference](/reference/index)

**Technical information** - Detailed [requirements](/reference/requirements)
```

```{grid-item} [Explanation](/explanation/index)

**Discussion and clarification** of key topics such as {ref}`networking <explanation-networking>` and the [initialization process](/explanation/initialization/)
```

````

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

MicroCloud is a member of the Ubuntu family. It’s an open source project that warmly welcomes community projects, contributions, suggestions, fixes and constructive feedback.

- [MicroCloud snap](https://snapcraft.io/microcloud)
- [Contribute](https://github.com/canonical/microcloud)
- [Get support](https://discourse.ubuntu.com/c/lxd/microcloud/145)
- [Thinking about using MicroCloud for your next project? Get in touch!](https://canonical.com/microcloud)


```{toctree}
:hidden:
:maxdepth: 2

self
Tutorial </tutorial/get_started>
/how-to/index
/reference/index
/explanation/index
