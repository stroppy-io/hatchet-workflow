-- name: AdminTenants :many
SELECT t.id, t.slug, t.name, t.description, t.public_name, t.status, t.owner_id, t.graphene_namespace, t.created_at, t.updated_at,
       (SELECT count(*) FROM tenant_members m WHERE m.tenant_id = t.id)::int AS member_count
FROM tenants t
WHERE t.deleted_at IS NULL
  AND (@search::text = '' OR t.name ILIKE '%' || @search::text || '%' OR t.slug ILIKE '%' || @search::text || '%')
  AND (@status::text = '' OR t.status = @status::text)
ORDER BY t.created_at DESC LIMIT @lim OFFSET @off;

-- name: TenantStatusCounts :one
SELECT count(*) FILTER (WHERE status = 'active')::int AS active,
       count(*) FILTER (WHERE status = 'suspended')::int AS suspended,
       count(*) FILTER (WHERE status = 'orphaned')::int AS orphaned
FROM tenants WHERE deleted_at IS NULL;

-- name: TenantSuspendedReason :one
SELECT suspended_reason FROM tenants WHERE id = @id;

-- name: SetTenantSuspended :execrows
UPDATE tenants SET status = @status, suspended_reason = @reason, updated_at = now() WHERE id = @id AND deleted_at IS NULL;

-- name: TenantCounters :one
SELECT (SELECT count(*) FROM tenant_members m WHERE m.tenant_id = @tenant_id)::int AS members,
       (SELECT count(*) FROM runs r WHERE r.tenant_id = @tenant_id AND r.deleted_at IS NULL)::int AS runs_total,
       (SELECT count(*) FROM runs r WHERE r.tenant_id = @tenant_id AND r.deleted_at IS NULL AND r.status IN ('pending', 'running', 'cancelling'))::int AS runs_running,
       (SELECT count(*) FROM runs r WHERE r.tenant_id = @tenant_id AND r.deleted_at IS NULL AND r.stand_kept)::int AS kept_stands,
       (SELECT count(*) FROM provider_profiles p WHERE p.tenant_id = @tenant_id AND p.deleted_at IS NULL)::int AS providers,
       (SELECT max(a.at) FROM audit_log a WHERE a.tenant_id = @tenant_id) AS last_activity_at;

-- name: AdminProfiles :many
SELECT p.id, p.email, p.display_name, p.avatar, p.is_platform_admin, p.preferences, p.notifications, p.created_at, p.updated_at,
       (SELECT count(*) FROM tenant_members m WHERE m.user_id = p.id)::int AS memberships,
       (SELECT t.id FROM tenants t WHERE t.owner_id = p.id AND t.deleted_at IS NULL LIMIT 1) AS owned_tenant_id,
       (SELECT t.name FROM tenants t WHERE t.owner_id = p.id AND t.deleted_at IS NULL LIMIT 1) AS owned_tenant_name
FROM profiles p
WHERE (@search::text = '' OR p.email ILIKE '%' || @search::text || '%' OR p.display_name ILIKE '%' || @search::text || '%')
  AND (NOT @only_admins::boolean OR p.is_platform_admin)
ORDER BY p.created_at DESC LIMIT @lim OFFSET @off;

-- name: SetPlatformAdmin :execrows
UPDATE profiles SET is_platform_admin = @is_platform_admin, updated_at = now() WHERE id = @id;

-- name: AdminRuns :many
SELECT r.id, r.tenant_id, r.name, r.status, r.phase, r.status_reason, r.trigger, r.suite_run_id, r.cell_id, r.schedule_id, r.parent_run_id, r.test_id, r.test_name, r.author_id,
       r.snapshot, r.run_spec, r.summary, r.result, r.runtime_state, r.last_event_id, r.rating_tenant, r.rating_global, r.keep, r.keep_until, r.stand_kept, r.notes, r.labels,
       r.graphene_namespace, r.graphene_run_id, r.pipeline_revision, r.tps, r.duration_seconds, r.created_at, r.started_at, r.finished_at, r.updated_at, r.deleted_at,
       t.slug AS tenant_slug, t.name AS tenant_name
FROM runs r JOIN tenants t ON t.id = r.tenant_id
WHERE r.deleted_at IS NULL
  AND (cardinality(@statuses::text[]) = 0 OR r.status = ANY(@statuses::text[]))
  AND (@tenant::text = '' OR t.slug = @tenant::text)
ORDER BY r.created_at DESC LIMIT @lim OFFSET @off;

-- name: AdminRunCounts :one
SELECT count(*) FILTER (WHERE status IN ('running', 'cancelling'))::int AS running,
       count(*) FILTER (WHERE status = 'pending')::int AS pending,
       count(*) FILTER (WHERE stand_kept)::int AS kept_stands
FROM runs WHERE deleted_at IS NULL;

-- name: AuditAll :many
SELECT a.id, a.at, a.tenant_id, a.actor_kind, a.actor_id, a.actor_name, a.action, a.target_kind, a.target_id, a.target_name, a.details, a.request_id
FROM audit_log a LEFT JOIN tenants t ON t.id = a.tenant_id
WHERE (@before_id::bigint = 0 OR a.id < @before_id::bigint)
  AND (@tenant::text = '' OR t.slug = @tenant::text)
  AND (@action::text = '' OR a.action = @action::text)
  AND (@actor_id::text = '' OR a.actor_id = @actor_id::text)
  AND (@since::timestamptz IS NULL OR a.at >= @since::timestamptz)
ORDER BY a.id DESC LIMIT @lim;

-- name: SystemSettingsRow :one
INSERT INTO system_settings (id) VALUES (1) ON CONFLICT (id) DO UPDATE SET id = 1
RETURNING id, tenant_creation, public_rating_enabled, examples_enabled, default_limits, run_retention_max_days, stroppy_catalog, updated_at, updated_by;

-- name: UpdateSystemSettings :one
UPDATE system_settings
SET tenant_creation        = COALESCE(@tenant_creation::text, tenant_creation),
    public_rating_enabled  = COALESCE(@public_rating_enabled::boolean, public_rating_enabled),
    examples_enabled       = COALESCE(@examples_enabled::boolean, examples_enabled),
    default_limits         = CASE WHEN @set_default_limits::boolean THEN @default_limits::jsonb ELSE default_limits END,
    run_retention_max_days = COALESCE(@run_retention_max_days::int, run_retention_max_days),
    stroppy_catalog        = CASE WHEN @set_catalog::boolean THEN @stroppy_catalog::jsonb ELSE stroppy_catalog END,
    updated_at             = now(),
    updated_by             = @updated_by
WHERE id = 1
RETURNING id, tenant_creation, public_rating_enabled, examples_enabled, default_limits, run_retention_max_days, stroppy_catalog, updated_at, updated_by;
