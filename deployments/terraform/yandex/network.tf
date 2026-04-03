resource "yandex_vpc_subnet" "subnet" {
  name           = var.networking.name
  zone           = var.networking.zone
  v4_cidr_blocks = [var.networking.cidr]
  network_id     = var.networking.external_id
}

resource "yandex_vpc_security_group" "security-group" {
  name        = "${var.networking.name}-sec-grp"
  description = "Security group for stroppy VMs"
  network_id  = var.networking.external_id
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
    v4_cidr_blocks = concat(yandex_vpc_subnet.subnet.v4_cidr_blocks)
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
  egress {
    protocol       = "ANY"
    v4_cidr_blocks = ["0.0.0.0/0"]
    from_port      = 0
    to_port        = 65535
  }
}
