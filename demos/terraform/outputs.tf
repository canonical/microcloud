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
    for i, instance in lxd_instance.microcloud : instance.name => cidrhost(var.lookup_subnet, var.ip_base_offset + (i * var.ip_increment))
  }
}

output "storage_layout" {
  description = "Storage configuration for each VM"
  value = {
    for i, system in local.systems : system.name => {
      local_disk = var.local_disk_device
      ceph_disk  = system.has_ceph ? var.ceph_disk_device : null
    }
  }
}
