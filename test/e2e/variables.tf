variable "remote" {
  description = "LXD remote to use"
  type        = string
  validation {
    condition     = !can(regex(":", var.remote))
    error_message = "Remote name must not contain `:` character"
  }
}

variable "project" {
  description = "Project to use"
  type        = string
  default     = "e2e-testing"
  validation {
    condition     = length(var.project) > 0
    error_message = "Project name must not be empty"
  }
}

variable "containers_per_host" {
  description = "Number of containers per host"
  type        = number
  default     = 9
  validation {
    condition     = var.containers_per_host >= 0
    error_message = "Number of containers per host must be greater or equal to 0"
  }
}

variable "vms_per_host" {
  description = "Number of VMs per host"
  type        = number
  default     = 3
  validation {
    condition     = var.vms_per_host >= 0
    error_message = "Number of VMs per host must be greater or equal to 0"
  }
}
