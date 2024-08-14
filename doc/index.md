---
discourse: lxc:15871
relatedlinks: https://snapcraft.io/microcloud
---

(home)=
# MicroCloud

MicroCloud is a collection of services that allows you to deploy your own fully functional cloud in minutes.
The MicroCloud snap automatically configures the different components across a set of servers, making it possible to set up a complete cluster by running a single command on one of the machines.

Once installed, MicroCloud uses LXD for virtualisation, Ceph for distributed storage, and OVN for networking.

This way, MicroCloud creates a small footprint cluster of compute nodes with distributed storage and secure networking, optimised for repeatable, reliable remote deployments.

MicroCloud is aimed at edge computing, and anyone in need of a small-scale private cloud.

---

## How to use this documentation

Since MicroCloud is a collection of services, this documentation consists of four different documentation sets.
````{only} integrated
You can navigate between these documentation sets by using the links in the top bar.
````

{doc}`index`
: The MicroCloud documentation contains information for getting started with MicroCloud, in addition to conceptual and architectural documentation.
  This documentation describes how the different components are used within a MicroCloud setup.

{doc}`lxd:index`
: LXD is the system container and virtual machine manager used for virtualisation in MicroCloud.
  This means that after you install MicroCloud, you will manage your instances through LXD and the LXD UI.

{doc}`microceph:index`
: MicroCeph provides a lightweight way of deploying and managing a [Ceph](https://ceph.io/en/) cluster.
  MicroCloud uses MicroCeph to set up distributed Ceph storage.

{doc}`microovn:index`
: MicroOVN is a snap-based distribution of [OVN](https://www.ovn.org/).
  MicroCloud uses MicroOVN to set up OVN networking.

```{note}
The MicroCloud documentation set is targeted specifically at users of MicroCloud.

The other three documentation set describe the full functionality of each component.
This functionality is available as part of your MicroCloud setup, but not all of it is relevant.
For example, all documentation sets contain installation information, but the components are already installed as part of MicroCloud.
Also, while each component documents how to remove cluster members, you should not remove machines from only one component.
Use MicroCloud to remove cluster members (see {ref}`howto-remove`).
```

---

## In the MicroCloud documentation

````{grid} 1 1 2 2

```{grid-item} [Tutorial](/tutorial/get_started)

**Start here**: a hands-on introduction to MicroCloud for new users
```

```{grid-item} [How-to guides](/how-to/index)

**Step-by-step guides** covering key operations and common tasks
```

````

````{grid} 1 1 2 2
:reverse:

```{grid-item} [Reference](/reference/index)

**Technical information** - specifications, APIs, architecture
```

```{grid-item} [Explanation](/explanation/index)

**Discussion and clarification** of key topics
```

````

---

## Project and community

MicroCloud is a member of the Ubuntu family. It’s an open source project that warmly welcomes community projects, contributions, suggestions, fixes and constructive feedback.

- [MicroCloud snap](https://snapcraft.io/microcloud)
- [Contribute](https://github.com/canonical/microcloud)
- [Get support](https://discourse.ubuntu.com/c/lxd/microcloud/)
- [Thinking about using MicroCloud for your next project? Get in touch!](https://canonical.com/microcloud)


```{toctree}
:hidden:
:maxdepth: 2

self
Tutorial </tutorial/get_started>
/how-to/index
/reference/index
/explanation/index
