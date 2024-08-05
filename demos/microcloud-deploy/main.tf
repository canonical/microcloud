terraform {
  required_providers {
    lxd = {
      source  = "terraform-lxd/lxd"
      version = "2.0.0"
    }
    ssh = {
      source = "loafoe/ssh"
      version = "2.7.0"
    }
  }
}

provider "lxd" {
}
provider ssh {
  # Configuration options
}
locals {
  cloud_init = <<EOT
#cloud-config
users:
  - name: ubuntu
    ssh_authorized_keys:
    - ${var.ssh_pubkey}
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /usr/bin/bash
snap:
  commands:
    1: [snap, refresh, lxd, --channel=5.21/stable, --cohort="+"]
    2: [snap, install, microceph, --channel=quincy/stable, --cohort="+"]
    3: [snap, install, microovn, --channel=22.03/stable, --cohort="+"]
    4: [snap, install, microcloud, --channel=latest/stable, --cohort="+"]
EOT
  cloud_init_network = <<EOT
network:
  version: 2
  ethernets:
      enp5s0:
          dhcp4: false
  bridges:
      br0:
        dhcp4: true
        dhcp6: false
        interfaces: [enp5s0]
EOT
}
resource "lxd_project" "microcloud" {
  name        = "${var.lxd_project}"
  description = "Your Microcloud!"
  config = {
    "features.storage.volumes" = true
    "features.images"          = false
    "features.profiles"        = false
  }
}

resource "lxd_volume" "mc_sdb_vols" {
  count = 3
  name = "microcloud-sdb-${count.index}"
  content_type = "block"
  pool = "default"
  project = "${var.lxd_project}"
}
resource "lxd_instance" "microcloud_nodes" {
  count            = 3
  name             = "microcloud-${count.index}"
  image            = "ubuntu:jammy"
  type             = "virtual-machine"
  project          = lxd_project.microcloud.name
  wait_for_network = true
  config = {
    "boot.autostart"       = true
    "cloud-init.user-data" = local.cloud_init
    "cloud-init.network-config" =local.cloud_init_network
  }

  limits = {
    cpu    = 4
    memory = "8GiB"
  }
  device {
    name = "root"
    type = "disk"
    properties = {
      pool = "default"
      path = "/"
      size = "200GiB"
    }
  }
  device {
    name = "sdb"
    type = "disk"
    properties = {
      pool = "default"
      source = lxd_volume.mc_sdb_vols[count.index].name
    }
  }
  device {
    name = "eth0"
    type = "nic"
    properties = {
      nictype = "bridged"
      parent  = "${var.host_bridge_network}"
    }
  }
}

resource "ssh_resource" "microcloud_init" {
  host         = lxd_instance.microcloud_nodes[0].ipv4_address
  user         = "ubuntu"
  agent        = false
  private_key  = file("~/.ssh/id_rsa")
  when         = "create" # Default
  depends_on = [ lxd_instance.microcloud_nodes ]
  file {
    content     = templatefile("${path.module}/templates/mc-init.tmpl", { instances = [for i in lxd_instance.microcloud_nodes : i.name], lookup_subnet = var.lookup_subnet, bridge_nic = var.bridge_nic, bridge_nic_cidr = var.lookup_subnet, ovn_gateway = var.ovn_gateway, ovn_range_start = var.ovn_range_start, ovn_range_end = var.ovn_range_end, microcloud_one_address = "${lxd_instance.microcloud_nodes[0].ipv4_address}/24"})
    destination = "/home/ubuntu/init-mc.yaml"
    permissions = "0600"
  }
    commands = [
    "cat /home/ubuntu/init-mc.yaml | sudo microcloud init --preseed",
  ]
}
