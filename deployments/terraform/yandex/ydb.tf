locals {
  managed_ydb_autoscale = local.managed_ydb_dedicated_config.scale_policy.auto != null
  managed_ydb_labels = local.managed_ydb_autoscale ? merge(
    local.managed_ydb_labels_input,
    { enable_autoscaling = "1" },
  ) : local.managed_ydb_labels_input
}

resource "yandex_ydb_database_serverless" "this" {
  count = local.managed_ydb_serverless ? 1 : 0

  name                = local.managed_ydb_name
  folder_id           = local.managed_ydb_folder_id
  location_id         = local.managed_ydb_location_id
  deletion_protection = local.managed_ydb_deletion_protection
  labels              = local.managed_ydb_labels_input

  dynamic "serverless_database" {
    for_each = (
      local.managed_ydb_serverless_config.enable_throttling_rcu_limit ||
      local.managed_ydb_serverless_config.throttling_rcu_limit > 0 ||
      local.managed_ydb_serverless_config.provisioned_rcu_limit > 0 ||
      local.managed_ydb_serverless_config.storage_size_limit > 0
    ) ? [local.managed_ydb_serverless_config] : []
    content {
      enable_throttling_rcu_limit = serverless_database.value.enable_throttling_rcu_limit
      throttling_rcu_limit        = serverless_database.value.throttling_rcu_limit > 0 ? serverless_database.value.throttling_rcu_limit : null
      provisioned_rcu_limit       = serverless_database.value.provisioned_rcu_limit > 0 ? serverless_database.value.provisioned_rcu_limit : null
      storage_size_limit          = serverless_database.value.storage_size_limit > 0 ? serverless_database.value.storage_size_limit : null
    }
  }
}

resource "yandex_ydb_database_dedicated" "this" {
  count = local.managed_ydb_dedicated ? 1 : 0

  name                = local.managed_ydb_name
  folder_id           = local.managed_ydb_folder_id
  location_id         = local.managed_ydb_location_id
  deletion_protection = local.managed_ydb_deletion_protection
  labels              = local.managed_ydb_labels

  network_id         = var.network.network_id
  subnet_ids         = length(local.managed_ydb_dedicated_config.subnet_ids) > 0 ? local.managed_ydb_dedicated_config.subnet_ids : [for s in yandex_vpc_subnet.subnet : s.id]
  security_group_ids = length(local.managed_ydb_dedicated_config.security_group_ids) > 0 ? local.managed_ydb_dedicated_config.security_group_ids : [yandex_vpc_security_group.security-group.id]
  assign_public_ips  = local.managed_ydb_dedicated_config.assign_public_ips

  resource_preset_id = local.managed_ydb_dedicated_config.resource_preset_id

  scale_policy {
    dynamic "fixed_scale" {
      for_each = local.managed_ydb_dedicated_config.scale_policy.fixed != null ? [local.managed_ydb_dedicated_config.scale_policy.fixed] : []
      content {
        size = fixed_scale.value.size
      }
    }

    dynamic "auto_scale" {
      for_each = local.managed_ydb_dedicated_config.scale_policy.auto != null ? [local.managed_ydb_dedicated_config.scale_policy.auto] : []
      content {
        min_size = auto_scale.value.min_size
        max_size = auto_scale.value.max_size
        target_tracking {
          cpu_utilization_percent = auto_scale.value.cpu_utilization_percent
        }
      }
    }
  }

  storage_config {
    group_count     = local.managed_ydb_dedicated_config.storage_config.group_count
    storage_type_id = local.managed_ydb_dedicated_config.storage_config.storage_type_id
  }
}
