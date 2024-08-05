
variable "pro_token" {
    type = string
}

variable "lxd_project" {
    type = string
    default = "microcloud2"
}
# TODO Remove this - it can be computed/derived.
variable "bridge_nic" {
    type = string
    default = "br0"
}

variable "host_bridge_network" {
    type = string
    default = "br0"
}


variable "lookup_subnet" {
    type = string
    default = "10.10.32.1/24"
}

variable "ovn_gateway" {
    type = string
    default = "192.168.254.1/24"
}
variable "ovn_range_start" {
    type = string
    default = "192.168.254.150"
}
variable "ovn_range_end" {
    type = string
    default = "192.168.254.200"
}
variable "ssh_pubkey" {
    type = string
}

