locals {
  # stroppy_machines_disk_device is always "" (empty), regardless of
  # whether a VM has a secondary/data disk: this module's secondary disk
  # `device_name` is a Yandex Compute API label, not the guest OS device
  # path a recipe's `mkfs ${{ machine.disks[0].path }}` step needs -- so
  # reporting it as disk_device would be actively wrong. Leaving it empty
  # lets the Go side fall back to its own defaultDiskDevice ("/dev/vdb",
  # the cloud-image convention for a single extra block device -- see
  # internal/infrastructure/provider/terraform.go's tfMachineOutput.
  # DiskDevice / defaultDiskDevice and machineState's doc comment) rather
  # than this module inventing a path from a label that isn't one.
  stroppy_machines_disk_device = {
    for vm_name, vm in local.vms : vm_name => ""
  }
}

# stroppy_machines is the provider contract output every terraform module
# behind internal/infrastructure/provider.NewTerraform must export (see
# terraform.go's stroppyMachinesOutputKey/tfMachineOutput): one entry per
# provisioned VM, id matching the stroppy_nodes id this module was given
# (== the vms map key == yandex_compute_instance.vms's resource "name",
# since vm.tf sets `name = each.key`).
output "stroppy_machines" {
  value = [
    for vm_name, vm in yandex_compute_instance.vms : {
      id          = vm_name
      private_ip  = vm.network_interface[0].ip_address
      public_ip   = vm.network_interface[0].nat_ip_address
      disk_device = local.stroppy_machines_disk_device[vm_name]
    }
  ]
}

output "vm_ips" {
  value = {
    for _, vm in yandex_compute_instance.vms :
    vm.name => {
      id          = vm.id
      nat_ip      = vm.network_interface[0].nat_ip_address
      internal_ip = vm.network_interface[0].ip_address
    }
  }
}

output "vms" {
  value = {
    for _, vm in yandex_compute_instance.vms :
    vm.name => {
      id          = vm.id
      public_ip   = vm.network_interface[0].nat_ip_address
      internal_ip = vm.network_interface[0].ip_address
    }
  }
}

output "ydb_endpoint" {
  value = local.managed_ydb_serverless ? (
    length(yandex_ydb_database_serverless.this) > 0 ? yandex_ydb_database_serverless.this[0].ydb_api_endpoint : ""
    ) : (
    local.managed_ydb_dedicated && length(yandex_ydb_database_dedicated.this) > 0 ? yandex_ydb_database_dedicated.this[0].ydb_api_endpoint : ""
  )
}

output "ydb_database_path" {
  value = local.managed_ydb_serverless ? (
    length(yandex_ydb_database_serverless.this) > 0 ? yandex_ydb_database_serverless.this[0].database_path : ""
    ) : (
    local.managed_ydb_dedicated && length(yandex_ydb_database_dedicated.this) > 0 ? yandex_ydb_database_dedicated.this[0].database_path : ""
  )
}

output "stroppy_service_account_id" {
  value = try(yandex_iam_service_account.stroppy[0].id, "")
}

output "service_account_id" {
  value = try(yandex_iam_service_account.stroppy[0].id, "")
}

output "managed_ydb" {
  value = local.managed_ydb_serverless ? (
    length(yandex_ydb_database_serverless.this) > 0 ? {
      id                    = yandex_ydb_database_serverless.this[0].id
      name                  = yandex_ydb_database_serverless.this[0].name
      type                  = "serverless"
      folder_id             = yandex_ydb_database_serverless.this[0].folder_id
      location_id           = yandex_ydb_database_serverless.this[0].location_id
      database_path         = yandex_ydb_database_serverless.this[0].database_path
      ydb_api_endpoint      = yandex_ydb_database_serverless.this[0].ydb_api_endpoint
      ydb_full_endpoint     = yandex_ydb_database_serverless.this[0].ydb_full_endpoint
      document_api_endpoint = yandex_ydb_database_serverless.this[0].document_api_endpoint
      tls_enabled           = yandex_ydb_database_serverless.this[0].tls_enabled
      status                = yandex_ydb_database_serverless.this[0].status
      created_at            = yandex_ydb_database_serverless.this[0].created_at
      labels                = yandex_ydb_database_serverless.this[0].labels
      resource_preset_id    = null
      network_id            = null
      subnet_ids            = null
      storage_config        = null
      scale_policy          = null
    } : null
    ) : (
    local.managed_ydb_dedicated && length(yandex_ydb_database_dedicated.this) > 0 ? {
      id                    = yandex_ydb_database_dedicated.this[0].id
      name                  = yandex_ydb_database_dedicated.this[0].name
      type                  = "dedicated"
      folder_id             = yandex_ydb_database_dedicated.this[0].folder_id
      location_id           = yandex_ydb_database_dedicated.this[0].location_id
      database_path         = yandex_ydb_database_dedicated.this[0].database_path
      ydb_api_endpoint      = yandex_ydb_database_dedicated.this[0].ydb_api_endpoint
      ydb_full_endpoint     = yandex_ydb_database_dedicated.this[0].ydb_full_endpoint
      document_api_endpoint = null
      tls_enabled           = yandex_ydb_database_dedicated.this[0].tls_enabled
      status                = yandex_ydb_database_dedicated.this[0].status
      created_at            = yandex_ydb_database_dedicated.this[0].created_at
      labels                = yandex_ydb_database_dedicated.this[0].labels
      resource_preset_id    = yandex_ydb_database_dedicated.this[0].resource_preset_id
      network_id            = yandex_ydb_database_dedicated.this[0].network_id
      subnet_ids            = yandex_ydb_database_dedicated.this[0].subnet_ids
      storage_config = {
        group_count     = local.managed_ydb_dedicated_config.storage_config.group_count
        storage_type_id = local.managed_ydb_dedicated_config.storage_config.storage_type_id
      }
      scale_policy = local.managed_ydb_dedicated_config.scale_policy
    } : null
  )
}
