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

// compute carries settings shared by every VM this module provisions --
// image/platform defaults and boot disk sizing. Per-VM data (which
// machines to create, how many, their sizing) comes from the standard
// provider-contract input `stroppy_nodes` below, not from this variable;
// see stroppy_nodes' own doc comment for why the split is drawn there.
variable "compute" {
  description = "Global compute defaults shared by every stroppy_nodes VM (image, platform, boot disk). Per-VM sizing comes from stroppy_nodes."
  type = object({
    platform_id        = string
    image_id           = string
    serial_port_enable = optional(bool, false)
    boot_disk_gb       = optional(number, 20)
    boot_disk_type     = optional(string, "network-hdd")
  })
  validation {
    condition     = var.compute.platform_id != ""
    error_message = "compute.platform_id must be set"
  }
  validation {
    condition     = var.compute.image_id != ""
    error_message = "compute.image_id must be set"
  }
}

// stroppy_nodes is the standard provider-contract input every terraform
// module behind internal/infrastructure/provider.NewTerraform is given (see
// docs/superpowers/specs/2026-07-03-yaml-dsl-pivot-design.md §3 and
// terraform.go's tfNode/tfNodesForGroup): one entry per requested machine,
// already expanded from the DSL's provider-agnostic MachineGroup list and
// already lowered (disk.type is yandex's own "network-ssd"/"network-hdd"
// strings, per manifest.yaml's lowering table) -- this module never sees
// MachineGroup, cpu/ram_mb, or the DSL's disk-type domain vocabulary
// directly, only this already-lowered shape.
//
// `ext` is this module's per-node escape hatch (cluster.yaml's
// `machines.<name>.yandex: {...}` block, see the stroppy_machine_ext
// variable below for its declared shape): a node may override
// platform_id/image_id/zone/public_ip/user_data/network_acceleration for
// itself, falling back to `compute`'s module-wide defaults (see
// locals.vms in vm.tf). ext is intentionally typed `any` here -- the DSL
// compiler enforces its real shape against stroppy_machine_ext's schema at
// compile time (internal/dsl/schema/tfvars_schemapb.go), so re-typing it
// here would only duplicate that check less precisely.
//
// default = [] so `terraform destroy` (which never sets stroppy_nodes --
// see terraform.go's Destroy, which only spreads provider params) still
// validates.
variable "stroppy_nodes" {
  description = "Standard provider-contract input: machines requested by the DSL compiler."
  type = list(object({
    id     = string
    group  = string
    cpu    = number
    ram_gb = number
    disk = optional(object({
      size_gb = number
      type    = string
    }))
    ext = optional(any, {})
  }))
  default = []
  validation {
    condition = alltrue([
      for n in var.stroppy_nodes :
      contains(["standard", "software_accelerated"], try(n.ext.network_acceleration, "standard"))
    ])
    error_message = "stroppy_nodes[*].ext.network_acceleration must be 'standard' or 'software_accelerated'"
  }
}

// stroppy_machine_ext declares the shape of stroppy_nodes[*].ext purely for
// schema derivation (internal/dsl/schema/tfvars_schemapb.go's
// machineExtVarName convention splits a variable with this exact name out
// of provider.params and into $defs.machineExt, so cluster.yaml's
// `machines.<name>.yandex: {...}` block gets validated at compile time).
// It is never itself set as a tfvar -- terraform.go passes each node's ext
// inline inside stroppy_nodes[*].ext, not as a separate top-level var --
// so `default = {}` only keeps `terraform validate`/apply happy for an
// unused input.
variable "stroppy_machine_ext" {
  description = "Schema-only: shape of stroppy_nodes[*].ext (cluster.yaml's machines.<name>.yandex: block). Never itself passed as a tfvar."
  type = object({
    platform_id          = optional(string, "")
    image_id             = optional(string, "")
    zone                 = optional(string, "")
    public_ip            = optional(bool, false)
    user_data            = optional(string, "")
    network_acceleration = optional(string, "standard")
  })
  default = {}
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
