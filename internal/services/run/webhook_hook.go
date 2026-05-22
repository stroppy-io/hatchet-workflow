package run

import (
	"context"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
)

// RunEventDeliverer delivers a run-lifecycle webhook (implemented by the
// WebhookService). Kept as an interface so runtime wiring stays decoupled.
type RunEventDeliverer interface {
	DeliverRunEvent(ctx context.Context, tenantID string, event models.Webhook_Event, id, name string) error
}

// WebhookTerminalHook bridges the generic runtime terminal-hook to run webhook
// delivery: a dag that reaches a terminal status fires the matching RUN_* event to
// the tenant's webhooks (tenant id + name carried in the dag metadata; the dag id is
// the run id, H22). Wire via runtime.WithTerminalHook(run.WebhookTerminalHook(svc)).
func WebhookTerminalHook(d RunEventDeliverer) func(context.Context, *primitive.Dag) {
	return func(ctx context.Context, dag *primitive.Dag) {
		tenantID := dag.GetMetadata()[metadataTenantID]
		if tenantID == "" {
			return // not a tenant-scoped run dag (e.g. an internal dag).
		}
		var event models.Webhook_Event
		switch dag.GetStatus() {
		case primitive.Status_STATUS_COMPLETED:
			event = models.Webhook_EVENT_RUN_COMPLETED
		case primitive.Status_STATUS_FAILED:
			event = models.Webhook_EVENT_RUN_FAILED
		default:
			return // cancelled/other: no webhook.
		}
		_ = d.DeliverRunEvent(ctx, tenantID, event, dag.GetId(), dag.GetMetadata()["name"])
	}
}
