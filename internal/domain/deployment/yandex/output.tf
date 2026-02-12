output "vm_ips" {
  value = {
    for _, vm in yandex_compute_instance.vms :
    vm.name => {
      id   = vm.id
      nat_ip   = vm.network_interface[0].nat_ip_address
      internal_ip   = vm.network_interface[0].ip_address
    }
  }
}
