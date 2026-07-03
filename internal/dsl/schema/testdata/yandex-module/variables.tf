variable "zone" {
  type        = string
  default     = "ru-central1-a"
  description = "YC zone"
}
variable "stroppy_machine_ext" {
  type = object({
    platform_id   = optional(string, "standard-v3")
    core_fraction = optional(number, 100)
    preemptible   = optional(bool, false)
    disk_type     = optional(string)
  })
}
