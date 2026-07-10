locals {
  # vms adapts the standard stroppy_nodes contract input (provider-agnostic:
  # id/group/cpu/ram_gb/disk/ext, see variables.tf's stroppy_nodes doc
  # comment) onto this module's own VM shape. Each node's single `disk` (if
  # any) becomes the module's one supported secondary/data disk -- the
  # stroppy_nodes contract, like MachineGroup.Disks upstream, only ever
  # carries at most one data disk per node (see terraform.go's
  # tfNodesForGroup, which only reads disks[0]).
  #
  # ext-declared fields (see stroppy_machine_ext) override this module's
  # compute-wide defaults per node; a node with no matching ext key falls
  # back to `compute`.
  vms = {
    for n in var.stroppy_nodes : n.id => {
      cores                = n.cpu
      memory_gb            = n.ram_gb
      boot_disk_gb         = var.compute.boot_disk_gb
      boot_disk_type       = var.compute.boot_disk_type
      platform_id          = try(n.ext.platform_id, "") != "" ? n.ext.platform_id : var.compute.platform_id
      image_id             = try(n.ext.image_id, "") != "" ? n.ext.image_id : var.compute.image_id
      zone                 = try(n.ext.zone, "")
      internal_ip          = "auto"
      public_ip            = try(n.ext.public_ip, false)
      user_data            = try(n.ext.user_data, "")
      network_acceleration = try(n.ext.network_acceleration, "standard")
      secondary_disks = n.disk == null ? [] : [{
        device_name = "data"
        size_gb     = n.disk.size_gb
        type        = n.disk.type
      }]
    }
  }

  secondary_disk_list = flatten([
    for vm_name, vm in local.vms : [
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
  for_each                  = local.vms
  name                      = each.key
  zone                      = each.value.zone != "" ? each.value.zone : var.network.zone
  platform_id               = each.value.platform_id
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
      image_id = each.value.image_id
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
