-- ===== recipe_records (records.go RecipeRepo) =====

-- name: CreateRecipeRecord :exec
insert into recipe_records (id, tenant_id, name, version, created_at, updated_at, source_ref, data)
values (@id, @tenant_id, @name, @version, now(), now(), @source_ref, @data);

-- name: GetRecipeRecord :one
select source_ref, data from recipe_records where tenant_id = @tenant_id and id = @id;

-- name: ListRecipeRecords :many
select source_ref, data from recipe_records where tenant_id = @tenant_id order by name, version;

-- name: GetLatestRecipeRecordByName :one
select source_ref, data from recipe_records where tenant_id = @tenant_id and name = @name
order by version desc limit 1;

-- name: DeleteRecipeRecord :execrows
delete from recipe_records where tenant_id = @tenant_id and id = @id;

-- name: UpdateRecipeRecordSourceRef :execrows
-- Persists a healed/materialized source_ref onto an existing row, and
-- refreshes data (the caller passes rec with Bundle.Files already stripped)
-- so the row's git ref and its protojson blob change atomically — see
-- healRecipeSourceRef's doc for why data must stop embedding file bytes the
-- moment a row's ref is healed.
update recipe_records set source_ref = @source_ref, data = @data, updated_at = now()
where tenant_id = @tenant_id and id = @id;
