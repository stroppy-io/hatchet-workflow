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
