variable "networking" {
  description = "Network to create"
  type = object({
    name        = string
    external_id = string
    cidr        = string
    zone        = string
  })
  validation {
    condition     = var.networking.name != ""
    error_message = "Network name should be specified"
  }
  validation {
    condition     = var.networking.external_id != ""
    error_message = "Network external ID should be specified"
  }
  validation {
    condition     = var.networking.cidr != ""
    error_message = "Network CIDR should be specified"
  }
}

variable "compute" {
  description = "Client VM(s) running stroppy. Each VM is provisioned with the stroppy SA attached so the patched ydb driver can pull SA token + CA from the YC metadata service."
  type = object({
    platform_id        = string
    image_id           = string
    serial_port_enable = bool
    vms = map(object({
      cores                     = number
      memory                    = number
      disk_size                 = number
      disk_type                 = string
      internal_ip               = string
      has_public_ip             = bool
      user_data                 = string
      network_acceleration_type = optional(string, "standard")
      secondary_disks = optional(list(object({
        device_name = string
        size_gb     = number
        type        = string
      })), [])
    }))
  })
  validation {
    condition     = length(keys(var.compute.vms)) > 0
    error_message = "At least one client VM should be specified"
  }
  validation {
    condition     = var.compute.image_id != ""
    error_message = "Image ID should be specified"
  }
  validation {
    condition     = var.compute.platform_id != ""
    error_message = "Platform ID should be specified"
  }
}

variable "managed" {
  description = "Managed YDB database. Type is 'serverless' or 'dedicated'."
  type = object({
    name                = string
    type                = string
    folder_id           = string
    location_id         = string
    resource_preset_id  = optional(string, "medium")
    storage_groups      = optional(number, 1)
    storage_type_id     = optional(string, "ssd")
    throttling_rcu_limit = optional(number, 0)
  })
  validation {
    condition     = contains(["serverless", "dedicated"], var.managed.type)
    error_message = "managed.type must be 'serverless' or 'dedicated'"
  }
  validation {
    condition     = var.managed.name != ""
    error_message = "managed.name is required"
  }
  validation {
    condition     = var.managed.folder_id != ""
    error_message = "managed.folder_id is required"
  }
}
