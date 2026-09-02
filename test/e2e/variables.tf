variable "remote" {
  description = "LXD remote to use"
  type        = string
  validation {
    condition     = !strcontains(var.remote, ":")
    error_message = "Remote name must not contain `:` character"
  }
}

variable "image_remote" {
  description = "LXD remote to fetch images from"
  type        = string
  default     = "ubuntu-minimal-daily"
}

variable "image_project" {
  description = "LXD project to cache source images into. Defaults to `project`. Set to \"default\" when `project` has features.images=false and shares images with another project instead"
  type        = string
  default     = null
}

variable "copy_image_aliases" {
  description = "Whether to copy the source image's aliases to the cached image copy. Set to true if image_remote's images carry aliases"
  type        = bool
  default     = false
}

variable "project" {
  description = "LXD project to use for e2e testing resources"
  type        = string
  default     = "e2e-testing"
}

variable "profile" {
  description = "LXD profile to use for e2e testing instances"
  type        = string
  default     = "e2e-testing"
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
