-- name: InsertToken :exec
INSERT INTO api_tokens (id, kind, name, prefix, secret_hash, tenant_id, role, owner_id, expires_at)
VALUES (@id, @kind, @name, @prefix, @secret_hash, @tenant_id, @role, @owner_id, @expires_at);

-- name: TokenByPrefix :one
-- Live token with its tenant slug; the verifier compares the hash itself.
SELECT a.id, a.kind, a.name, a.prefix, a.secret_hash, a.tenant_id, a.role, a.owner_id, a.expires_at, a.last_used_at, a.created_at,
       t.slug AS tenant_slug, t.name AS tenant_name
FROM api_tokens a
JOIN tenants t ON t.id = a.tenant_id AND t.deleted_at IS NULL
WHERE a.prefix = @prefix AND a.revoked_at IS NULL;

-- name: TokensOfOwner :many
SELECT a.id, a.kind, a.name, a.prefix, a.tenant_id, a.role, a.owner_id, a.expires_at, a.last_used_at, a.created_at,
       t.slug AS tenant_slug, t.name AS tenant_name
FROM api_tokens a
JOIN tenants t ON t.id = a.tenant_id
WHERE a.owner_id = @owner_id AND a.kind = 'personal' AND a.revoked_at IS NULL
ORDER BY a.created_at;

-- name: ServiceTokensOfTenant :many
SELECT a.id, a.kind, a.name, a.prefix, a.tenant_id, a.role, a.owner_id, a.expires_at, a.last_used_at, a.created_at,
       t.slug AS tenant_slug, t.name AS tenant_name
FROM api_tokens a
JOIN tenants t ON t.id = a.tenant_id
WHERE a.tenant_id = @tenant_id AND a.kind = 'service' AND a.revoked_at IS NULL
ORDER BY a.created_at;

-- name: RevokeToken :execrows
UPDATE api_tokens SET revoked_at = now()
WHERE id = @id AND revoked_at IS NULL
  AND (@owner_id::uuid IS NULL OR owner_id = @owner_id::uuid)
  AND (@tenant_id::uuid IS NULL OR tenant_id = @tenant_id::uuid);

-- name: RevokeTokensOfMember :execrows
-- Personal tokens die with the membership.
UPDATE api_tokens SET revoked_at = now()
WHERE tenant_id = @tenant_id AND owner_id = @owner_id AND kind = 'personal' AND revoked_at IS NULL;

-- name: TouchToken :exec
UPDATE api_tokens SET last_used_at = now() WHERE id = @id;
