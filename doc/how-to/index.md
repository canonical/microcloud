---
myst:
  html_meta:
    description: How-to guides for key MicroCloud operations, including installation, initialization, UI access, configuration, cluster management, updates, and more.
---

(howto)=
# How-to guides

These MicroCloud how-to guides cover key operations and processes.


## Get started

Follow these guides to install MicroCloud in a testing or production
environment, and learn how to initialize MicroCloud through interactive or
automated configuration processes. You can also set up access to the MicroCloud
UI.

```{toctree}
:maxdepth: 1

Install MicroCloud </how-to/install>
Initialize MicroCloud </how-to/initialize>
Access the UI </how-to/ui>
```

## Configure services

You can configure storage with MicroCeph and networking with MicroOVN during the
initialization process, or you can add a service later.

```{toctree}
:maxdepth: 1

Configure Ceph networking </how-to/ceph_networking>
Configure OVN underlay </how-to/ovn_underlay>
Add a service </how-to/add_service>
```

## Manage clusters and cluster members

As your needs change, follow these steps to manage your clusters and cluster
members and keep your deployment up to date.


```{toctree}
:maxdepth: 1

Add a cluster member </how-to/member_add>
Remove a cluster member </how-to/member_remove>
Shut down a cluster member </how-to/member_shutdown>
Manage multiple clusters </how-to/cluster_manager>
Recover MicroCloud </how-to/recover>
Update and upgrade </how-to/update_upgrade>
Manage the snaps </how-to/snaps>
```

## Automated deployment with Terraform

Follow this guide to learn how to use Terraform to automate the deployment of a
MicroCloud test environment. As in our {ref}`introductory tutorial
<tutorial-multi>`, this test environment uses Virtual Machines (VMs) as
MicroCloud cluster members.

```{toctree}
:maxdepth: 1

Automate a test deployment with Terraform </how-to/terraform_automation>
```

## Engage with us

Find out how to get community and commercial support, and learn how to make
contributions to MicroCloud.

```{toctree}
:maxdepth: 1

Get support </how-to/support>
Contribute to MicroCloud </how-to/contribute>
```
