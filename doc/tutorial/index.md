(get-started)=
(tutorials)=
(tutorial)=
# Get started with MicroCloud

MicroCloud is quick to set up. Once initialized, you can start using MicroCloud in the same way as a regular LXD cluster. The tutorials in this section provide you with an introduction to MicroCloud concepts and use, including its installation and initialization, and use of its graphical UI.

A production MicroCloud should use at least three physical machines as cluster members. However, when first learning about MicroCloud, it's common to only have a single machine available. Thus, we offer two approaches for learning about MicroCloud that both use one physical machine:

{ref}`Set up a single-member MicroCloud with a physical machine <tutorial-single>`
: This tutorial helps you understand the MicroCloud production environment. While certain cluster features are not available in a single-member setup (such as communication between cluster members), you will learn about other important MicroCloud concepts. After you complete this tutorial, you can join other physical machines to the cluster to create a multi-member setup.

  This approach requires a higher level of Linux system administration knowledge on your part, such as how to configure your network interfaces. It also requires at least two additional physical storage disks and two network interfaces.

{ref}`Set up a multi-member MicroCloud with virtual machines <tutorial-multi>`
: This tutorial uses a sandbox approach. It shows you how to create multiple LXD virtual machines (VMs) on a single physical host machine and use those VMs as MicroCloud cluster members. 

  Since this approach guides you in building and configuring a virtual environment step by step, you do not need system administration knowledge. You'll be able to create a multi-member cluster without requiring multiple physical machines, disks, and network interfaces. However, keep in mind that this approach is intended for learning purposes only. In production environments, only physical machines should be used for MicroCloud cluster members.

```{toctree}
:hidden:
:maxdepth: 2

Single physical machine as a cluster member </tutorial/single-member>
Multiple virtual machines as cluster members </tutorial/multi-member>
