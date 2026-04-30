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
  # Dedicated YDB requires a subnet per availability zone — the API rejects
  # creates that don't cover every zone in the region with at least one
  # subnet (validated server-side, not by the provider).
  subnet_ids = [for s in yandex_vpc_subnet.zone : s.id]

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
