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
  # Authentication via environment variables:
  #   YC_TOKEN, YC_CLOUD_ID, YC_FOLDER_ID
}
