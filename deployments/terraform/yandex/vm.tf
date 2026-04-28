locals {
  # Flatten (vm, secondary_disk) pairs into a map keyed by "<vm>:<device_name>"
  # so each pair becomes its own yandex_compute_disk resource and the disk can
  # then be attached to its parent VM.
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
  for_each    = var.compute.vms
  name        = each.key
  platform_id = var.compute.platform_id
  network_interface {
    subnet_id          = yandex_vpc_subnet.subnet.id
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
  # Attach raw block devices. device_name becomes the virtio serial in-guest,
  # so the agent can find them at /dev/disk/by-id/virtio-<device_name>.
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
}
