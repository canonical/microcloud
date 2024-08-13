---
discourse: lxc:15871
relatedlinks: https://snapcraft.io/microcloud
---

(home)=
# MicroCloud

MicroCloud allows you to deploy your own fully functional cloud in minutes.

The MicroCloud snap automatically configures [LXD](https://canonical.com/lxd), [Ceph](https://ceph.io/en/), and [OVN](https://www.ovn.org/) across a set of servers.
MicroCloud relies on mDNS to automatically detect other servers on the network, making it possible to set up a complete cluster by running a single command on one of the machines.

This way, MicroCloud creates a small footprint cluster of compute nodes with distributed storage and secure networking, optimised for repeatable, reliable remote deployments.

MicroCloud is aimed at edge computing, and anyone in need of a small-scale private cloud.

Link {doc}`lxd:index`, {doc}`microceph:index`, {doc}`microovn:index`.

---

## In this documentation

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
