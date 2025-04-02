# Data sources
data "lxd_info" "cluster" {
  remote = var.remote
}

# Project
resource "lxd_project" "e2e" {
  name   = "e2e-testing"
  remote = var.remote
}

# Profile
resource "lxd_profile" "e2e" {
  name    = "e2e-testing"
  project = lxd_project.e2e.name
  remote  = lxd_project.e2e.remote

  # Configuration
  config = {
    "limits.cpu"    = 1
    "limits.memory" = "1GiB"
  }

  # Devices
  device {
    name = "root"
    type = "disk"
    properties = {
      pool = "remote"
      path = "/"
      size = "4GiB"
    }
  }

  device {
    name = "eth0"
    type = "nic"
    properties = {
      network = "default"
    }
  }
}

# Images
resource "lxd_cached_image" "ctn" {
  count         = var.containers_per_host > 0 ? 1 : 0
  project       = lxd_project.e2e.name
  remote        = lxd_project.e2e.remote
  source_remote = "ubuntu-minimal-daily"
  source_image  = "24.04"
  type          = "container"
}

resource "lxd_cached_image" "vm" {
  count         = var.vms_per_host > 0 ? 1 : 0
  project       = lxd_project.e2e.name
  remote        = lxd_project.e2e.remote
  source_remote = "ubuntu-minimal-daily"
  source_image  = "24.04"
  type          = "virtual-machine"
}

# Containers
resource "lxd_instance" "e2e-ctn" {
  for_each = {
    for _, v in local.containers : v.instance => v.target
  }

  name             = each.key
  target           = each.value
  type             = "container"
  remote           = lxd_project.e2e.remote
  project          = lxd_project.e2e.name
  profiles         = [lxd_profile.e2e.name]
  image            = lxd_cached_image.ctn[0].fingerprint
  wait_for_network = true
}

# VMs
resource "lxd_instance" "e2e-vm" {
  for_each = {
    for _, v in local.vms : v.instance => v.target
  }

  name             = each.key
  target           = each.value
  type             = "virtual-machine"
  remote           = lxd_project.e2e.remote
  project          = lxd_project.e2e.name
  profiles         = [lxd_profile.e2e.name]
  image            = lxd_cached_image.vm[0].fingerprint
  wait_for_network = true
}

# Locals
locals {
  cluster_member_names = [for k, _ in data.lxd_info.cluster.cluster_members : k]

  containers = flatten([
    for index, cluster_member_name in local.cluster_member_names : [
      for i in range(var.containers_per_host) : {
        instance = "c${format("%02d", var.containers_per_host * index + i + 1)}"
        target   = cluster_member_name
      }
    ]
  ])
  vms = flatten([
    for index, cluster_member_name in local.cluster_member_names : [
      for i in range(var.vms_per_host) : {
        instance = "vm${format("%02d", var.vms_per_host * index + i + 1)}"
        target   = cluster_member_name
      }
    ]
  ])
}

# Outputs
output "instances" {
  value = {
    "ctns" = local.containers
    "vms"  = local.vms
  }
}
