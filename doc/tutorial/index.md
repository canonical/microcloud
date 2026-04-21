---
myst:
  html_meta:
    description: An index to hands-on tutorials to install, initialize, and try out test deployments of MicroCloud, on a single physical machine or multiple virtual machines.
---

(get-started)=
(tutorials)=
(tutorial)=
# Tutorials

These tutorials provide you with an introduction to MicroCloud concepts and
usage.

A production MicroCloud should use at least three physical machines as cluster
members. However, you may only have a single physical machine available when
first learning about MicroCloud. Our tutorials only require a single physical
machine to complete, but keep in mind that these tutorials are intended for
learning purposes only.

## Get started with MicroCloud

- {ref}`Set up a multi-member MicroCloud with virtual machines <tutorial-multi>`

In this tutorial, you will create multiple LXD virtual machines (VMs) on a
single physical host machine and use those VMs as cluster members.

Follow this tutorial to learn about MicroCloud concepts, cluster initialization,
and the MicroCloud UI.

- Hardware requirements: A single machine that supports LXD
- Prerequisites: Basic understanding of a CLI

```{admonition} In production environments
   :class: note
   The use of virtual machines in this tutorial is intended for learning
   purposes only. In production environments, only physical machines should be
   used as MicroCloud cluster members.
```

## Advanced tutorial

- {ref}`Set up a single-member MicroCloud with a physical machine
  <tutorial-single>`

In this tutorial, you will install and initialize MicroCloud on a single
physical machine and access the MicroCloud UI.

Follow this tutorial to learn more about the MicroCloud production environment.
Certain cluster features are not available in a single-member setup, but you
will still learn other important MicroCloud concepts. After you complete this
tutorial, you can join other physical machines to the cluster to create a
multi-member setup.

- Hardware requirements: A single machine with two additional physical storage
  disks and two network interfaces
- Prerequisites: Knowledge of Linux system administration, including the
  configuration of network interfaces

```{admonition} In production environments
   :class: note
   The use of a single physical machine in this tutorial is intended for
   learning purposes only. In production environments, at least three physical
   machines should be used as cluster members.
```

```{toctree}
:hidden:
:maxdepth: 2

Get started with MicroCloud </tutorial/multi-member>
Advanced tutorial </tutorial/single-member>
```
