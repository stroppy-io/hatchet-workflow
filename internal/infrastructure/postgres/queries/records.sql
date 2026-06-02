-- ===== test_run_records (store.go TestRunRepo) =====

-- name: CreateTestRunRecord :exec
insert into test_run_records (id, tenant_id, created_at, updated_at, data)
values (@id, @tenant_id, now(), now(), @data);

-- name: GetTestRunRecord :one
select data from test_run_records where tenant_id = @tenant_id and id = @id;

-- name: ListTestRunRecords :many
select data from test_run_records where tenant_id = @tenant_id;

-- name: UpdateTestRunRecord :execrows
update test_run_records set data = @data, updated_at = now()
where tenant_id = @tenant_id and id = @id;

-- name: DeleteTestRunRecord :execrows
delete from test_run_records where tenant_id = @tenant_id and id = @id;

-- name: ExistsTestRunRecord :one
select 1 from test_run_records where tenant_id = @tenant_id and id = @id;

-- ===== test_wizard_drafts (store.go DraftRepo) =====

-- name: CreateTestWizardDraft :exec
insert into test_wizard_drafts (id, tenant_id, created_at, updated_at, data)
values (@id, @tenant_id, now(), now(), @data);

-- name: GetTestWizardDraft :one
select data from test_wizard_drafts where tenant_id = @tenant_id and id = @id;

-- name: ListTestWizardDrafts :many
select data from test_wizard_drafts where tenant_id = @tenant_id;

-- name: UpdateTestWizardDraft :execrows
update test_wizard_drafts set data = @data, updated_at = now()
where tenant_id = @tenant_id and id = @id;

-- name: DeleteTestWizardDraft :execrows
delete from test_wizard_drafts where tenant_id = @tenant_id and id = @id;

-- ===== test_preset_records (store.go PresetRepo + records.go TestPresetRepo) =====

-- name: CreateTestPresetRecord :exec
insert into test_preset_records (id, tenant_id, created_at, updated_at, data)
values (@id, @tenant_id, now(), now(), @data);

-- name: GetTestPresetRecord :one
select data from test_preset_records where tenant_id = @tenant_id and id = @id;

-- name: ListTestPresetRecords :many
select data from test_preset_records where tenant_id = @tenant_id;

-- name: UpdateTestPresetRecord :execrows
update test_preset_records set data = @data, updated_at = now()
where tenant_id = @tenant_id and id = @id;

-- name: DeleteTestPresetRecord :execrows
delete from test_preset_records where tenant_id = @tenant_id and id = @id;

-- ===== suite_records (records.go SuiteRepo) =====

-- name: CreateSuiteRecord :exec
insert into suite_records (id, tenant_id, created_at, updated_at, data)
values (@id, @tenant_id, now(), now(), @data);

-- name: GetSuiteRecord :one
select data from suite_records where tenant_id = @tenant_id and id = @id;

-- name: ListSuiteRecords :many
select data from suite_records where tenant_id = @tenant_id;

-- name: UpdateSuiteRecord :execrows
update suite_records set data = @data, updated_at = now()
where tenant_id = @tenant_id and id = @id;

-- name: DeleteSuiteRecord :execrows
delete from suite_records where tenant_id = @tenant_id and id = @id;

-- ===== suite_run_records (records.go SuiteRunRepo) =====

-- name: CreateSuiteRunRecord :exec
insert into suite_run_records (id, tenant_id, created_at, updated_at, data)
values (@id, @tenant_id, now(), now(), @data);

-- name: GetSuiteRunRecord :one
select data from suite_run_records where id = @id;

-- name: ListSuiteRunRecords :many
select data from suite_run_records where tenant_id = @tenant_id;

-- name: UpdateSuiteRunRecord :execrows
update suite_run_records set tenant_id = @tenant_id, data = @data, updated_at = now()
where id = @id;

-- name: DeleteSuiteRunRecord :execrows
delete from suite_run_records where id = @id;

-- ===== suite_wizard_drafts (records.go SuiteDraftRepo) =====

-- name: CreateSuiteWizardDraft :exec
insert into suite_wizard_drafts (id, tenant_id, created_at, updated_at, data)
values (@id, @tenant_id, now(), now(), @data);

-- name: GetSuiteWizardDraft :one
select data from suite_wizard_drafts where tenant_id = @tenant_id and id = @id;

-- name: ListSuiteWizardDrafts :many
select data from suite_wizard_drafts where tenant_id = @tenant_id;

-- name: UpdateSuiteWizardDraft :execrows
update suite_wizard_drafts set data = @data, updated_at = now()
where tenant_id = @tenant_id and id = @id;

