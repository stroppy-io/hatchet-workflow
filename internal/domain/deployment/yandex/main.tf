terraform {
  required_providers {
    yandex = {
      source = "yandex-cloud/yandex"
    }
  }
  required_version = ">= 0.13"
}

provider "yandex" {
  zone = ""
  //YC_TOKEN
  //YC_CLOUD_ID
  //YC_FOLDER_ID

  # token     = var.cloud-settings.token # can be found with yc init // YC_TOKEN
  # zone      = var.cloud-settings.zone
  # cloud_id  = var.cloud-settings.cloud_id // YC_FOLDER_ID
  # folder_id = var.cloud-settings.folder_id // YC_ENDPOINT
}
