locals {
  secondary_disks = merge([
    for vm_name, vm in var.compute.vms : {
      for d in vm.secondary_disks :
      "${vm_name}:${d.device_name}" => {
        vm_name     = vm_name
        device_name = d.device_name
        size        = d.size_gb
        type        = d.type
      }
    }
  ]...)
}

resource "yandex_compute_disk" "secondary" {
  for_each = local.secondary_disks
  name     = "${each.value.vm_name}-${each.value.device_name}"
  size     = each.value.size
  type     = each.value.type
}

resource "yandex_compute_instance" "vms" {
  for_each                  = var.compute.vms
  name                      = each.key
  platform_id               = var.compute.platform_id
  network_acceleration_type = each.value.network_acceleration_type

  # Attaching the SA is the whole point of this module — without it the
  # patched stroppy ydb driver's metadata fallback can't fetch a token, and
  # auth against managed YDB fails before the first request.
  service_account_id = yandex_iam_service_account.stroppy.id

  network_interface {
    # Place the client VM in the subnet matching var.networking.zone — the
    # zone that the user picked for the run. Falls back to ru-central1-b
    # (legacy default) if the picked zone has no matching subnet.
    subnet_id          = yandex_vpc_subnet.zone[var.networking.zone].id
    nat                = each.value.has_public_ip
    ip_address         = each.value.internal_ip
    security_group_ids = [yandex_vpc_security_group.security-group.id]
  }
  resources {
    cores  = each.value.cores
    memory = each.value.memory
  }
  boot_disk {
    initialize_params {
      image_id = var.compute.image_id
      size     = each.value.disk_size
      type     = each.value.disk_type
    }
  }
  dynamic "secondary_disk" {
    for_each = each.value.secondary_disks
    content {
      disk_id     = yandex_compute_disk.secondary["${each.key}:${secondary_disk.value.device_name}"].id
      device_name = secondary_disk.value.device_name
      auto_delete = true
    }
  }
  metadata = {
    user-data          = each.value.user_data
    serial-port-enable = var.compute.serial_port_enable
  }

  depends_on = [
    yandex_resourcemanager_folder_iam_member.stroppy_ydb_editor,
  ]
}
