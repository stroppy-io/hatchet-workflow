# Service account attached to the stroppy client VM. The patched stroppy
# ydb driver (pkg/driver/ydb/driver.go) falls back to yc.WithCredentials() +
# yc.WithInternalCA() — both pull from the YC metadata service, which only
# works when a service account is attached to the VM.
resource "yandex_iam_service_account" "stroppy" {
  name        = "${var.managed.name}-sa"
  description = "Stroppy client SA — used by patched ydb driver to fetch SA token and YC internal CA from the VM metadata service"
  folder_id   = var.managed.folder_id
}

# ydb.editor is the minimum role that allows DDL + DML on a managed YDB
# database. Mentioned explicitly by the user as the gate for the metadata
# fallback to work.
resource "yandex_resourcemanager_folder_iam_member" "stroppy_ydb_editor" {
  folder_id = var.managed.folder_id
  role      = "ydb.editor"
  member    = "serviceAccount:${yandex_iam_service_account.stroppy.id}"
}
