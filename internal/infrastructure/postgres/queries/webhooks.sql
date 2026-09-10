-- name: InsertWebhook :exec
INSERT INTO webhooks (id, tenant_id, url, events, enabled, description, secret)
VALUES (@id, @tenant_id, @url, @events, @enabled, @description, @secret);

-- name: WebhookByID :one
SELECT id, tenant_id, url, events, enabled, description, secret, prev_secret, prev_expires_at, created_at, updated_at
FROM webhooks WHERE id = @id;

-- name: WebhooksOfTenant :many
SELECT id, tenant_id, url, events, enabled, description, secret, prev_secret, prev_expires_at, created_at, updated_at
FROM webhooks WHERE tenant_id = @tenant_id ORDER BY created_at;

-- name: EnabledWebhooksOfTenant :many
SELECT id, tenant_id, url, events, enabled, description, secret, prev_secret, prev_expires_at, created_at, updated_at
FROM webhooks WHERE tenant_id = @tenant_id AND enabled ORDER BY created_at;

-- name: UpdateWebhook :execrows
UPDATE webhooks
SET url         = COALESCE(@url::text, url),
    events      = COALESCE(@events::jsonb, events),
    enabled     = COALESCE(@enabled::boolean, enabled),
    description = COALESCE(@description::text, description),
    updated_at  = now()
WHERE id = @id;

-- name: RotateWebhookSecret :execrows
UPDATE webhooks
SET prev_secret = secret, prev_expires_at = @prev_expires_at, secret = @secret, updated_at = now()
WHERE id = @id;

-- name: DeleteWebhook :execrows
DELETE FROM webhooks WHERE id = @id;

-- name: LastDeliveryOfWebhook :one
SELECT status, COALESCE(last_attempt_at, created_at) AS at
FROM webhook_deliveries WHERE webhook_id = @webhook_id ORDER BY created_at DESC LIMIT 1;

-- name: InsertDelivery :exec
INSERT INTO webhook_deliveries (id, webhook_id, event, payload) VALUES (@id, @webhook_id, @event, @payload);

-- name: DeliveryByID :one
SELECT id, webhook_id, event, status, attempts, next_attempt_at, last_attempt_at, response_status, error, payload, created_at
FROM webhook_deliveries WHERE id = @id;

-- name: DeliveriesOfWebhook :many
SELECT id, webhook_id, event, status, attempts, next_attempt_at, last_attempt_at, response_status, error, payload, created_at
FROM webhook_deliveries
WHERE webhook_id = @webhook_id AND (@before::timestamptz IS NULL OR created_at < @before::timestamptz)
ORDER BY created_at DESC
LIMIT @lim;

-- name: DueDeliveries :many
-- Claimed by the dispatcher: bump next_attempt_at so a second replica
-- does not pick the same rows within the lease.
UPDATE webhook_deliveries
SET next_attempt_at = now() + @lease::interval
WHERE id IN (
    SELECT id FROM webhook_deliveries
    WHERE status = 'pending' AND next_attempt_at <= now()
    ORDER BY next_attempt_at
    LIMIT @lim
    FOR UPDATE SKIP LOCKED
)
RETURNING id, webhook_id, event, status, attempts, next_attempt_at, last_attempt_at, response_status, error, payload, created_at;

-- name: FinishDelivery :exec
UPDATE webhook_deliveries
SET status = @status, attempts = attempts + 1, last_attempt_at = now(), next_attempt_at = @next_attempt_at,
    response_status = @response_status, error = @error
WHERE id = @id;

-- name: ResetDelivery :execrows
-- Replay: back to pending, attempts kept for the record.
UPDATE webhook_deliveries SET status = 'pending', next_attempt_at = now(), error = '' WHERE id = @id;
