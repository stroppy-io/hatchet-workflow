resource "yandex_iam_service_account" "stroppy" {
  count       = local.managed_ydb_enabled ? 1 : 0
  name        = local.managed_ydb_service_account_name != "" ? local.managed_ydb_service_account_name : "${local.managed_ydb_name}-stroppy-sa"
  description = "Stroppy client service account for Managed YDB access"
  folder_id   = local.managed_ydb_folder_id
}

resource "yandex_resourcemanager_folder_iam_member" "stroppy_ydb_editor" {
  count     = local.managed_ydb_enabled ? 1 : 0
  folder_id = local.managed_ydb_folder_id
  role      = "ydb.editor"
  member    = "serviceAccount:${yandex_iam_service_account.stroppy[0].id}"
}
