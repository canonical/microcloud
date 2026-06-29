terraform {
  required_version = ">= 1.3.0"
  required_providers {
    lxd = {
      source  = "terraform-lxd/lxd"
      version = ">= 3.0.2"
    }
  }
}

provider "lxd" {
  remote {
    name    = "local"
    address = "unix://"
  }
}
