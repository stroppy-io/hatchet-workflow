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

-- ===== run_records (run_store.go RunRepo) =====

-- name: CreateRunRecord :exec
insert into run_records (id, tenant_id, created_at, updated_at, data)
values (@id, @tenant_id, now(), now(), @data);

-- name: GetRunRecord :one
select data from run_records where tenant_id = @tenant_id and id = @id;

-- name: ListRunRecords :many
select data from run_records where tenant_id = @tenant_id;

-- name: UpdateRunRecord :execrows
update run_records set data = @data, updated_at = now()
where tenant_id = @tenant_id and id = @id;

-- name: DeleteRunRecord :execrows
delete from run_records where tenant_id = @tenant_id and id = @id;

-- name: ExistsRunRecord :one
select 1 from run_records where tenant_id = @tenant_id and id = @id;

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
