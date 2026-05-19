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

  # Auto-scale is a preview feature gated by an explicit label per the
  # provider docs. Stamp the label automatically when auto_scale is set so
  # callers don't have to know the magic.
  labels = var.managed.auto_scale == null ? {} : { enable_autoscaling = "1" }

  scale_policy {
    dynamic "fixed_scale" {
      for_each = var.managed.auto_scale == null ? [var.managed.node_count] : []
      content {
        size = fixed_scale.value
      }
    }
    dynamic "auto_scale" {
      for_each = var.managed.auto_scale == null ? [] : [var.managed.auto_scale]
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
    group_count     = var.managed.storage_groups
    storage_type_id = var.managed.storage_type_id
  }
}
