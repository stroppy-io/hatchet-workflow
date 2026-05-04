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

# Managed YDB exposes a TLS gRPC endpoint on port 2135. The provider already
# returns ydb_full_endpoint in the form "grpcs://host:2135/?database=/path"
# for serverless, but we want the host:port and database path separately so
# task_stroppy can build the URL the same way as for self-hosted clusters.
output "ydb_endpoint" {
  value = local.is_serverless ? (
    length(yandex_ydb_database_serverless.this) > 0 ? yandex_ydb_database_serverless.this[0].ydb_api_endpoint : ""
  ) : (
    length(yandex_ydb_database_dedicated.this) > 0 ? yandex_ydb_database_dedicated.this[0].ydb_api_endpoint : ""
  )
}

output "ydb_database_path" {
  value = local.is_serverless ? (
    length(yandex_ydb_database_serverless.this) > 0 ? yandex_ydb_database_serverless.this[0].database_path : ""
  ) : (
    length(yandex_ydb_database_dedicated.this) > 0 ? yandex_ydb_database_dedicated.this[0].database_path : ""
  )
}

output "stroppy_service_account_id" {
  value = yandex_iam_service_account.stroppy.id
}

# Full attribute snapshot of the managed YDB resource. Mirrors every field
# the Yandex provider exposes so the run overview can show the database as
# it actually exists in the cloud (id, status, endpoints, location, …).
# Fields that don't apply to a flavour are emitted as null.
output "ydb_managed" {
  value = local.is_serverless ? (
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
      storage_groups        = null
      storage_type_id       = null
      throttling_rcu_limit  = var.managed.throttling_rcu_limit
    } : null
  ) : (
    length(yandex_ydb_database_dedicated.this) > 0 ? {
      id                    = yandex_ydb_database_dedicated.this[0].id
      name                  = yandex_ydb_database_dedicated.this[0].name
      type                  = "dedicated"
      folder_id             = yandex_ydb_database_dedicated.this[0].folder_id
      location_id           = yandex_ydb_database_dedicated.this[0].location_id
      database_path         = yandex_ydb_database_dedicated.this[0].database_path
      ydb_api_endpoint      = yandex_ydb_database_dedicated.this[0].ydb_api_endpoint
      ydb_full_endpoint     = yandex_ydb_database_dedicated.this[0].ydb_full_endpoint
      document_api_endpoint = yandex_ydb_database_dedicated.this[0].document_api_endpoint
      tls_enabled           = yandex_ydb_database_dedicated.this[0].tls_enabled
      status                = yandex_ydb_database_dedicated.this[0].status
      created_at            = yandex_ydb_database_dedicated.this[0].created_at
      labels                = yandex_ydb_database_dedicated.this[0].labels
      resource_preset_id    = yandex_ydb_database_dedicated.this[0].resource_preset_id
      network_id            = yandex_ydb_database_dedicated.this[0].network_id
      subnet_ids            = yandex_ydb_database_dedicated.this[0].subnet_ids
      storage_groups        = var.managed.storage_groups
      storage_type_id       = var.managed.storage_type_id
      throttling_rcu_limit  = null
    } : null
  )
}
