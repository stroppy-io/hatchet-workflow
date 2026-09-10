-- name: InsertTenant :exec
INSERT INTO tenants (id, slug, name, description, owner_id, graphene_namespace)
VALUES (@id, @slug, @name, @description, @owner_id, @graphene_namespace);

-- name: TenantBySlug :one
SELECT t.id, t.slug, t.name, t.description, t.public_name, t.status, t.owner_id, t.graphene_namespace, t.created_at, t.updated_at,
       (SELECT count(*) FROM tenant_members m WHERE m.tenant_id = t.id)::int AS member_count
FROM tenants t
WHERE t.slug = @slug AND t.deleted_at IS NULL;

-- name: TenantByID :one
SELECT t.id, t.slug, t.name, t.description, t.public_name, t.status, t.owner_id, t.graphene_namespace, t.created_at, t.updated_at,
       (SELECT count(*) FROM tenant_members m WHERE m.tenant_id = t.id)::int AS member_count
FROM tenants t
WHERE t.id = @id AND t.deleted_at IS NULL;

-- name: TenantsOfUser :many
SELECT t.id, t.slug, t.name, t.description, t.public_name, t.status, t.owner_id, t.graphene_namespace, t.created_at, t.updated_at,
       (SELECT count(*) FROM tenant_members mm WHERE mm.tenant_id = t.id)::int AS member_count,
       m.role, m.joined_at
FROM tenant_members m
JOIN tenants t ON t.id = m.tenant_id
WHERE m.user_id = @user_id AND t.deleted_at IS NULL
ORDER BY m.joined_at;

-- name: OwnedTenant :one
SELECT id, slug FROM tenants WHERE owner_id = @owner_id AND deleted_at IS NULL;

-- name: SlugTaken :one
-- Any row, deleted or not: a retired namespace keeps its name.
SELECT id FROM tenants WHERE slug = @slug LIMIT 1;

-- name: UpdateTenant :execrows
UPDATE tenants
SET name        = COALESCE(@name::text, name),
    description = COALESCE(@description::text, description),
    public_name = CASE WHEN @clear_public_name::boolean THEN NULL ELSE COALESCE(@public_name::text, public_name) END,
    updated_at  = now()
WHERE id = @id AND deleted_at IS NULL;

-- name: SetTenantOwner :execrows
UPDATE tenants SET owner_id = @owner_id, updated_at = now() WHERE id = @id AND deleted_at IS NULL;

-- name: SetTenantStatus :execrows
UPDATE tenants SET status = @status, updated_at = now() WHERE id = @id AND deleted_at IS NULL;

-- name: SoftDeleteTenant :execrows
UPDATE tenants SET deleted_at = now(), status = 'deleted', updated_at = now() WHERE id = @id AND deleted_at IS NULL;

-- name: TenantNamespaces :many
SELECT graphene_namespace FROM tenants WHERE deleted_at IS NULL ORDER BY created_at;
