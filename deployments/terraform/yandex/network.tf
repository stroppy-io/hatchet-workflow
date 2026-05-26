locals {
  managed_ydb_enabled    = var.managed_ydb != null
  managed_ydb_serverless = try(var.managed_ydb.serverless != null, false)
  managed_ydb_dedicated  = try(var.managed_ydb.dedicated != null, false)

  managed_ydb_name                 = local.managed_ydb_enabled ? var.managed_ydb.name : ""
  managed_ydb_folder_id            = local.managed_ydb_enabled ? var.managed_ydb.folder_id : ""
  managed_ydb_location_id          = local.managed_ydb_enabled ? var.managed_ydb.location_id : ""
  managed_ydb_deletion_protection  = local.managed_ydb_enabled ? var.managed_ydb.deletion_protection : false
  managed_ydb_labels_input         = local.managed_ydb_enabled ? var.managed_ydb.labels : {}
  managed_ydb_service_account_name = local.managed_ydb_enabled ? var.managed_ydb.service_account_name : ""

  managed_ydb_serverless_config = local.managed_ydb_serverless ? var.managed_ydb.serverless : {
    enable_throttling_rcu_limit = false
    throttling_rcu_limit        = 0
    provisioned_rcu_limit       = 0
    storage_size_limit          = 0
  }

  managed_ydb_dedicated_config = local.managed_ydb_dedicated ? var.managed_ydb.dedicated : {
    resource_preset_id = ""
    scale_policy = {
      fixed = {
        size = 1
      }
      auto = null
    }
    storage_config = {
      group_count     = 1
      storage_type_id = "ssd"
    }
    subnet_ids         = []
    security_group_ids = []
    assign_public_ips  = false
  }

  managed_zones = length(var.network.managed_zones) > 0 ? var.network.managed_zones : [
    "ru-central1-a",
    "ru-central1-b",
    "ru-central1-d",
  ]

  dedicated_subnets = {
    for idx, zone in local.managed_zones :
    zone => {
      zone = zone
      cidr = cidrsubnet(var.network.cidr, 4, idx)
    }
  }

  fallback_subnets = {
    (var.network.zone) = {
      zone = var.network.zone
      cidr = var.network.cidr
    }
  }

  subnets = local.managed_ydb_dedicated ? local.dedicated_subnets : (
    length(var.network.subnets) > 0 ? var.network.subnets : local.fallback_subnets
  )
}

resource "yandex_vpc_subnet" "subnet" {
  for_each       = local.subnets
  name           = "${var.network.name}-${each.key}"
  zone           = each.value.zone
  v4_cidr_blocks = [each.value.cidr]
  network_id     = var.network.network_id
}

resource "yandex_vpc_security_group" "security-group" {
  name        = "${var.network.name}-sec-grp"
  description = "Security group for stroppy VMs"
  network_id  = var.network.network_id

  ingress {
    protocol          = "TCP"
    predefined_target = "loadbalancer_healthchecks"
    from_port         = 0
    to_port           = 65535
  }

  ingress {
    protocol          = "ANY"
    predefined_target = "self_security_group"
    from_port         = 0
    to_port           = 65535
  }

  ingress {
    protocol       = "ANY"
    v4_cidr_blocks = flatten([for s in yandex_vpc_subnet.subnet : s.v4_cidr_blocks])
    from_port      = 0
    to_port        = 65535
  }

  ingress {
    protocol       = "ICMP"
    v4_cidr_blocks = ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"]
  }

  ingress {
    protocol       = "TCP"
    v4_cidr_blocks = ["0.0.0.0/0"]
    from_port      = 30000
    to_port        = 32767
  }

  ingress {
    protocol       = "TCP"
    v4_cidr_blocks = ["0.0.0.0/0"]
    from_port      = 22
    to_port        = 22
  }

  egress {
    protocol       = "ANY"
    v4_cidr_blocks = ["0.0.0.0/0"]
    from_port      = 0
    to_port        = 65535
  }
}
