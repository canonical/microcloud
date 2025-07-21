variable "memory_per_instance" {
  description = "Memory per instance in GiB"
  type        = number
  default     = 2
}

variable "cpu_per_instance" {
  description = "CPU cores per instance"
  type        = number
  default     = 2
}

variable "storage_pool" {
  description = "Storage pool to use"
  type        = string
  default     = "default"
}

variable "ubuntu_image" {
  description = "Ubuntu image to use"
  type        = string
  default     = "ubuntu:24.04"
}

variable "session_timeout" {
  description = "Session timeout in seconds"
  type        = number
  default     = 300
}

variable "lookup_timeout" {
  description = "Lookup timeout in seconds"
  type        = number
  default     = 300
}

variable "ovn_ipv4_gateway" {
  description = "OVN IPv4 gateway"
  type        = string
  default     = "192.0.2.1/24"
}

variable "ovn_ipv4_range" {
  description = "OVN IPv4 range"
  type        = string
  default     = "192.0.2.100-192.0.2.254"
}

variable "ovn_ipv6_gateway" {
  description = "OVN IPv6 gateway"
  type        = string
  default     = "2001:db8:d:200::1/64"
}

variable "ovn_dns_servers" {
  description = "OVN DNS servers"
  type        = string
  default     = "192.0.2.1,2001:db8:d:200::1"
}

variable "microceph_channel" {
  description = "MicroCeph snap channel"
  type        = string
  default     = "squid/stable"
}

variable "microovn_channel" {
  description = "MicroOVN snap channel"
  type        = string
  default     = "24.03/stable"
}

variable "microcloud_channel" {
  description = "MicroCloud snap channel"
  type        = string
  default     = "2/stable"
}

variable "lxd_channel" {
  description = "LXD snap channel"
  type        = string
  default     = "5.21/stable"
}

variable "vm_names" {
  description = "List of VM names"
  type        = list(string)
  default     = ["micro1", "micro2", "micro3", "micro4"]
}

variable "ceph_nodes" {
  description = "List of VM names that should have Ceph storage"
  type        = list(string)
  default     = ["micro1", "micro2", "micro3"]
}

variable "network_name" {
  description = "Name of the MicroCloud network"
  type        = string
  default     = "microbr0"
}

variable "network_ipv4_address" {
  description = "IPv4 address for MicroCloud network"
  type        = string
  default     = "192.0.2.1/24"
}

variable "network_ipv6_address" {
  description = "IPv6 address for MicroCloud network"
  type        = string
  default     = "2001:db8:d:200::1/64"
}

variable "local_disk_size" {
  description = "Size of local disk"
  type        = string
  default     = "10GiB"
}

variable "ceph_disk_size" {
  description = "Size of Ceph disk"
  type        = string
  default     = "20GiB"
}

variable "local_disk_device" {
  description = "Local disk device path"
  type        = string
  default     = "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_local"
}

variable "ceph_disk_device" {
  description = "Ceph disk device path"
  type        = string
  default     = "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_lxd_remote"
}

variable "lookup_interface" {
  description = "Interface for MicroCloud lookup and discovery traffic"
  type        = string
  default     = "eth0"
}

variable "uplink_device_name" {
  description = "Uplink network device name for VM configuration"
  type        = string
  default     = "eth1"
}

variable "lookup_bridge" {
  description = "Bridge for MicroCloud lookup and discovery traffic"
  type        = string
  default     = "lxdbr0"
}


variable "vm_count" {
  description = "Number of VMs to create"
  type        = number
  default     = 4
}

variable "ip_base_offset" {
  description = "Base IP address offset for the first VM"
  type        = number
  default     = 10
}

variable "ip_increment" {
  description = "IP address increment between VMs"
  type        = number
  default     = 10
}

variable "local_disk_name_prefix" {
  description = "Prefix for local disk volume names"
  type        = string
  default     = "local"
}

variable "ceph_disk_name_prefix" {
  description = "Prefix for Ceph disk volume names"
  type        = string
  default     = "remote"
}

variable "initiator" {
  description = "Name of the initiator node for MicroCloud cluster (matches preseed initiator)"
  type        = string
  default     = "micro1"
}

variable "ovn_uplink_interface" {
  description = "OVN uplink interface name"
  type        = string
  default     = "enp6s0"
}
