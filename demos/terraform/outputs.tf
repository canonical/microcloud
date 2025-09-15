output "microcloud_instances" {
  description = "List of MicroCloud instance names"
  value       = lxd_instance.microcloud[*].name
}

output "session_passphrase" {
  description = "MicroCloud session passphrase"
  value       = random_password.session_passphrase.result
  sensitive   = true
}

output "vm_ips" {
  description = "IP addresses of VMs"
  value = {
    for i, instance in lxd_instance.microcloud : instance.name => cidrhost(local.lookup_subnet, var.ip_base_offset + (i * var.ip_increment))
  }
}

output "detected_lookup_subnet" {
  description = "Auto-detected IPv4 subnet from the lookup bridge"
  value       = local.lookup_subnet
}

output "detected_lookup_subnet_ipv6" {
  description = "Auto-detected IPv6 subnet from the lookup bridge"
  value       = local.lookup_subnet_ipv6
}

output "bridge_config" {
  description = "Bridge configuration"
  value = {
    ipv4_address = data.lxd_network.lookup_bridge.config["ipv4.address"]
    ipv6_address = try(data.lxd_network.lookup_bridge.config["ipv6.address"], "not configured")
    all_config   = data.lxd_network.lookup_bridge.config
  }
}


output "storage_layout" {
  description = "Storage configuration for each VM"
  value = {
    for i, system in local.systems : system.name => {
      local_disk = "${var.disk_device_base}${lxd_volume.local_disk[i].name}"
      ceph_disk  = system.has_ceph ? "${var.disk_device_base}${lxd_volume.ceph_disk[local.ceph_disk_mapping[i]].name}" : null
    }
  }
}

output "system_ready_status" {
  description = "System boot completion status and output"
  value = {
    for instance in lxd_instance.microcloud : instance.name => {
      stdout    = try(instance.execs["01_wait_system_ready"].stdout, "No system ready exec configured")
      stderr    = try(instance.execs["01_wait_system_ready"].stderr, "")
      exit_code = try(instance.execs["01_wait_system_ready"].exit_code, null)
    } if instance.name == var.initiator
  }
}


output "cloud_init_status" {
  description = "Cloud-init completion status and output"
  value = {
    for instance in lxd_instance.microcloud : instance.name => {
      stdout    = try(instance.execs["02_wait_cloud_init"].stdout, "No cloud-init exec configured")
      stderr    = try(instance.execs["02_wait_cloud_init"].stderr, "")
      exit_code = try(instance.execs["02_wait_cloud_init"].exit_code, null)
    } if instance.name == var.initiator
  }
}


output "microcloud_status" {
  description = "MicroCloud status output"
  value = {
    for instance in lxd_instance.microcloud : instance.name => {
      stdout    = try(instance.execs["03_microcloud_status"].stdout, "No microcloud status exec configured")
      stderr    = try(instance.execs["03_microcloud_status"].stderr, "")
      exit_code = try(instance.execs["03_microcloud_status"].exit_code, null)
    } if instance.name == var.initiator
  }
}
