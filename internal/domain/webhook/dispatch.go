package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Dispatcher delivers pending rows: one HTTP POST per attempt, exponential
// backoff, capped attempts. Safe to run in several replicas (rows are
// leased with SKIP LOCKED).
type Dispatcher struct {
	repo   Repository
	client *http.Client
	// Now is injectable for tests.
	Now func() time.Time
}

// Dispatch tuning.
const (
	MaxAttempts   = 10
	firstBackoff  = 30 * time.Second
	maxBackoff    = 5 * time.Minute
	lease         = 2 * time.Minute
	batch         = 50
	requestLimit  = 15 * time.Second
	responseLimit = 4 << 10
)

// NewDispatcher builds the dispatcher.
func NewDispatcher(repo Repository) *Dispatcher {
	return &Dispatcher{repo: repo, client: &http.Client{Timeout: requestLimit}, Now: time.Now}
}

// Run loops until ctx ends: tick, deliver due rows.
func (d *Dispatcher) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		d.Tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Tick delivers one batch; returns how many rows were attempted.
func (d *Dispatcher) Tick(ctx context.Context) int {
	due, err := d.repo.Due(ctx, lease, batch)
	if err != nil {
		return 0
	}
	for _, del := range due {
		d.deliver(ctx, del)
	}
	return len(due)
}

func (d *Dispatcher) deliver(ctx context.Context, del Delivery) {
	w, err := d.repo.ByID(ctx, del.WebhookID)
	if err != nil || !w.Enabled {
		_ = d.repo.Finish(ctx, del.ID, DeliveryFailed, d.Now(), nil, "webhook disabled or gone") //nolint:errcheck // terminal
		return
	}
	status, err := d.post(ctx, w, del)
	attempt := del.Attempts + 1
	switch {
	case err == nil:
		_ = d.repo.Finish(ctx, del.ID, DeliverySuccess, d.Now(), &status, "") //nolint:errcheck // terminal
	case attempt >= MaxAttempts:
		_ = d.repo.Finish(ctx, del.ID, DeliveryFailed, d.Now(), optStatus(status), err.Error()) //nolint:errcheck // terminal
	default:
		_ = d.repo.Finish(ctx, del.ID, DeliveryPending, d.Now().Add(backoff(attempt)), optStatus(status), err.Error()) //nolint:errcheck // retried
	}
}

func optStatus(s int) *int {
	if s == 0 {
		return nil
	}
	return &s
}

func backoff(attempt int) time.Duration {
	b := firstBackoff << (attempt - 1)
	if b > maxBackoff || b <= 0 {
		return maxBackoff
	}
	return b
}

// post sends one Standard Webhooks request. Non-2xx is an error.
func (d *Dispatcher) post(ctx context.Context, w Webhook, del Delivery) (int, error) {
	env := envelope{ID: del.ID.String(), Type: del.Event, Timestamp: d.Now().UTC(), Data: del.Payload}
	body, err := json.Marshal(env)
	if err != nil {
		return 0, err
	}
	ts := strconv.FormatInt(env.Timestamp.Unix(), 10)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "stroppy-cloud-webhooks/1")
	req.Header.Set("Webhook-Id", del.ID.String())
	req.Header.Set("Webhook-Timestamp", ts)
	req.Header.Set("Webhook-Signature", Signatures(w, del.ID.String(), ts, body, d.Now()))
	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, responseLimit)) //nolint:errcheck // drain
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return resp.StatusCode, fmt.Errorf("status %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

// envelope is the wire body.
type envelope struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// Signatures renders the Webhook-Signature header: the current secret and,
// inside the overlap, the previous one.
func Signatures(w Webhook, id, ts string, body []byte, now time.Time) string {
	sigs := []string{"v1," + sign(w.Secret, id, ts, body)}
	if w.PrevSecret != "" && w.PrevExpiresAt != nil && now.Before(*w.PrevExpiresAt) {
		sigs = append(sigs, "v1,"+sign(w.PrevSecret, id, ts, body))
	}
	return strings.Join(sigs, " ")
}

func sign(secret, id, ts string, body []byte) string {
	key := []byte(secret)
	if strings.HasPrefix(secret, "whsec_") {
		if raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_")); err == nil {
			key = raw
		}
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + ts + "."))
	mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// Verify checks a signature header against a secret — the consumer side,
// used by tests and documented for integrators.
func Verify(secret, header, id, ts string, body []byte) bool {
	want := sign(secret, id, ts, body)
	for _, part := range strings.Fields(header) {
		if v, ok := strings.CutPrefix(part, "v1,"); ok && hmac.Equal([]byte(v), []byte(want)) {
			return true
		}
	}
	return false
}
