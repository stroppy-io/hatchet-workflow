-- name: UpsertMember :exec
INSERT INTO tenant_members (tenant_id, user_id, role)
VALUES (@tenant_id, @user_id, @role)
ON CONFLICT (tenant_id, user_id) DO UPDATE SET role = EXCLUDED.role;

-- name: MemberRole :one
SELECT role FROM tenant_members WHERE tenant_id = @tenant_id AND user_id = @user_id;

-- name: MembersOfTenant :many
SELECT m.user_id, m.role, m.joined_at, m.last_seen_at, p.display_name, p.avatar, p.email
FROM tenant_members m
JOIN profiles p ON p.id = m.user_id
WHERE m.tenant_id = @tenant_id
ORDER BY m.joined_at;

-- name: SetMemberRole :execrows
UPDATE tenant_members SET role = @role WHERE tenant_id = @tenant_id AND user_id = @user_id;

-- name: TouchMember :exec
UPDATE tenant_members SET last_seen_at = now() WHERE tenant_id = @tenant_id AND user_id = @user_id;

-- name: DeleteMember :execrows
DELETE FROM tenant_members WHERE tenant_id = @tenant_id AND user_id = @user_id;
