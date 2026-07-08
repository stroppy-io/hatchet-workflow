-- ===== catalog_entries (catalog.go CatalogEntryRepo) =====

-- name: CreateCatalogEntry :exec
insert into catalog_entries (id, level, tenant_id, kind, slug, version, origin, source_entry_id, created_at, updated_at, data)
values (@id, @level, @tenant_id, @kind, @slug, @version, @origin, @source_entry_id, now(), now(), @data);

-- name: GetCatalogEntry :one
select data from catalog_entries where level = @level and tenant_id = @tenant_id and id = @id;

-- name: ListCatalogEntries :many
select data from catalog_entries where level = @level and tenant_id = @tenant_id and kind = @kind order by slug, version;

-- name: GetLatestCatalogEntryBySlug :one
select data from catalog_entries
where level = @level and tenant_id = @tenant_id and kind = @kind and slug = @slug
order by version desc limit 1;

-- name: GetCatalogEntryBySlugVersion :one
select data from catalog_entries
where level = @level and tenant_id = @tenant_id and kind = @kind and slug = @slug and version = @version;

-- name: ListCatalogEntriesBySource :many
select data from catalog_entries where source_entry_id = @source_entry_id;

-- name: UpdateCatalogEntry :execrows
update catalog_entries set data = @data, updated_at = now()
where level = @level and tenant_id = @tenant_id and id = @id;

-- name: DeleteCatalogEntry :execrows
delete from catalog_entries where level = @level and tenant_id = @tenant_id and id = @id;
