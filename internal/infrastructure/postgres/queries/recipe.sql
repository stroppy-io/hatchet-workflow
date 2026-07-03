-- ===== recipe_records (records.go RecipeRepo) =====

-- name: CreateRecipeRecord :exec
insert into recipe_records (id, tenant_id, name, version, created_at, updated_at, data)
values (@id, @tenant_id, @name, @version, now(), now(), @data);

-- name: GetRecipeRecord :one
select data from recipe_records where tenant_id = @tenant_id and id = @id;

-- name: ListRecipeRecords :many
select data from recipe_records where tenant_id = @tenant_id order by name, version;

-- name: GetLatestRecipeRecordByName :one
select data from recipe_records where tenant_id = @tenant_id and name = @name
order by version desc limit 1;

-- name: DeleteRecipeRecord :execrows
delete from recipe_records where tenant_id = @tenant_id and id = @id;
