package ops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/yaroher/ratel/pkg/dml/set"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	opspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/ops"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// WebhookService provides CRUD and TestWebhook for the webhooks table.
type WebhookService struct {
	repo   *repository.ProtoRepository[opspb.WebhookAlias, opspb.WebhookColumnAlias, *opspb.WebhookScanner, *opspb.Webhook]
	dRepo  *repository.ProtoRepository[opspb.WebhookDeliveryAlias, opspb.WebhookDeliveryColumnAlias, *opspb.WebhookDeliveryScanner, *opspb.WebhookDelivery]
	txMgr  pgtx.TxManager
	events eventing.Bus
}

// NewWebhookService constructs a WebhookService.
func NewWebhookService(executor exec.DB, txMgr pgtx.TxManager, events eventing.Bus) *WebhookService {
	return &WebhookService{
		repo: repository.NewProtoRepository(
			repository.NewScannerRepository(opspb.Webhooks.Table, executor),
			opspb.WebhookConverter,
		),
		dRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(opspb.WebhookDeliverys.Table, executor),
			opspb.WebhookDeliveryConverter,
		),
		txMgr:  txMgr,
		events: events,
	}
}

// encodeHeaders JSON-encodes a map[string]string to a text value for postgres.
func encodeHeaders(h map[string]string) string {
	if len(h) == 0 {
		return "{}"
	}
	b, _ := json.Marshal(h)
	return string(b)
}

// CreateWebhook inserts a new webhook and returns it with generated ID/timestamps.
func (s *WebhookService) CreateWebhook(ctx context.Context, tenantID *iampb.TenantId, createdBy *iampb.UserId, w *opspb.Webhook) (*opspb.Webhook, error) {
	return pgtx.WithSerializableRet(ctx, s.txMgr, func(ctx context.Context) (*opspb.Webhook, error) {
		nowTS := timestamppb.Now()
		w.Id = &opspb.WebhookId{Value: ids.New()}
		w.TenantId = tenantID
		w.CreatedBy = createdBy
		w.Timestamps = &commonpb.Timestamps{CreatedAt: nowTS, UpdatedAt: nowTS}

		scanner := w.IntoPlain()
		if scanner.Label == nil {
			scanner.Label = []string{}
		}
		if scanner.Events == nil {
			scanner.Events = []string{}
		}

		// Build explicit setters — skip AllSetters() because Headers (map[string]string)
		// must be JSON-encoded to text for the text column, and TimeoutSeconds/MaxRetries
		// must be int32, not uint32.
		var createdByVal *string
		if scanner.CreatedBy != nil {
			createdByVal = scanner.CreatedBy
		}

		_, err := s.repo.Execute(ctx,
			opspb.Webhooks.Insert().From(
				set.NewSetter(opspb.WebhookColumnId, scanner.Id),
				set.NewSetter(opspb.WebhookColumnTenantId, scanner.TenantId),
				set.NewSetter(opspb.WebhookColumnCreatedAt, scanner.CreatedAt),
				set.NewSetter(opspb.WebhookColumnUpdatedAt, scanner.UpdatedAt),
				set.NewSetter(opspb.WebhookColumnName, scanner.Name),
				set.NewSetter(opspb.WebhookColumnLabel, scanner.Label),
				set.NewSetter(opspb.WebhookColumnCreatedBy, createdByVal),
				set.NewSetter(opspb.WebhookColumnUrl, scanner.Url),
				set.NewSetter(opspb.WebhookColumnEvents, scanner.Events),
				set.NewSetter(opspb.WebhookColumnSecret, scanner.Secret),
				set.NewSetter(opspb.WebhookColumnHeaders, encodeHeaders(scanner.Headers)),
				set.NewSetter(opspb.WebhookColumnTimeoutSeconds, int32(scanner.TimeoutSeconds)),
				set.NewSetter(opspb.WebhookColumnMaxRetries, int32(scanner.MaxRetries)),
				set.NewSetter(opspb.WebhookColumnEnabled, scanner.Enabled),
				set.NewSetter(opspb.WebhookColumnLastFailureError, scanner.LastFailureError),
			),
		)
		if err != nil {
			return nil, err
		}
		return w, nil
	})
}

// GetWebhook fetches a webhook by id.
func (s *WebhookService) GetWebhook(ctx context.Context, id *opspb.WebhookId) (*opspb.Webhook, error) {
	w, err := s.repo.QueryRow(ctx,
		opspb.Webhooks.SelectAll().Where(
			opspb.Webhooks.Id.Eq(id.GetValue()),
			opspb.Webhooks.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("webhook", id.GetValue()))
		}
		return nil, err
	}
	return w, nil
}

// ListWebhooks returns all non-deleted webhooks for a tenant.
func (s *WebhookService) ListWebhooks(ctx context.Context, tenantID *iampb.TenantId) ([]*opspb.Webhook, error) {
	return s.repo.Query(ctx,
		opspb.Webhooks.SelectAll().Where(
			opspb.Webhooks.TenantId.Eq(tenantID.GetValue()),
			opspb.Webhooks.DeletedAt.IsNull(),
		),
	)
}

