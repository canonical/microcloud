(howto-terraform-automation)=
# How to automate a MicroCloud test deployment with Terraform

This guide shows you how to automatically deploy a MicroCloud test environment using Terraform and the LXD provider. The Terraform configuration replicates the same 4-VM setup described in the {ref}`get-started` tutorial, automating all the manual steps including VM creation, disk provisioning, network configuration, and MicroCloud initialization.

For production deployments, consider using physical machines instead of VMs and review the {ref}`reference-requirements` for proper hardware specifications.

```{note}
This Terraform configuration creates the same infrastructure as the manual {ref}`get-started` tutorial. If you have already followed that tutorial, you may encounter naming conflicts with existing resources (VMs, networks, storage volumes). Either clean up the existing resources first or modify the variable names in the Terraform configuration.
```

## Prerequisites

Before using this Terraform configuration, ensure you have:

- **Terraform installed** (version 1.0 or later)
- **LXD installed and initialized** on your host machine (see {ref}`get-started` step 1)
- **Sufficient storage space** as described in the {ref}`get-started` tutorial (step 2)
- **Network connectivity** for downloading Ubuntu images and snaps
- **Nested virtualization enabled** as mentioned in the {ref}`get-started` tutorial introduction

The host LXD should have:
- A storage pool that meets the requirements described in {ref}`get-started` step 2
- A bridge network for VM connectivity (configured during LXD initialization in {ref}`get-started` step 1)

## Configuration overview

The Terraform configuration automates the entire MicroCloud setup process:

1. **Infrastructure provisioning**: Creates 4 Ubuntu VMs with proper resource allocation
2. **Storage setup**: Provisions local and Ceph storage disks for each VM
3. **Network configuration**: Sets up lookup and uplink network interfaces
4. **Service installation**: Installs MicroCloud, LXD, MicroCeph, and MicroOVN snaps via cloud-init
5. **Cluster initialization**: Uses preseed configuration to automatically initialize the MicroCloud cluster

The resulting setup matches the {ref}`get-started` tutorial:
- **4 VMs**: `micro1` (initiator), `micro2`, `micro3`, `micro4`
- **Storage**: Local storage on all VMs, Ceph storage on first 3 VMs
- **Networking**: OVN distributed networking with uplink connectivity
- **High availability**: 3-node Ceph cluster for distributed storage

## Setup steps

1. **Navigate to the Terraform configuration**:
   ```bash
   cd demos/terraform/
   ```

2. **Review and customize variables** (optional):
   ```bash
   cp terraform.tfvars.example terraform.tfvars
   # Edit terraform.tfvars to customize VM names, IP addresses, etc.
   ```

3. **Initialize Terraform**:
   ```bash
   terraform init
   ```

4. **Review the planned changes**:
   ```bash
   terraform plan
   ```

5. **Deploy the infrastructure**:
   ```bash
   terraform apply
   ```

6. **Check the deployment**:
   ```bash
   # Check cluster status
   terraform output microcloud_status
   
   # Get VM IP addresses
   terraform output vm_ips
   
   # Access a cluster member
   lxc shell micro1
   ```

## Configuration

### Common configuration options

**Change VM names and IP addresses**:
```hcl
# In terraform.tfvars
vm_names = ["node1", "node2", "node3", "node4"]
ip_base_offset = 50  # IPs will be .50, .60, .70, .80
```

**Modify resource allocation**:
```hcl
cpu_per_instance = 4
memory_per_instance = 4  # GiB
```

**Adjust storage configuration**:
```hcl
local_disk_size = "20GiB"
ceph_disk_size = "30GiB"
```

**Use different snap channels**:
```hcl
microcloud_channel = "latest/edge"
lxd_channel = "latest/edge"
```

### Network configuration

The configuration uses your existing LXD bridge network. To use a different network:

```hcl
lookup_bridge = "mybr0"  # Your custom bridge name
```

## Cleanup

To remove the entire deployment:
```bash
terraform destroy
```

This will delete all VMs, storage volumes, and networks created by Terraform.
