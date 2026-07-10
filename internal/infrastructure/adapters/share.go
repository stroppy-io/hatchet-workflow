package adapters

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strconv"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
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
	view.WorkloadSegments = sharedWorkloadSegments(rec.GetSpec().GetWorkload())
	view.Database = sharedDatabase(rec.GetSpec().GetDatabase())
	return view
}

/*
	Public projection of the run spec.

	These helpers are an ALLOWLIST, never a redaction pass: a field reaches the
	public view only because it is named here. The spec also holds `parameters.env`,
	`segment.sql`, workload files, `execution.extra_args` and the engines'
	free-form `*_options` maps — all author-supplied, all capable of carrying
	credentials or private schemas — and none of them are read below. Adding a
	field means deciding, by hand, that it is safe for a stranger with a link.
*/

// sharedWorkloadSegments projects the stroppy launch knobs that explain a
// number: what ran, how hard, and how rows were written.
func sharedWorkloadSegments(w *domain.Workload) []*models.SharedWorkloadSegment {
	segments := w.GetSegments()
	if len(segments) == 0 {
		return nil
	}
	out := make([]*models.SharedWorkloadSegment, 0, len(segments))
	for _, seg := range segments {
		exec := seg.GetExecution()
		params := seg.GetParameters()
		out = append(out, &models.SharedWorkloadSegment{
			Name:         seg.GetName(),
			Script:       seg.GetScript(),
			Vus:          exec.GetVus(),
			Duration:     exec.GetDuration(),
			Iterations:   exec.GetIterations(),
			Quiet:        exec.GetQuiet(),
			NoThresholds: exec.GetNoThresholds(),
			PoolSize:     params.GetPoolSize(),
			ScaleFactor:  params.GetScaleFactor(),
			InsertMethod: params.GetDefaultInsertMethod(),
			BulkSize:     params.GetBulkSize(),
			Steps:        params.GetSteps(),
			NoSteps:      params.GetNoSteps(),
		})
	}
	return out
}

// sharedDatabase projects the database sizing + typed tuning. Every setting key
// is chosen here and every value is rendered from a typed field, so no
// author-supplied map entry can reach the public view.
func sharedDatabase(db *domain.Database) *models.SharedDatabase {
	params := db.GetParams()
	if params == nil {
		return nil
	}
	view := &models.SharedDatabase{Version: params.GetVersion()}

	add := func(key, value string) {
		if value == "" {
			return
		}
		view.Settings = append(view.Settings, &models.SharedDatabase_Setting{Key: key, Value: value})
	}
	addNum := func(key string, n uint32) {
		if n == 0 {
			return
		}
		add(key, strconv.FormatUint(uint64(n), 10))
	}
	addBool := func(key string, on bool) {
		if on {
			add(key, "true")
		}
	}

	switch engine := params.GetEngine().(type) {
	case *domain.DatabaseParams_Postgres:
		p := engine.Postgres
		addNum("replicas", p.GetReplicas())
		addNum("sync_replicas", p.GetSyncReplicas())
		addNum("haproxy", p.GetHaproxy())
		addBool("pgbouncer", p.GetPgbouncer())
		addBool("patroni", p.GetPatroni())
		addBool("etcd", p.GetEtcd())
	case *domain.DatabaseParams_Orioledb:
		p := engine.Orioledb
		add("image", p.GetImage())
		addNum("shared_buffers_mb", p.GetSharedBuffersMb())
		addNum("replicas", p.GetReplicas())
		addNum("haproxy", p.GetHaproxy())
	case *domain.DatabaseParams_Mysql:
		sharedMySQL(engine.Mysql, addNum, addBool)
	case *domain.DatabaseParams_Mariadb:
		sharedMySQL(engine.Mariadb, addNum, addBool)
	case *domain.DatabaseParams_Picodata:
		p := engine.Picodata
		addNum("instances", p.GetInstances())
		addNum("replication_factor", p.GetReplicationFactor())
		addNum("shards", p.GetShards())
		addNum("haproxy", p.GetHaproxy())
	case *domain.DatabaseParams_Ydb:
		p := engine.Ydb
		addNum("storage_nodes", p.GetStorageNodes())
		addNum("database_nodes", p.GetDatabaseNodes())
		addNum("pdisks_per_storage_node", p.GetPdisksPerStorageNode())
		addNum("storage_groups", p.GetStorageGroups())
		addBool("auto_size_pdisks", p.GetAutoSizePdisks())
		addNum("haproxy", p.GetHaproxy())
	case *domain.DatabaseParams_Cockroach:
		addNum("nodes", engine.Cockroach.GetNodes())
	}

	if view.GetVersion() == "" && len(view.GetSettings()) == 0 {
		return nil
	}
	return view
}

func sharedMySQL(p *domain.MySqlParams, addNum func(string, uint32), addBool func(string, bool)) {
	addNum("replicas", p.GetReplicas())
	addNum("proxysql", p.GetProxysql())
	addBool("group_replication", p.GetGroupReplication())
	addBool("semi_sync", p.GetSemiSync())
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
