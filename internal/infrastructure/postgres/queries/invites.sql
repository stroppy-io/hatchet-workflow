-- name: InsertInvite :exec
INSERT INTO tenant_invites (id, tenant_id, email, role, invited_by, message, expires_at)
VALUES (@id, @tenant_id, @email, @role, @invited_by, @message, @expires_at);

-- name: InviteByID :one
SELECT i.id, i.tenant_id, i.email, i.role, i.status, i.invited_by, i.message, i.created_at, i.expires_at,
       COALESCE(p.display_name, '') AS inviter_name, COALESCE(p.avatar, '') AS inviter_avatar
FROM tenant_invites i
LEFT JOIN profiles p ON p.id = i.invited_by
WHERE i.id = @id;

-- name: PendingInvitesOfTenant :many
SELECT i.id, i.tenant_id, i.email, i.role, i.status, i.invited_by, i.message, i.created_at, i.expires_at,
       COALESCE(p.display_name, '') AS inviter_name, COALESCE(p.avatar, '') AS inviter_avatar
FROM tenant_invites i
LEFT JOIN profiles p ON p.id = i.invited_by
WHERE i.tenant_id = @tenant_id AND i.status = 'pending' AND i.expires_at > now()
ORDER BY i.created_at;

-- name: PendingInvitesForEmail :many
SELECT i.id, i.tenant_id, i.email, i.role, i.status, i.invited_by, i.message, i.created_at, i.expires_at,
       COALESCE(p.display_name, '') AS inviter_name, COALESCE(p.avatar, '') AS inviter_avatar
FROM tenant_invites i
LEFT JOIN profiles p ON p.id = i.invited_by
WHERE i.email = lower(@email) AND i.status = 'pending' AND i.expires_at > now()
ORDER BY i.created_at;

-- name: ResolveInvite :execrows
-- Only a pending invite resolves; a second accept/decline is a no-op.
UPDATE tenant_invites SET status = @status, resolved_at = now()
WHERE id = @id AND status = 'pending';

-- name: ExpireInvites :execrows
UPDATE tenant_invites SET status = 'expired', resolved_at = now()
WHERE status = 'pending' AND expires_at <= now();
