-- name: CreatePackage :exec
INSERT INTO packages (id, tenant_id, name, description, db_kind, db_version, is_builtin, apt_packages, pre_install, custom_repo, custom_repo_key, deb_filename, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW());

-- name: GetPackage :one
SELECT id, tenant_id, name, description, db_kind, db_version, is_builtin,
       apt_packages, pre_install, custom_repo, custom_repo_key,
       deb_filename, created_at, updated_at
FROM packages WHERE id = $1 AND tenant_id = $2;

-- name: ListPackages :many
SELECT id, tenant_id, name, description, db_kind, db_version, is_builtin,
       apt_packages, pre_install, custom_repo, custom_repo_key,
       deb_filename, created_at, updated_at
FROM packages WHERE tenant_id = $1 ORDER BY is_builtin DESC, name;

-- name: ListPackagesByKind :many
SELECT id, tenant_id, name, description, db_kind, db_version, is_builtin,
       apt_packages, pre_install, custom_repo, custom_repo_key,
       deb_filename, created_at, updated_at
FROM packages WHERE tenant_id = $1 AND db_kind = $2 ORDER BY is_builtin DESC, name;

-- name: ListPackagesByKindVersion :many
SELECT id, tenant_id, name, description, db_kind, db_version, is_builtin,
       apt_packages, pre_install, custom_repo, custom_repo_key,
       deb_filename, created_at, updated_at
FROM packages WHERE tenant_id = $1 AND db_kind = $2 AND db_version = $3 ORDER BY is_builtin DESC, name;

-- name: UpdatePackage :exec
UPDATE packages SET name = $3, description = $4, db_kind = $5, db_version = $6,
       apt_packages = $7, pre_install = $8, custom_repo = $9, custom_repo_key = $10,
       deb_filename = $11, updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND is_builtin = FALSE;

-- name: UpdatePackageDebData :exec
UPDATE packages SET deb_data = $3, deb_filename = $4, updated_at = NOW()
WHERE id = $1 AND tenant_id = $2;

-- name: GetPackageDebData :one
SELECT deb_data, deb_filename FROM packages WHERE id = $1 AND tenant_id = $2;

-- name: DeletePackage :exec
DELETE FROM packages WHERE id = $1 AND tenant_id = $2 AND is_builtin = FALSE;
