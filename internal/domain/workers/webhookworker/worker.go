package webhookworker

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	"go.uber.org/zap"

	opspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/ops"
)

const (
	maxBackoffSeconds = 3600 // 1 hour cap
	claimBatchSize    = 10
)

// DeliveryStore is implemented by ops.WebhookService.
type DeliveryStore interface {
	ListPendingDeliveries(ctx context.Context, limit int) ([]*opspb.WebhookDelivery, error)
	GetWebhook(ctx context.Context, id *opspb.WebhookId) (*opspb.Webhook, error)
	UpdateDeliveryState(ctx context.Context, id string, state opspb.WebhookDeliveryState, attempts uint32, nextAt *time.Time, deliveredAt *time.Time, lastErr string, statusCode *uint32) error
}

// Worker drains pending WebhookDelivery rows and POSTs with HMAC signature.
type Worker struct {
	store  DeliveryStore
	client *http.Client
	log    *zap.Logger
	tick   time.Duration
}

// New constructs a webhookworker.
func New(store DeliveryStore, log *zap.Logger, tick time.Duration) *Worker {
	if tick == 0 {
		tick = 10 * time.Second
	}
	return &Worker{
		store:  store,
		client: &http.Client{Timeout: 30 * time.Second},
		log:    log,
		tick:   tick,
	}
}

// Run loops until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	t := time.NewTicker(w.tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := w.TickOnce(ctx); err != nil {
				w.log.Warn("webhook worker tick error", zap.Error(err))
			}
		}
	}
}

// TickOnce claims and processes one batch of pending deliveries.
func (w *Worker) TickOnce(ctx context.Context) error {
	deliveries, err := w.store.ListPendingDeliveries(ctx, claimBatchSize)
	if err != nil {
		return fmt.Errorf("webhookworker: list pending: %w", err)
	}
	for _, d := range deliveries {
		w.process(ctx, d)
	}
	return nil
}

func (w *Worker) process(ctx context.Context, d *opspb.WebhookDelivery) {
	webhook, err := w.store.GetWebhook(ctx, d.GetWebhookId())
	if err != nil {
		w.log.Warn("webhookworker: get webhook failed",
			zap.String("webhook_id", d.GetWebhookId().GetValue()), zap.Error(err))
		return
	}

	payload := d.GetPayload()
	sig := hmacSign([]byte(webhook.GetSecret()), payload)

	timeout := time.Duration(webhook.GetTimeoutSeconds()) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, webhook.GetUrl(), bytes.NewReader(payload))
	if err != nil {
		w.markFailed(ctx, d, webhook, fmt.Sprintf("build request: %v", err), nil)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Stroppy-Signature", "sha256="+sig)
	for k, v := range webhook.GetHeaders() {
		req.Header.Set(k, v)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		w.markFailed(ctx, d, webhook, err.Error(), nil)
		return
	}
	defer resp.Body.Close()

	code := uint32(resp.StatusCode)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		now := time.Now()
		_ = w.store.UpdateDeliveryState(ctx, d.GetId().GetValue(),
			opspb.WebhookDeliveryState_WEBHOOK_DELIVERY_STATE_DELIVERED,
			d.GetAttempts()+1, nil, &now, "", &code)
		return
	}
	w.markFailed(ctx, d, webhook, fmt.Sprintf("status %d", resp.StatusCode), &code)
}

func (w *Worker) markFailed(ctx context.Context, d *opspb.WebhookDelivery, webhook *opspb.Webhook, lastErr string, code *uint32) {
	attempts := d.GetAttempts() + 1
	maxRetries := webhook.GetMaxRetries()

	var nextState opspb.WebhookDeliveryState
	var nextAt *time.Time

	if maxRetries > 0 && attempts > maxRetries {
		nextState = opspb.WebhookDeliveryState_WEBHOOK_DELIVERY_STATE_DEAD
	} else {
		nextState = opspb.WebhookDeliveryState_WEBHOOK_DELIVERY_STATE_PENDING
		backoff := time.Duration(math.Min(
			math.Pow(2, float64(attempts)),
			float64(maxBackoffSeconds),
		)) * time.Second
		t := time.Now().Add(backoff)
		nextAt = &t
	}
	_ = w.store.UpdateDeliveryState(ctx, d.GetId().GetValue(), nextState, attempts, nextAt, nil, lastErr, code)
}
