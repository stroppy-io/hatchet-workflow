variable "network" {
  description = "Existing Yandex VPC network and run subnets."
  type = object({
    name       = string
    network_id = string
    cidr       = string
    zone       = string
    subnets = optional(map(object({
      zone = string
      cidr = string
    })), {})
    managed_zones = optional(list(string), [])
  })
  validation {
    condition     = var.network.name != ""
    error_message = "network.name must be set"
  }
  validation {
    condition     = var.network.network_id != ""
    error_message = "network.network_id must be set"
  }
  validation {
    condition     = var.network.cidr != ""
    error_message = "network.cidr must be set"
  }
  validation {
    condition     = var.network.zone != ""
    error_message = "network.zone must be set"
  }
}

variable "compute" {
  description = "VMs to create."
  type = object({
    platform_id        = string
    image_id           = string
    serial_port_enable = bool
    vms = map(object({
      cores                = number
      memory_gb            = number
      boot_disk_gb         = number
      boot_disk_type       = string
      zone                 = optional(string, "")
      internal_ip          = string
      public_ip            = bool
      user_data            = string
      network_acceleration = optional(string, "standard")
      secondary_disks = optional(list(object({
        device_name = string
        size_gb     = number
        type        = string
      })), [])
    }))
  })
  validation {
    condition     = var.compute.platform_id != ""
    error_message = "compute.platform_id must be set"
  }
  validation {
    condition     = var.compute.image_id != ""
    error_message = "compute.image_id must be set"
  }
  validation {
    condition     = length(keys(var.compute.vms)) > 0
    error_message = "compute.vms must contain at least one VM"
  }
  validation {
    condition = alltrue([
      for vm in var.compute.vms :
      contains(["standard", "software_accelerated"], vm.network_acceleration)
    ])
    error_message = "compute.vms[*].network_acceleration must be 'standard' or 'software_accelerated'"
  }
}

variable "managed_ydb" {
  description = "Optional Yandex Managed Service for YDB database. null means self-hosted/IaaS only."
  type = object({
    name                 = string
    folder_id            = string
    location_id          = string
    deletion_protection  = optional(bool, false)
    labels               = optional(map(string), {})
    service_account_name = optional(string, "")
    serverless = optional(object({
      enable_throttling_rcu_limit = optional(bool, false)
      throttling_rcu_limit        = optional(number, 0)
      provisioned_rcu_limit       = optional(number, 0)
      storage_size_limit          = optional(number, 0)
    }))
    dedicated = optional(object({
      resource_preset_id = string
      scale_policy = object({
        fixed = optional(object({
          size = number
        }))
        auto = optional(object({
          min_size                = number
          max_size                = number
          cpu_utilization_percent = number
        }))
      })
      storage_config = object({
        group_count     = number
        storage_type_id = string
      })
      subnet_ids         = optional(list(string), [])
      security_group_ids = optional(list(string), [])
      assign_public_ips  = optional(bool, false)
    }))
  })
  default = null
  validation {
    condition = var.managed_ydb == null ? true : (
      var.managed_ydb.name != "" &&
      var.managed_ydb.folder_id != "" &&
      var.managed_ydb.location_id != ""
    )
    error_message = "managed_ydb.name, folder_id and location_id must be set"
  }
  validation {
    condition = var.managed_ydb == null ? true : (
      (var.managed_ydb.serverless != null ? 1 : 0) +
      (var.managed_ydb.dedicated != null ? 1 : 0)
    ) == 1
    error_message = "managed_ydb must contain exactly one of serverless or dedicated"
  }
  validation {
    condition = var.managed_ydb == null ? true : (var.managed_ydb.dedicated == null ? true : (
      (var.managed_ydb.dedicated.scale_policy.fixed != null ? 1 : 0) +
      (var.managed_ydb.dedicated.scale_policy.auto != null ? 1 : 0)
    ) == 1)
    error_message = "managed_ydb.dedicated.scale_policy must contain exactly one of fixed or auto"
  }
  validation {
    condition = var.managed_ydb == null ? true : (var.managed_ydb.dedicated == null ? true : (var.managed_ydb.dedicated.scale_policy.auto == null ? true : (
      var.managed_ydb.dedicated.scale_policy.auto.min_size >= 1 &&
      var.managed_ydb.dedicated.scale_policy.auto.max_size >= var.managed_ydb.dedicated.scale_policy.auto.min_size &&
      var.managed_ydb.dedicated.scale_policy.auto.cpu_utilization_percent >= 1 &&
      var.managed_ydb.dedicated.scale_policy.auto.cpu_utilization_percent <= 100
    )))
    error_message = "managed_ydb.dedicated.scale_policy.auto must satisfy min_size >= 1, max_size >= min_size, cpu_utilization_percent 1..100"
  }
}