// UpdateWebhook applies a full-replace update to an existing webhook.
func (s *WebhookService) UpdateWebhook(ctx context.Context, w *opspb.Webhook) (*opspb.Webhook, error) {
	return pgtx.WithSerializableRet(ctx, s.txMgr, func(ctx context.Context) (*opspb.Webhook, error) {
		existing, err := s.GetWebhook(ctx, w.GetId())
		if err != nil {
			return nil, err
		}

		now := timestamppb.Now()
		w.TenantId = existing.GetTenantId()
		w.CreatedBy = existing.GetCreatedBy()
		if existing.GetTimestamps() != nil {
			w.Timestamps = &commonpb.Timestamps{
				CreatedAt: existing.GetTimestamps().GetCreatedAt(),
				UpdatedAt: now,
			}
		} else {
			w.Timestamps = &commonpb.Timestamps{UpdatedAt: now}
		}

		scanner := w.IntoPlain()
		if scanner.Label == nil {
			scanner.Label = []string{}
		}
		if scanner.Events == nil {
			scanner.Events = []string{}
		}
		if scanner.Headers == nil {
			scanner.Headers = map[string]string{}
		}

		nowTime := time.Now()

		// Headers stored as JSON text; encode map back to JSON string.
		var headersJSON string
		if len(scanner.Headers) > 0 {
			hb, _ := json.Marshal(scanner.Headers)
			headersJSON = string(hb)
		} else {
			headersJSON = "{}"
		}

		if _, err := s.repo.Execute(ctx,
			opspb.Webhooks.Update().
				Set(opspb.Webhooks.Url.Set(scanner.Url)).
				Set(opspb.Webhooks.Events.Set(scanner.Events)).
				Set(opspb.Webhooks.Secret.Set(scanner.Secret)).
				Set(opspb.Webhooks.Headers.Set(headersJSON)).
				Set(opspb.Webhooks.TimeoutSeconds.Set(int32(scanner.TimeoutSeconds))).
				Set(opspb.Webhooks.MaxRetries.Set(int32(scanner.MaxRetries))).
				Set(opspb.Webhooks.Enabled.Set(scanner.Enabled)).
				Set(opspb.Webhooks.UpdatedAt.Set(nowTime)).
				Where(
					opspb.Webhooks.Id.Eq(w.GetId().GetValue()),
					opspb.Webhooks.DeletedAt.IsNull(),
				),
		); err != nil {
			return nil, err
		}
		return w, nil
	})
}

// DeleteWebhook soft-deletes a webhook and returns the deleted record.
func (s *WebhookService) DeleteWebhook(ctx context.Context, id *opspb.WebhookId) (*opspb.Webhook, error) {
	return pgtx.WithSerializableRet(ctx, s.txMgr, func(ctx context.Context) (*opspb.Webhook, error) {
		existing, err := s.GetWebhook(ctx, id)
		if err != nil {
			return nil, err
		}

		now := time.Now()
		if _, err := s.repo.Execute(ctx,
			opspb.Webhooks.Update().
				Set(opspb.Webhooks.DeletedAt.Set(&now)).
				Where(
					opspb.Webhooks.Id.Eq(id.GetValue()),
					opspb.Webhooks.DeletedAt.IsNull(),
				),
		); err != nil {
			return nil, err
		}
		return existing, nil
	})
}