-- name: DeleteSuiteWizardDraft :execrows
delete from suite_wizard_drafts where tenant_id = @tenant_id and id = @id;

-- ===== share_records (records.go ShareRepo) =====

-- name: CreateShareRecord :exec
insert into share_records (id, tenant_id, token, created_at, updated_at, data)
values (@id, @tenant_id, @token, now(), now(), @data);

-- name: GetShareRecord :one
select data from share_records where tenant_id = @tenant_id and id = @id;

-- name: GetShareRecordByToken :one
select data from share_records where token = @token;

-- name: ListShareRecords :many
select data from share_records where tenant_id = @tenant_id;

-- name: UpdateShareRecord :execrows
update share_records set token = @token, data = @data, updated_at = now()
where tenant_id = @tenant_id and id = @id;

-- name: DeleteShareRecord :execrows
delete from share_records where tenant_id = @tenant_id and id = @id;

-- ===== favorite_records (records.go FavoriteRepo) =====

-- name: CreateFavoriteRecord :exec
insert into favorite_records (id, account_id, tenant_id, kind, target_id, created_at, updated_at, data)
values (@id, @account_id, @tenant_id, @kind, @target_id, now(), now(), @data);

-- name: GetFavoriteRecord :one
select data from favorite_records
where account_id = @account_id and kind = @kind and target_id = @target_id;

-- name: DeleteFavoriteRecord :execrows
delete from favorite_records
where account_id = @account_id and kind = @kind and target_id = @target_id;

-- name: ListFavoriteRecords :many
select data from favorite_records
where account_id = @account_id and tenant_id = @tenant_id
  and (@kind = 0 or kind = @kind);

-- ===== package_records (records.go PackageRepo) =====

-- name: CreatePackageRecord :exec
insert into package_records (id, tenant_id, created_at, updated_at, data)
values (@id, @tenant_id, now(), now(), @data);

-- name: GetPackageRecord :one
select data from package_records where tenant_id = @tenant_id and id = @id;

-- name: ListPackageRecords :many
select data from package_records where tenant_id = @tenant_id;

-- name: UpdatePackageRecord :execrows
update package_records set data = @data, updated_at = now()
where tenant_id = @tenant_id and id = @id;

-- name: DeletePackageRecord :execrows
delete from package_records where tenant_id = @tenant_id and id = @id;

-- ===== database_preset_records (records.go DatabasePresetRepo) =====

-- name: CreateDatabasePresetRecord :exec
insert into database_preset_records (id, tenant_id, created_at, updated_at, data)
values (@id, @tenant_id, now(), now(), @data);

-- name: GetDatabasePresetRecord :one
select data from database_preset_records where tenant_id = @tenant_id and id = @id;

-- name: ListDatabasePresetRecords :many
select data from database_preset_records where tenant_id = @tenant_id;

-- name: UpdateDatabasePresetRecord :execrows
update database_preset_records set data = @data, updated_at = now()
where tenant_id = @tenant_id and id = @id;

-- name: DeleteDatabasePresetRecord :execrows
delete from database_preset_records where tenant_id = @tenant_id and id = @id;

-- ===== workload_preset_records (records.go WorkloadPresetRepo) =====

-- name: CreateWorkloadPresetRecord :exec
insert into workload_preset_records (id, tenant_id, created_at, updated_at, data)
values (@id, @tenant_id, now(), now(), @data);

-- name: GetWorkloadPresetRecord :one
select data from workload_preset_records where tenant_id = @tenant_id and id = @id;

-- name: ListWorkloadPresetRecords :many
select data from workload_preset_records where tenant_id = @tenant_id;

-- name: UpdateWorkloadPresetRecord :execrows
update workload_preset_records set data = @data, updated_at = now()
where tenant_id = @tenant_id and id = @id;

-- name: DeleteWorkloadPresetRecord :execrows
delete from workload_preset_records where tenant_id = @tenant_id and id = @id;

-- ===== platform_settings (settings.go SettingsRepo) =====

-- name: GetPlatformSettings :one
select data from platform_settings where id = @id;

-- name: UpsertPlatformSettings :exec
insert into platform_settings (id, updated_at, data)
values (@id, now(), @data)
on conflict (id) do update set updated_at = now(), data = excluded.data;

-- ===== tenant_settings_records (records.go TenantSettingsRepo) =====

-- name: GetTenantSettingsRecord :one
select data from tenant_settings_records where tenant_id = @tenant_id;

-- name: UpsertTenantSettingsRecord :exec
insert into tenant_settings_records (tenant_id, updated_at, data)
values (@tenant_id, now(), @data)
on conflict (tenant_id) do update set updated_at = now(), data = excluded.data;
