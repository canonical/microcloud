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

resource "random_password" "session_passphrase" {
  length  = 32
  special = true
}

locals {
  systems = [
    for i, name in var.vm_names : {
      name     = name
      ip       = cidrhost(var.lookup_subnet, var.ip_base_offset + (i * var.ip_increment))
      has_ceph = contains(var.ceph_nodes, name)
    }
  ]
  
  # Create a simple mapping: VM index -> ceph disk index
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
  count  = var.vm_count
  name   = var.vm_names[count.index]
  image  = var.ubuntu_image
  type   = "virtual-machine"
  
  config = {
    "security.secureboot" = "false"
    "cloud-init.user-data" = templatefile("${path.module}/cloud-init.yaml.tpl", {
      hostname = var.vm_names[count.index]
      lookup_interface = var.lookup_interface
      uplink_device_name = var.uplink_device_name
      ovn_uplink_interface = var.ovn_uplink_interface
      microceph_channel = var.microceph_channel
      microovn_channel = var.microovn_channel
      microcloud_channel = var.microcloud_channel
      lxd_channel = var.lxd_channel
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
      nictype = "bridged"
      parent  = var.lookup_bridge
      "ipv4.address" = cidrhost(var.lookup_subnet, var.ip_base_offset + (count.index * var.ip_increment))
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
      subnet = var.lookup_subnet
      internal_subnet = var.lookup_subnet
      lookup_timeout = var.lookup_timeout
      session_passphrase = random_password.session_passphrase.result
      session_timeout = var.session_timeout
      systems = local.systems
      ovn_ipv4_gateway = var.ovn_ipv4_gateway
      ovn_ipv4_range = var.ovn_ipv4_range
      ovn_ipv6_gateway = var.ovn_ipv6_gateway
      ovn_dns_servers = var.ovn_dns_servers
      local_disk_device = var.local_disk_device
      ceph_disk_device = var.ceph_disk_device
      uplink_device_name = var.uplink_device_name
      initiator = var.initiator
      ovn_uplink_interface = var.ovn_uplink_interface
      current_node_has_ceph = local.systems[count.index].has_ceph
    })
    target_path = "/root/microcloud_preseed.yaml"
    mode        = "0600"
  }


  depends_on = [
    lxd_network.microbr0,
    lxd_volume.local_disk,
    lxd_volume.ceph_disk
  ]
}
