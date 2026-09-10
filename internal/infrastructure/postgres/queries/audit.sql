-- name: InsertAudit :exec
INSERT INTO audit_log (tenant_id, actor_kind, actor_id, actor_name, action, target_kind, target_id, target_name, details, request_id)
VALUES (@tenant_id, @actor_kind, @actor_id, @actor_name, @action, @target_kind, @target_id, @target_name, @details, @request_id);

-- name: AuditOfTenant :many
-- Cursor = last id seen (descending). limit+1 rows tell has_more.
SELECT id, at, tenant_id, actor_kind, actor_id, actor_name, action, target_kind, target_id, target_name, details, request_id
FROM audit_log
WHERE tenant_id = @tenant_id
  AND (@before_id::bigint = 0 OR id < @before_id::bigint)
  AND (@action::text = '' OR action = @action::text)
  AND (@actor_id::text = '' OR actor_id = @actor_id::text)
  AND (@since::timestamptz IS NULL OR at >= @since::timestamptz)
ORDER BY id DESC
LIMIT @lim;
