locals {
  secondary_disk_list = flatten([
    for vm_name, vm in var.compute.vms : [
      for d in vm.secondary_disks : {
        vm_name     = vm_name
        device_name = d.device_name
        size        = d.size_gb
        type        = d.type
        zone        = vm.zone != "" ? vm.zone : var.network.zone
      }
    ]
  ])

  secondary_disks = {
    for d in local.secondary_disk_list :
    "${d.vm_name}:${d.device_name}" => d
  }
}

resource "yandex_compute_disk" "secondary" {
  for_each = local.secondary_disks
  name     = "${each.value.vm_name}-${each.value.device_name}"
  size     = each.value.size
  type     = each.value.type
  zone     = each.value.zone
}

resource "yandex_compute_instance" "vms" {
  for_each                  = var.compute.vms
  name                      = each.key
  zone                      = each.value.zone != "" ? each.value.zone : var.network.zone
  platform_id               = var.compute.platform_id
  network_acceleration_type = each.value.network_acceleration
  service_account_id        = try(yandex_iam_service_account.stroppy[0].id, null)

  network_interface {
    subnet_id          = yandex_vpc_subnet.subnet[each.value.zone != "" ? each.value.zone : var.network.zone].id
    nat                = each.value.public_ip
    ip_address         = each.value.internal_ip == "auto" ? null : each.value.internal_ip
    security_group_ids = [yandex_vpc_security_group.security-group.id]
  }

  resources {
    cores  = each.value.cores
    memory = each.value.memory_gb
  }

  boot_disk {
    initialize_params {
      image_id = var.compute.image_id
      size     = each.value.boot_disk_gb
      type     = each.value.boot_disk_type
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
