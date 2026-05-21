// Package share implements run.ShareStore: an immutable, public, read-only
// snapshot of a run (+ its metrics) addressed by an opaque token (G5). The
// snapshot is frozen in Valkey at create time so a later config/metric change
// does not alter a shared link.
package share

import (
	"context"
	"fmt"

	"github.com/gopherex/xlog"
	"github.com/valkey-io/valkey-go"
	"google.golang.org/protobuf/encoding/protojson"

	domainauth "github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	metricspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/metrics"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

const shareKeyPrefix = "share:"

// MetricsFetcher snapshots a run's metric summaries (run.MetricsClient satisfies it).
type MetricsFetcher interface {
	GetRunMetrics(ctx context.Context, runID string) (*metricspb.RunMetrics, error)
}

// Config supplies the public base URL for share links.
type Config interface {
	ShareBaseURL() string
}

// Store persists immutable run snapshots in Valkey.
type Store struct {
	*tracing.Entity
	valkey  valkey.Client
	metrics MetricsFetcher
	cfg     Config
}

// New builds a share Store.
func New(logger *xlog.Logger, vk valkey.Client, metrics MetricsFetcher, cfg Config) *Store {
	return &Store{Entity: tracing.NewEntity(logger.AppendName("ShareStore")), valkey: vk, metrics: metrics, cfg: cfg}
}

// CreateShareLink freezes the run + its current metrics under a fresh token and
// returns the token and the public URL.
func (s *Store) CreateShareLink(ctx context.Context, run *models.TestRun) (string, string, error) {
	token, err := domainauth.GenerateOpaqueToken()
	if err != nil {
		return "", "", err
	}
	// best-effort metric snapshot — a run with no metrics still shares.
	m, _ := s.metrics.GetRunMetrics(ctx, run.GetEntity().GetId().GetValue())

	data, err := protojson.Marshal(&uipb.GetSharedRunResponse{Run: run, Metrics: m})
	if err != nil {
		return "", "", fmt.Errorf("share: marshal snapshot: %w", err)
	}
	if err := s.valkey.Do(ctx, s.valkey.B().Set().Key(shareKeyPrefix+token).Value(string(data)).Build()).Error(); err != nil {
		return "", "", fmt.Errorf("share: store snapshot: %w", err)
	}
	return token, s.cfg.ShareBaseURL() + token, nil
}

// GetSharedRun resolves a public share token to its frozen snapshot.
func (s *Store) GetSharedRun(ctx context.Context, token string) (*uipb.GetSharedRunResponse, error) {
	raw, err := s.valkey.Do(ctx, s.valkey.B().Get().Key(shareKeyPrefix+token).Build()).ToString()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return nil, fmt.Errorf("share: link not found")
		}
		return nil, err
	}
	var snap uipb.GetSharedRunResponse
	if err := protojson.Unmarshal([]byte(raw), &snap); err != nil {
		return nil, fmt.Errorf("share: decode snapshot: %w", err)
	}
	return &snap, nil
}
