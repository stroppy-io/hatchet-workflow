locals {
  is_serverless = var.managed.type == "serverless"
  is_dedicated  = var.managed.type == "dedicated"
}

resource "yandex_ydb_database_serverless" "this" {
  count = local.is_serverless ? 1 : 0

  name        = var.managed.name
  folder_id   = var.managed.folder_id
  location_id = var.managed.location_id

  # serverless_database is the optional tuning block; emit only when the
  # caller asked for a non-default throttling cap, otherwise let YC apply
  # its defaults.
  dynamic "serverless_database" {
    for_each = var.managed.throttling_rcu_limit > 0 ? [var.managed.throttling_rcu_limit] : []
    content {
      throttling_rcu_limit = serverless_database.value
    }
  }
}

resource "yandex_ydb_database_dedicated" "this" {
  count = local.is_dedicated ? 1 : 0

  name        = var.managed.name
  folder_id   = var.managed.folder_id
  location_id = var.managed.location_id

  network_id = var.networking.external_id
  subnet_ids = [yandex_vpc_subnet.subnet.id]

  resource_preset_id = var.managed.resource_preset_id

  scale_policy {
    fixed_scale {
      size = 1
    }
  }

  storage_config {
    group_count     = var.managed.storage_groups
    storage_type_id = var.managed.storage_type_id
  }
}
