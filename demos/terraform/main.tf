terraform {
  required_providers {
    lxd = {
      source  = "terraform-lxd/lxd"
      version = "~> 2.5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.5.1"
    }
  }
  required_version = ">= 1.0.0"
}

provider "lxd" {
  generate_client_certificates = true
  accept_remote_certificate    = true
}

data "lxd_network" "lookup_bridge" {
  name = var.lookup_bridge
}

resource "random_password" "session_passphrase" {
  length  = 32
  special = true
}

locals {
  lookup_subnet      = cidrsubnet(data.lxd_network.lookup_bridge.config["ipv4.address"], 0, 0)
  lookup_subnet_ipv6 = cidrsubnet(data.lxd_network.lookup_bridge.config["ipv6.address"], 0, 0)

  systems = [
    for i, name in var.vm_names : {
      name     = name
      ip       = cidrhost(local.lookup_subnet, var.ip_base_offset + (i * var.ip_increment))
      has_ceph = contains(var.ceph_nodes, name)
    }
  ]

  ceph_disk_mapping = {
    for i, system in local.systems : i => length([
      for j, s in local.systems : j if j < i && s.has_ceph
    ]) if system.has_ceph
  }
}

resource "lxd_network" "microbr0" {
  name = var.network_name

  config = {
    "ipv4.address" = var.network_ipv4_address
    "ipv4.nat"     = "true"
    "ipv6.address" = var.network_ipv6_address
    "ipv6.nat"     = "true"
  }
}

resource "lxd_volume" "local_disk" {
  count        = var.vm_count
  name         = "${var.local_disk_name_prefix}${count.index + 1}"
  pool         = var.storage_pool
  type         = "custom"
  content_type = "block"
  config = {
    size = var.local_disk_size
  }
}

resource "lxd_volume" "ceph_disk" {
  count        = length([for system in local.systems : system if system.has_ceph])
  name         = "${var.ceph_disk_name_prefix}${count.index + 1}"
  pool         = var.storage_pool
  type         = "custom"
  content_type = "block"
  config = {
    size = var.ceph_disk_size
  }
}

resource "lxd_instance" "microcloud" {
  count            = var.vm_count
  name             = var.vm_names[count.index]
  image            = var.ubuntu_image
  type             = "virtual-machine"
  wait_for_network = true

  timeouts = {
    create = var.instance_create_timeout
  }

  config = {
    "security.secureboot" = "false"
    "cloud-init.user-data" = templatefile("${path.module}/cloud-init.yaml.tpl", {
      hostname             = var.vm_names[count.index]
      lookup_interface     = var.lookup_interface
      uplink_device_name   = var.uplink_device_name
      ovn_uplink_interface = var.ovn_uplink_interface
      microceph_channel    = var.microceph_channel
      microovn_channel     = var.microovn_channel
      microcloud_channel   = var.microcloud_channel
      lxd_channel          = var.lxd_channel
    })
  }

  limits = {
    cpu    = var.cpu_per_instance
    memory = "${var.memory_per_instance}GiB"
  }

  device {
    name = var.lookup_interface
    type = "nic"
    properties = {
      nictype        = "bridged"
      parent         = var.lookup_bridge
      "ipv4.address" = local.systems[count.index].ip
    }
  }

  device {
    name = var.uplink_device_name
    type = "nic"
    properties = {
      nictype = "bridged"
      parent  = lxd_network.microbr0.name
    }
  }

  device {
    name = "local"
    type = "disk"
    properties = {
      pool   = var.storage_pool
      source = lxd_volume.local_disk[count.index].name
    }
  }

  dynamic "device" {
    for_each = local.systems[count.index].has_ceph ? [1] : []
    content {
      name = "remote"
      type = "disk"
      properties = {
        pool   = var.storage_pool
        source = lxd_volume.ceph_disk[local.ceph_disk_mapping[count.index]].name
      }
    }
  }

  file {
    content = templatefile("${path.module}/microcloud_preseed.yaml.tpl", {
      subnet                = local.lookup_subnet
      internal_subnet       = local.lookup_subnet
      lookup_timeout        = var.lookup_timeout
      session_passphrase    = random_password.session_passphrase.result
      session_timeout       = var.session_timeout
      systems               = local.systems
      ovn_ipv4_gateway      = var.ovn_ipv4_gateway
      ovn_ipv4_range        = var.ovn_ipv4_range
      ovn_ipv6_gateway      = var.ovn_ipv6_gateway
      ovn_dns_servers       = var.ovn_dns_servers
      local_disk_device     = "${var.disk_device_base}${lxd_volume.local_disk[count.index].name}"
      ceph_disk_device      = local.systems[count.index].has_ceph ? "${var.disk_device_base}${lxd_volume.ceph_disk[local.ceph_disk_mapping[count.index]].name}" : ""
      uplink_device_name    = var.uplink_device_name
      initiator             = var.initiator
      ovn_uplink_interface  = var.ovn_uplink_interface
      current_node_has_ceph = local.systems[count.index].has_ceph
    })
    target_path = "/root/microcloud_preseed.yaml"
    mode        = "0600"
  }


  # cloud-init status --wait fails with an "invalid argument" error (129) because the command is triggered by Terraform's on_start hook very early in the boot cycle.
  # At this point, the cloud-init daemon or its environment is not yet fully initialized to correctly respond to status queries, even though cloud-init eventually completes successfully.
  #
  # Solution: Wait for systemctl first to ensure the system is fully booted before calling cloud-init status --wait.
  # This prevents the early boot timing issue and allows cloud-init to respond properly.
  execs = var.vm_names[count.index] == var.initiator ? {

    "01_wait_system_ready" = {
      command       = ["systemctl", "is-system-running", "--wait"]
      trigger       = "on_start"
      record_output = true
      fail_on_error = true
    }

    "02_wait_cloud_init" = {
      command       = ["cloud-init", "status", "--wait"]
      trigger       = "on_start"
      record_output = false # Suppress progress dots for cleaner output
      fail_on_error = true
    }

    "03_microcloud_status" = {
      command       = ["/snap/bin/microcloud", "status"]
      trigger       = "on_start"
      record_output = true
      fail_on_error = true
    }
  } : {}


  depends_on = [
    data.lxd_network.lookup_bridge,
    lxd_network.microbr0,
    lxd_volume.local_disk,
    lxd_volume.ceph_disk
  ]
}
