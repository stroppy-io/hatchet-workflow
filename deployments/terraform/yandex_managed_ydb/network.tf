locals {
  # Yandex Cloud Managed YDB (dedicated mode) requires at least one subnet
  # per availability zone, even though the database internally picks where
  # to place its nodes. Carve var.networking.cidr (a /16) into a /20 per
  # zone using cidrsubnet — gives ~4k IPs per zone, plenty for the static
  # Database/Storage layout YC schedules.
  zones = ["ru-central1-a", "ru-central1-b", "ru-central1-d"]
  zone_subnets = {
    for idx, zone in local.zones :
    zone => cidrsubnet(var.networking.cidr, 4, idx)
  }
}

resource "yandex_vpc_subnet" "zone" {
  for_each       = local.zone_subnets
  name           = "${var.networking.name}-${each.key}"
  zone           = each.key
  v4_cidr_blocks = [each.value]
  network_id     = var.networking.external_id
}

resource "yandex_vpc_security_group" "security-group" {
  name        = "${var.networking.name}-sec-grp"
  description = "Security group for stroppy client VMs against managed YDB"
  network_id  = var.networking.external_id
  ingress {
    protocol          = "ANY"
    predefined_target = "self_security_group"
    from_port         = 0
    to_port           = 65535
  }
  ingress {
    protocol       = "ANY"
    v4_cidr_blocks = [for s in yandex_vpc_subnet.zone : s.v4_cidr_blocks[0]]
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
