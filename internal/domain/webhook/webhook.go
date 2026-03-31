package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Event types sent to webhook endpoints.
const (
	EventRunStarted   = "run.started"
	EventRunCompleted = "run.completed"
	EventRunFailed    = "run.failed"
	EventNodeDone     = "node.done"
	EventNodeFailed   = "node.failed"
)

// Payload is the JSON body sent to webhook endpoints.
type Payload struct {
	Event     string    `json:"event"`
	RunID     string    `json:"run_id"`
	NodeID    string    `json:"node_id,omitempty"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data,omitempty"`
}

// Config holds webhook configuration.
type Config struct {
	URLs    []string `json:"urls"`    // HTTP endpoints to POST to
	Events  []string `json:"events"`  // filter: only send these events (empty = all)
	Timeout int      `json:"timeout"` // request timeout in seconds (default 10)
}

// Sender sends webhook notifications.
type Sender struct {
	config Config
	client *http.Client
	logger *zap.Logger
}

// NewSender creates a webhook sender. If config has no URLs, all Send calls are no-ops.
func NewSender(config Config, logger *zap.Logger) *Sender {
	timeout := time.Duration(config.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Sender{
		config: config,
		client: &http.Client{Timeout: timeout},
		logger: logger,
	}
}

// Send dispatches a webhook payload to all configured URLs.
// Errors are logged but never returned — webhooks are best-effort.
func (s *Sender) Send(ctx context.Context, p Payload) {
	if len(s.config.URLs) == 0 {
		return
	}

	// Filter by event type if configured.
	if len(s.config.Events) > 0 {
		found := false
		for _, e := range s.config.Events {
			if e == p.Event {
				found = true
				break
			}
		}
		if !found {
			return
		}
	}

	p.Timestamp = time.Now()
	body, err := json.Marshal(p)
	if err != nil {
		s.logger.Error("webhook marshal", zap.Error(err))
		return
	}

	for _, u := range s.config.URLs {
		go func(url string) {
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
			if err != nil {
				s.logger.Warn("webhook request", zap.String("url", url), zap.Error(err))
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Stroppy-Event", p.Event)

			resp, err := s.client.Do(req)
			if err != nil {
				s.logger.Warn("webhook send", zap.String("url", url), zap.Error(err))
				return
			}
			resp.Body.Close()

			if resp.StatusCode >= 400 {
				s.logger.Warn("webhook response",
					zap.String("url", url),
					zap.Int("status", resp.StatusCode),
					zap.String("event", p.Event))
			}
		}(u)
	}
}

// Convenience helpers.

func (s *Sender) RunStarted(runID string) {
	s.Send(context.Background(), Payload{Event: EventRunStarted, RunID: runID})
}

func (s *Sender) RunCompleted(runID string, data any) {
	s.Send(context.Background(), Payload{Event: EventRunCompleted, RunID: runID, Data: data})
}

func (s *Sender) RunFailed(runID string, err error) {
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	s.Send(context.Background(), Payload{Event: EventRunFailed, RunID: runID, Error: errStr})
}

func (s *Sender) NodeDone(runID, nodeID string) {
	s.Send(context.Background(), Payload{Event: EventNodeDone, RunID: runID, NodeID: nodeID})
}

func (s *Sender) NodeFailed(runID, nodeID string, err error) {
	errStr := ""
	if err != nil {
		errStr = fmt.Sprintf("%v", err)
	}
	s.Send(context.Background(), Payload{Event: EventNodeFailed, RunID: runID, NodeID: nodeID, Error: errStr})
}
