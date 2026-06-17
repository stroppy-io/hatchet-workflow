package adapters

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	"github.com/stroppy-io/stroppy-cloud/internal/services/share"
)

// RandomTokenMinter mints opaque, unguessable share tokens from a cryptographic
// random source. Each token carries >=128 bits of entropy (default 32 bytes,
// 256 bits) and is URL-safe base64 without padding so it drops straight into a
// share URL.
type RandomTokenMinter struct{ nbytes int }

var _ share.TokenMinter = (*RandomTokenMinter)(nil)

// NewRandomTokenMinter builds a token minter. nbytes is the random byte count
// per token; values below 16 (128 bits) are bumped to 16, and 0 defaults to 32.
func NewRandomTokenMinter(nbytes int) *RandomTokenMinter {
	switch {
	case nbytes == 0:
		nbytes = 32
	case nbytes < 16:
		nbytes = 16
	}
	return &RandomTokenMinter{nbytes: nbytes}
}

// Mint returns a fresh URL-safe random token.
func (m *RandomTokenMinter) Mint() (string, error) {
	buf := make([]byte, m.nbytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("share: mint token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// ShareRunReader is the minimal consumer interface SnapshotBuilder needs to load
// the targeted run for projection. The gormstore TestRuns repo (Get by id) and
// SuiteRuns repo (Get by id) satisfy these once injected. Get returns
// derrors.ErrNotFound for an unknown id.
type ShareRunReader interface {
	// GetTestRun loads a test run by id (tenant ownership is asserted by the
	// builder against the returned record's entity).
	GetTestRun(ctx context.Context, id string) (*models.TestRunRecord, error)
	// GetSuiteRun loads a suite run by id.
	GetSuiteRun(ctx context.Context, id string) (*models.SuiteRunRecord, error)
}

// ShareMetricsReader optionally yields a run's computed metric snapshot so the
// shared test-run view can expose our metrics (never a Grafana link). A nil
// reader (or a not-found result) simply omits metrics from the snapshot.
type ShareMetricsReader interface {
	Get(ctx context.Context, runID string, window *monitor.TimeRange) (*monitor.RunMetrics, error)
}

// RunSnapshotBuilder implements share.SnapshotBuilder: it freezes the limited,
// public projection of a run at share-creation time. It verifies the target run
// exists and belongs to the share's tenant, then projects only safe fields (no
// baked spec, params, secrets, raw logs, shell or tenant data).
type RunSnapshotBuilder struct {
	runs    ShareRunReader
	metrics ShareMetricsReader
}

var _ share.SnapshotBuilder = (*RunSnapshotBuilder)(nil)

// NewRunSnapshotBuilder builds the snapshot builder. metrics may be nil (metrics
// are then omitted from the test-run snapshot).
func NewRunSnapshotBuilder(runs ShareRunReader, metrics ShareMetricsReader) *RunSnapshotBuilder {
	return &RunSnapshotBuilder{runs: runs, metrics: metrics}
}

// Build assembles the frozen snapshot for the target. derrors.ErrNotFound when
// the target run does not exist in the tenant.
func (b *RunSnapshotBuilder) Build(ctx context.Context, tenantID string, target *models.ShareRecord_Target) (*models.ShareRecord_Snapshot, error) {
	snap := &models.ShareRecord_Snapshot{CapturedAt: nowTimestamp()}
	switch target.GetKind() {
	case models.ShareRecord_Target_KIND_TEST_RUN:
		rec, err := b.runs.GetTestRun(ctx, target.GetId())
		if err != nil {
			return nil, err
		}
		if rec.GetEntity().GetTenantId() != tenantID {
			return nil, derrors.NotFound("test_run", "run not found in tenant")
		}
		snap.View = &models.ShareRecord_Snapshot_TestRun{TestRun: b.sharedTestRun(ctx, rec)}
	case models.ShareRecord_Target_KIND_SUITE_RUN:
		rec, err := b.runs.GetSuiteRun(ctx, target.GetId())
		if err != nil {
			return nil, err
		}
		if rec.GetEntity().GetTenantId() != tenantID {
			return nil, derrors.NotFound("suite_run", "suite run not found in tenant")
		}
		snap.View = &models.ShareRecord_Snapshot_SuiteRun{SuiteRun: b.sharedSuiteRun(rec)}
	default:
		return nil, derrors.Invalid("target.kind", "unsupported share target kind")
	}
	return snap, nil
}

// sharedTestRun projects a stored test run into its limited public view.
func (b *RunSnapshotBuilder) sharedTestRun(ctx context.Context, rec *models.TestRunRecord) *models.SharedTestRun {
	sum := rec.GetSummary()
	view := &models.SharedTestRun{
		Name:           rec.GetEntity().GetName(),
		Status:         rec.GetStatus(),
		DbKind:         sum.GetDbKind(),
		DbName:         sum.GetDbPresetName(),
		WorkloadName:   sum.GetWorkloadName(),
		StroppyVersion: sum.GetStroppyVersion(),
		Provider:       sum.GetProvider(),
		TopologyLabel:  sum.GetTopologyLabel(),
		NodeCount:      sum.GetNodeCount(),
		StartedAt:      sum.GetStartedAt(),
		FinishedAt:     sum.GetFinishedAt(),
		Duration:       sum.GetDuration(),
		ProgressPct:    sum.GetProgressPct(),
	}
	if b.metrics != nil {
		if m, err := b.metrics.Get(ctx, rec.GetEntity().GetId(), nil); err == nil {
			view.Metrics = m
		}
	}
	return view
}

// sharedSuiteRun projects a stored suite run into its limited public view.
func (b *RunSnapshotBuilder) sharedSuiteRun(rec *models.SuiteRunRecord) *models.SharedSuiteRun {
	sum := rec.GetSummary()
	return &models.SharedSuiteRun{
		Name:        rec.GetEntity().GetName(),
		Status:      rec.GetStatus(),
		Provider:    sum.GetProvider(),
		DbKinds:     sum.GetDbKinds(),
		Total:       sum.GetTotal(),
		Completed:   sum.GetCompleted(),
		Failed:      sum.GetFailed(),
		Running:     sum.GetRunning(),
		ProgressPct: sum.GetProgressPct(),
		StartedAt:   sum.GetStartedAt(),
		FinishedAt:  sum.GetFinishedAt(),
		Duration:    sum.GetDuration(),
	}
}
