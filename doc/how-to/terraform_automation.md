(howto-terraform-automation)=
# How to automate a MicroCloud test deployment with Terraform

This guide shows you how to automatically deploy a MicroCloud test environment using Terraform and the LXD provider. The Terraform configuration replicates the same setup as in our {ref}`multi-member tutorial <tutorial-multi>`, which uses four VMs on a single physical machine for the MicroCloud cluster members. It automates all the manual steps from that tutorial, including VM creation, disk provisioning, network configuration, and MicroCloud initialization.

For production deployments, use physical machines instead of VMs and review the {ref}`reference-requirements` for proper hardware specifications.

<!-- Include start rename-terraform-resources-note -->
```{note}
This Terraform configuration creates the same infrastructure as the manual {ref}`multi-member tutorial <tutorial-multi>`. If you have already followed that tutorial, you may encounter naming conflicts with existing resources (VMs, networks, storage volumes). Either clean up the existing resources first or modify the variable names in the Terraform configuration.
```
<!-- Include end rename-terraform-resources-note -->

## Requirements

Before using this Terraform configuration, ensure you have:

- **Terraform installed** (version 1.0 or later)
- **LXD installed and initialized** on your host machine (see the {ref}`multi-member tutorial <tutorial-multi>` step 1)
- **Sufficient storage space** as described in the {ref}`multi-member tutorial <tutorial-multi>` (step 2)
- **Network connectivity** for downloading Ubuntu images and snaps
- **Nested virtualization enabled** as mentioned in the {ref}`multi-member tutorial <tutorial-multi>` introduction

The host LXD should have:
- A storage pool that meets the requirements described in the {ref}`multi-member tutorial <tutorial-multi>` step 2
- A bridge network for VM connectivity (configured during LXD initialization in the {ref}`multi-member tutorial <tutorial-multi>` step 1)

## Configuration overview

The Terraform configuration automates the entire MicroCloud setup process:

- **Infrastructure provisioning**: Creates 4 Ubuntu VMs with proper resource allocation
- **Storage setup**: Provisions local and Ceph storage disks for each VM
- **Network configuration**: Sets up lookup and uplink network interfaces
- **Service installation**: Installs MicroCloud, LXD, MicroCeph, and MicroOVN snaps via cloud-init
- **Cluster initialization**: Uses preseed configuration to automatically initialize the MicroCloud cluster

The resulting setup matches the {ref}`multi-member tutorial <tutorial-multi>`:
- **4 VMs**: `micro1` (initiator), `micro2`, `micro3`, `micro4`
- **Storage**: Local storage on all VMs, Ceph storage on first 3 VMs
- **Networking**: OVN distributed networking with uplink connectivity
- **High availability**: 3-node Ceph cluster for distributed storage

## Setup steps

1. Navigate to the Terraform configuration:
   ```bash
   cd demos/terraform/
   ```

1. Review and customize variables (optional):
   Edit `terraform.tfvars` to customize the VM names, IP addresses, and other variables.
   ```bash
   cp terraform.tfvars.example terraform.tfvars
   ```

   ```{include} terraform_automation.md
      :start-after: <!-- Include start rename-terraform-resources-note -->
      :end-before: <!-- Include end rename-terraform-resources-note -->
   ```

1. Initialize Terraform:
   ```bash
   terraform init
   ```

1. Review the planned changes:
   ```bash
   terraform plan
   ```

1. Deploy the infrastructure:
   ```bash
   terraform apply
   ```

1. Verify that the deployment is functioning:
   Check cluster status:
   ```bash
   terraform output microcloud_status
   ```

   Get the VMs IP addresses:
   ```bash
   terraform output vm_ips
   ```

   Confirm that you can access a cluster member:
   ```bash
   lxc shell <cluster member name> 
   ```

## Configuration

To change these options, edit `terraform.tfvars`. The option values provided below are examples, and you must update them to suit your setup.

Change VMs names and IP addresses:
```hcl
vm_names = ["node1", "node2", "node3", "node4"]
ip_base_offset = 50  # IPs will be .50, .60, .70, .80
```

Modify resource allocation:
```hcl
cpu_per_instance = 4
memory_per_instance = 4  # GiB
```

Adjust storage configuration:
```hcl
local_disk_size = "20GiB"
ceph_disk_size = "30GiB"
```

Use different snap channels:
```hcl
microcloud_channel = "latest/edge"
lxd_channel = "latest/edge"
```

### Network configuration

The configuration uses your existing LXD bridge network. To use a different network, modify the `lookup_bridge` option. Example:
```hcl
lookup_bridge = "mybr0"  
```

## Cleanup

To remove the entire deployment, run:
```bash
terraform destroy
```

This will delete all VMs, storage volumes, and networks created by Terraform.
