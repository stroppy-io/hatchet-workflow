-- name: DenySession :exec
INSERT INTO iam_denylist (session_id, user_id, expires_at)
VALUES (@session_id, @user_id, @expires_at)
ON CONFLICT (session_id) DO UPDATE SET expires_at = EXCLUDED.expires_at;

-- name: DeniedSession :one
-- No row = not denied.
SELECT expires_at FROM iam_denylist WHERE session_id = @session_id AND expires_at > now();

-- name: PurgeDenylist :execrows
DELETE FROM iam_denylist WHERE expires_at <= now();

-- name: RecordWebhookEvent :execrows
-- 0 rows = duplicate delivery, the caller skips the side effects.
INSERT INTO iam_webhook_events (id) VALUES (@id) ON CONFLICT DO NOTHING;

-- name: PurgeWebhookEvents :execrows
DELETE FROM iam_webhook_events WHERE received_at < @before;
