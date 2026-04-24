---
myst:
  html_meta:
    description: An index of how-to guides for key MicroCloud operations, including installation, initialization, UI access, configuration, management, and updates.
---

(howto)=
# How-to guides

These MicroCloud how-to guides cover key operations and processes.

## Set up and deploy

MicroCloud can be installed in testing or production environments and
initialized interactively or through an automated process. Once deployed, the
MicroCloud UI provides an interface alongside the {ref}`MicroCloud CLI
<ref-commands>`.

```{toctree}
:maxdepth: 1

Install MicroCloud </how-to/install>
Initialize MicroCloud </how-to/initialize>
Access the UI </how-to/ui>
```

Terraform makes it possible to automate the MicroCloud deployment process.

```{toctree}
:maxdepth: 1

Deploy a MicroCloud test environment with Terraform </how-to/terraform_automation>
```

## Configure services

MicroCeph and MicroOVN make it possible to configure storage and networking to
meet your needs. Configure these services during a MicroCloud initialization, or
add a service later.

```{toctree}
:maxdepth: 1

Configure Ceph networking </how-to/ceph_networking>
Configure OVN underlay </how-to/ovn_underlay>
Add a service </how-to/add_service>
```

## Manage clusters and cluster members

As your needs change, manage clusters and cluster members to keep your
deployment up to date.

```{toctree}
:maxdepth: 1

Manage cluster members </how-to/members_manage>
Manage multiple clusters </how-to/cluster_manager>
Recover MicroCloud </how-to/recover>
Update and upgrade </how-to/update_upgrade>
Manage the snaps </how-to/snaps>
```

## Engage with us

```{toctree}
:maxdepth: 1

Get support </how-to/support>
Contribute to MicroCloud </how-to/contribute>
```