// TestWebhook POSTs a test JSON payload to the webhook's URL inline.
func (s *WebhookService) TestWebhook(ctx context.Context, id *opspb.WebhookId, event opspb.WebhookEvent) (*opspb.TestWebhookResponse, error) {
	w, err := s.GetWebhook(ctx, id)
	if err != nil {
		return nil, err
	}

	if event == opspb.WebhookEvent_WEBHOOK_EVENT_UNSPECIFIED {
		event = opspb.WebhookEvent_WEBHOOK_EVENT_RUN_STARTED
	}

	payload := map[string]any{
		"event":     event.String(),
		"test":      true,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	body, _ := json.Marshal(payload)

	timeout := time.Duration(w.GetTimeoutSeconds()) * time.Second
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	httpClient := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.GetUrl(), bytes.NewReader(body))
	if err != nil {
		return &opspb.TestWebhookResponse{
			Delivered: false,
			Error:     fmt.Sprintf("build request: %v", err),
		}, nil
	}
	req.Header.Set("Content-Type", "application/json")
	if secret := w.GetSecret(); secret != "" {
		req.Header.Set("X-Stroppy-Signature", "sha256="+hmacSign([]byte(secret), body))
	}
	for k, v := range w.GetHeaders() {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := httpClient.Do(req)
	latencyMs := uint32(time.Since(start).Milliseconds())
	if err != nil {
		return &opspb.TestWebhookResponse{
			Delivered: false,
			LatencyMs: latencyMs,
			Error:     err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	delivered := resp.StatusCode >= 200 && resp.StatusCode < 300
	result := &opspb.TestWebhookResponse{
		Delivered:  delivered,
		StatusCode: uint32(resp.StatusCode),
		LatencyMs:  latencyMs,
	}
	if !delivered {
		result.Error = fmt.Sprintf("unexpected status %d", resp.StatusCode)
	}
	return result, nil
}

// EnqueueDelivery inserts a WebhookDelivery row with state=PENDING.
func (s *WebhookService) EnqueueDelivery(ctx context.Context, webhookID string, event opspb.WebhookEvent, payloadJSON []byte) error {
	now := time.Now()
	nowTs := timestamppb.New(now)
	d := &opspb.WebhookDelivery{
		Id:        &opspb.WebhookDeliveryId{Value: ids.New()},
		WebhookId: &opspb.WebhookId{Value: webhookID},
		Event:     event,
		Payload:   payloadJSON,
		State:     opspb.WebhookDeliveryState_WEBHOOK_DELIVERY_STATE_PENDING,
		Attempts:  0,
		LastError: "",
		Timestamps: &commonpb.Timestamps{
			CreatedAt: nowTs,
			UpdatedAt: nowTs,
		},
	}
	nextAttempt := now
	d.NextAttemptAt = timestamppb.New(nextAttempt)

	scanner := d.IntoPlain()
	_, err := s.dRepo.Execute(ctx,
		opspb.WebhookDeliverys.Insert().From(
			set.NewSetter(opspb.WebhookDeliveryColumnId, scanner.Id),
			set.NewSetter(opspb.WebhookDeliveryColumnCreatedAt, scanner.CreatedAt),
			set.NewSetter(opspb.WebhookDeliveryColumnUpdatedAt, scanner.UpdatedAt),
			set.NewSetter(opspb.WebhookDeliveryColumnWebhookId, scanner.WebhookId),
			set.NewSetter(opspb.WebhookDeliveryColumnEvent, scanner.Event),
			set.NewSetter(opspb.WebhookDeliveryColumnPayload, scanner.Payload),
			set.NewSetter(opspb.WebhookDeliveryColumnState, scanner.State),
			set.NewSetter(opspb.WebhookDeliveryColumnAttempts, scanner.Attempts),
			set.NewSetter(opspb.WebhookDeliveryColumnNextAttemptAt, scanner.NextAttemptAt),
			set.NewSetter(opspb.WebhookDeliveryColumnLastError, scanner.LastError),
		),
	)
	return err
}

// RecoverInFlightDeliveries resets any delivery stuck in IN_FLIGHT (e.g. after
// a server crash) back to PENDING so the outbox worker will retry it.
// Called once at startup by the recovery worker.
// Returns the number of rows updated.
func (s *WebhookService) RecoverInFlightDeliveries(ctx context.Context) (int64, error) {
	return s.dRepo.Execute(ctx,
		opspb.WebhookDeliverys.Update().
			Set(
				opspb.WebhookDeliverys.State.Set(opspb.WebhookDeliveryState_WEBHOOK_DELIVERY_STATE_PENDING.String()),
				opspb.WebhookDeliverys.LastError.Set("restart_requeue"),
				opspb.WebhookDeliverys.UpdatedAt.Set(time.Now()),
			).
			Where(
				opspb.WebhookDeliverys.State.Eq(opspb.WebhookDeliveryState_WEBHOOK_DELIVERY_STATE_IN_FLIGHT.String()),
			),
	)
}

// ListPendingDeliveries returns up to limit PENDING deliveries.
func (s *WebhookService) ListPendingDeliveries(ctx context.Context, limit int) ([]*opspb.WebhookDelivery, error) {
	return s.dRepo.Query(ctx,
		opspb.WebhookDeliverys.SelectAll().Where(
			opspb.WebhookDeliverys.State.Eq(opspb.WebhookDeliveryState_WEBHOOK_DELIVERY_STATE_PENDING.String()),
			opspb.WebhookDeliverys.DeletedAt.IsNull(),
		),
	)
}

// UpdateDeliveryState transitions a delivery row to the given state.
func (s *WebhookService) UpdateDeliveryState(
	ctx context.Context,
	id string,
	state opspb.WebhookDeliveryState,
	attempts uint32,
	nextAt *time.Time,
	deliveredAt *time.Time,
	lastErr string,
	statusCode *uint32,
) error {
	now := time.Now()
	attemptsI32 := int32(attempts)
	var statusCodeI32 *int32
	if statusCode != nil {
		v := int32(*statusCode)
		statusCodeI32 = &v
	}
	_, err := s.dRepo.Execute(ctx,
		opspb.WebhookDeliverys.Update().
			Set(
				opspb.WebhookDeliverys.State.Set(state.String()),
				opspb.WebhookDeliverys.Attempts.Set(attemptsI32),
				opspb.WebhookDeliverys.NextAttemptAt.Set(nextAt),
				opspb.WebhookDeliverys.DeliveredAt.Set(deliveredAt),
				opspb.WebhookDeliverys.LastError.Set(lastErr),
				opspb.WebhookDeliverys.LastStatusCode.Set(statusCodeI32),
				opspb.WebhookDeliverys.UpdatedAt.Set(now),
			).
			Where(opspb.WebhookDeliverys.Id.Eq(id)),
	)
	return err
}
