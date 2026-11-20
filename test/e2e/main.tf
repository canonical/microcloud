# Data sources
data "lxd_info" "cluster" {
  remote = var.remote
}

# Check whether the project already exists
data "external" "project_exists" {
  program = ["bash", "-c", "lxc project list \"${var.remote}:\" -f csv | awk -F, '{gsub(/ \\(current\\)$/, \"\", $1); print $1}' | grep -qxF \"${var.project}\" && echo '{\"exists\": \"true\"}' || echo '{\"exists\": \"false\"}'"]
}

# Check whether the profile already exists
data "external" "profile_exists" {
  program = ["bash", "-c", "lxc profile list --project \"${var.project}\" \"${var.remote}:\" -f csv 2>/dev/null | awk -F, '{gsub(/ \\(current\\)$/, \"\", $1); print $1}' | grep -qxF \"${var.profile}\" && echo '{\"exists\": \"true\"}' || echo '{\"exists\": \"false\"}'"]
}

# Project - created only when it does not already exist
resource "lxd_project" "e2e" {
  count  = data.external.project_exists.result["exists"] == "true" ? 0 : 1
  name   = var.project
  remote = var.remote
}

# Profile - created only when it does not already exist
resource "lxd_profile" "e2e" {
  count   = data.external.profile_exists.result["exists"] == "true" ? 0 : 1
  name    = var.profile
  project = var.project
  remote  = var.remote

  depends_on = [lxd_project.e2e]

  # Configuration
  config = {
    "limits.cpu"    = 1
    "limits.memory" = "384MiB"
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
  project       = coalesce(var.image_project, var.project)
  remote        = var.remote
  source_remote = var.image_remote
  source_image  = "24.04"
  type          = "container"
  copy_aliases  = var.copy_image_aliases
  depends_on    = [lxd_project.e2e]
}

resource "lxd_cached_image" "vm" {
  count         = var.vms_per_host > 0 ? 1 : 0
  project       = coalesce(var.image_project, var.project)
  remote        = var.remote
  source_remote = var.image_remote
  source_image  = "24.04"
  type          = "virtual-machine"
  copy_aliases  = var.copy_image_aliases
  depends_on    = [lxd_project.e2e]
}

# Containers
resource "lxd_instance" "e2e-ctn" {
  for_each = {
    for _, v in local.containers : v.instance => v.target
  }

  name             = each.key
  target           = each.value
  type             = "container"
  remote           = var.remote
  project          = var.project
  profiles         = [var.profile]
  image            = lxd_cached_image.ctn[0].fingerprint
  wait_for_network = true
  depends_on       = [lxd_project.e2e, lxd_profile.e2e]
}

# VMs
resource "lxd_instance" "e2e-vm" {
  for_each = {
    for _, v in local.vms : v.instance => v.target
  }

  name             = each.key
  target           = each.value
  type             = "virtual-machine"
  remote           = var.remote
  project          = var.project
  profiles         = [var.profile]
  image            = lxd_cached_image.vm[0].fingerprint
  wait_for_network = true
  allow_restart    = true
  depends_on       = [lxd_project.e2e, lxd_profile.e2e]

  config = {
    "migration.stateful" = "true"
  }
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
        instance = "v${format("%02d", var.vms_per_host * index + i + 1)}"
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
