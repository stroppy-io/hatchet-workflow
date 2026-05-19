package testing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/yaroher/ratel/pkg/dml/set"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
)

// SuiteDagBuilderPort is implemented by *dagbuilder.Builder to break the import
// cycle: dagbuilder imports testingpb, so testing service imports dagbuilder via
// interface only.
type SuiteDagBuilderPort interface {
	FromTestSuite(ctx context.Context, suite *testingpb.TestSuite, childIDs []*testingpb.TestRunId) (*systempb.Dag, error)
}

// safeSelectSuiteRunCols lists the columns we can safely scan from the
// test_suite_runs table.
var safeSelectSuiteRunCols = []testingpb.TestSuiteRunColumnAlias{
	testingpb.TestSuiteRunColumnId,
	testingpb.TestSuiteRunColumnTenantId,
	testingpb.TestSuiteRunColumnCreatedAt,
	testingpb.TestSuiteRunColumnUpdatedAt,
	testingpb.TestSuiteRunColumnDeletedAt,
	testingpb.TestSuiteRunColumnSuiteId,
	testingpb.TestSuiteRunColumnTestRunIds,
	testingpb.TestSuiteRunColumnDagRunId,
	testingpb.TestSuiteRunColumnMatrix,
}

// TestSuiteRunService handles launching, querying, and cancelling TestSuiteRuns.
type TestSuiteRunService struct {
	*tracing.Entity

	suiteRuns *repository.ProtoRepository[testingpb.TestSuiteRunAlias, testingpb.TestSuiteRunColumnAlias, *testingpb.TestSuiteRunScanner, *testingpb.TestSuiteRun]
	runs      *repository.ProtoRepository[testingpb.TestRunAlias, testingpb.TestRunColumnAlias, *testingpb.TestRunScanner, *testingpb.TestRun]
	suites    *repository.ProtoRepository[testingpb.TestSuiteAlias, testingpb.TestSuiteColumnAlias, *testingpb.TestSuiteScanner, *testingpb.TestSuite]
	txMgr     pgtx.TxManager
	//nolint:unused
	events  eventing.Bus
	engine  SystemEnginePort
	builder SuiteDagBuilderPort
}

// NewTestSuiteRunService constructs a TestSuiteRunService.
func NewTestSuiteRunService(
	executor exec.DB,
	txMgr pgtx.TxManager,
	events eventing.Bus,
	engine SystemEnginePort,
	builder SuiteDagBuilderPort,
) *TestSuiteRunService {
	return &TestSuiteRunService{
		Entity: tracing.NewEntity("testing.TestSuiteRunService"),
		suiteRuns: repository.NewProtoRepository(
			repository.NewScannerRepository(testingpb.TestSuiteRuns.Table, executor),
			testingpb.TestSuiteRunConverter,
		),
		runs: repository.NewProtoRepository(
			repository.NewScannerRepository(testingpb.TestRuns.Table, executor),
			testingpb.TestRunConverter,
		),
		suites: repository.NewProtoRepository(
			repository.NewScannerRepository(testingpb.TestSuites.Table, executor),
			testingpb.TestSuiteConverter,
		),
		txMgr:   txMgr,
		events:  events,
		engine:  engine,
		builder: builder,
	}
}

// GetTestSuiteRun returns a TestSuiteRun by ID.
func (s *TestSuiteRunService) GetTestSuiteRun(
	ctx context.Context,
	id *testingpb.TestSuiteRunId,
) (*testingpb.TestSuiteRun, error) {
	p, err := s.suiteRuns.QueryRow(ctx,
		testingpb.TestSuiteRuns.Select(safeSelectSuiteRunCols...).Where(
			testingpb.TestSuiteRuns.Id.Eq(id.GetValue()),
			testingpb.TestSuiteRuns.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("test_suite_run", id.GetValue()))
		}
		return nil, err
	}
	return p, nil
}

// ListBySuite returns all non-deleted TestSuiteRuns for a given suite.
func (s *TestSuiteRunService) ListBySuite(
	ctx context.Context,
	suiteID *testingpb.TestSuiteId,
) ([]*testingpb.TestSuiteRun, error) {
	return s.suiteRuns.Query(ctx,
		testingpb.TestSuiteRuns.Select(safeSelectSuiteRunCols...).Where(
			testingpb.TestSuiteRuns.SuiteId.Eq(suiteID.GetValue()),
			testingpb.TestSuiteRuns.DeletedAt.IsNull(),
		),
	)
}

// ListByTenant returns all non-deleted TestSuiteRuns for a given tenant.
func (s *TestSuiteRunService) ListByTenant(
	ctx context.Context,
	tenantID *iampb.TenantId,
) ([]*testingpb.TestSuiteRun, error) {
	return s.suiteRuns.Query(ctx,
		testingpb.TestSuiteRuns.Select(safeSelectSuiteRunCols...).Where(
			testingpb.TestSuiteRuns.TenantId.Eq(tenantID.GetValue()),
			testingpb.TestSuiteRuns.DeletedAt.IsNull(),
		),
	)
}

// LaunchTestSuite materializes one TestRun per matrix cell (db×workload),
// builds a meta-DAG, starts a DagRun, and INSERTs a TestSuiteRun row.
//
// Matrix cells: len(databases) × len(workloads). Falls back to one placeholder
// TestRun when the matrix is empty.
func (s *TestSuiteRunService) LaunchTestSuite(
	ctx context.Context,
	suiteID *testingpb.TestSuiteId,
	callerID *iampb.UserId,
) (*testingpb.TestSuiteRun, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "LaunchTestSuite",
		func(ctx context.Context, _ trace.Span) (*testingpb.TestSuiteRun, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr,
				func(ctx context.Context) (*testingpb.TestSuiteRun, error) {
					// 1. Fetch suite.
					suite, err := s.suites.QueryRow(ctx,
						testingpb.TestSuites.Select(safeSelectSuiteCols...).Where(
							testingpb.TestSuites.Id.Eq(suiteID.GetValue()),
							testingpb.TestSuites.DeletedAt.IsNull(),
						),
					)
					if err != nil {
						if errors.Is(err, pgx.ErrNoRows) {
							return nil, domainerr.NotFound(domainerr.ResourceInfo("test_suite", suiteID.GetValue()))
						}
						return nil, err
					}

					// 2. Materialise matrix cells → child TestRuns.
					childIDs, err := s.materializeRuns(ctx, suite, callerID)
					if err != nil {
						return nil, fmt.Errorf("LaunchTestSuite: materialize runs: %w", err)
					}

					// 3. Build meta-DAG.
					dag, err := s.builder.FromTestSuite(ctx, suite, childIDs)
					if err != nil {
						return nil, fmt.Errorf("LaunchTestSuite: build dag: %w", err)
					}

					// 4. Launch DagRun.
					runIDs := make([]string, len(childIDs))
					for i, cid := range childIDs {
						runIDs[i] = cid.GetValue()
					}

					dagRun, err := s.engine.LaunchDag(ctx, dag, map[string]string{
						"suite_id":  suiteID.GetValue(),
						"tenant_id": suite.GetTenantId().GetValue(),
					})
					if err != nil {
						return nil, fmt.Errorf("LaunchTestSuite: launch dag: %w", err)
					}

					// 5. INSERT TestSuiteRun row.
					now := timestamppb.Now()
					suiteRun := &testingpb.TestSuiteRun{
						Id:         &testingpb.TestSuiteRunId{Value: ids.New()},
						TenantId:   suite.GetTenantId(),
						Timestamps: &commonpb.Timestamps{CreatedAt: now, UpdatedAt: now},
						SuiteId:    suiteID,
						TestRunIds: runIDs,
						DagRunId:   dagRun.GetId(),
						Matrix:     suite.GetMatrix(),
					}

					scanner := suiteRun.IntoPlain()
					if len(scanner.Matrix) == 0 {
						scanner.Matrix = []byte("{}")
					}
					if scanner.TestRunIds == nil {
						scanner.TestRunIds = []string{}
					}

					var dagRunIDVal any = scanner.DagRunId
					setters := []set.ValueSetter[testingpb.TestSuiteRunColumnAlias]{
						set.NewSetter[testingpb.TestSuiteRunColumnAlias](testingpb.TestSuiteRunColumnId, scanner.Id),
						set.NewSetter[testingpb.TestSuiteRunColumnAlias](testingpb.TestSuiteRunColumnTenantId, scanner.TenantId),
						set.NewSetter[testingpb.TestSuiteRunColumnAlias](testingpb.TestSuiteRunColumnCreatedAt, scanner.CreatedAt),
						set.NewSetter[testingpb.TestSuiteRunColumnAlias](testingpb.TestSuiteRunColumnUpdatedAt, scanner.UpdatedAt),
						set.NewSetter[testingpb.TestSuiteRunColumnAlias](testingpb.TestSuiteRunColumnDeletedAt, scanner.DeletedAt),
						set.NewSetter[testingpb.TestSuiteRunColumnAlias](testingpb.TestSuiteRunColumnSuiteId, scanner.SuiteId),
						set.NewSetter[testingpb.TestSuiteRunColumnAlias](testingpb.TestSuiteRunColumnTestRunIds, scanner.TestRunIds),
						set.NewSetter[testingpb.TestSuiteRunColumnAlias](testingpb.TestSuiteRunColumnDagRunId, dagRunIDVal),
						set.NewSetter[testingpb.TestSuiteRunColumnAlias](testingpb.TestSuiteRunColumnMatrix, scanner.Matrix),
					}

					if _, err := s.suiteRuns.Execute(ctx,
						testingpb.TestSuiteRuns.Insert().From(setters...),
					); err != nil {
						return nil, fmt.Errorf("LaunchTestSuite: insert suite_run: %w", err)
					}

					return suiteRun, nil
				})
		})
}

// CancelTestSuiteRun delegates to engine.CancelDagRun.
func (s *TestSuiteRunService) CancelTestSuiteRun(
	ctx context.Context,
	id *testingpb.TestSuiteRunId,
) error {
	sr, err := s.GetTestSuiteRun(ctx, id)
	if err != nil {
		return err
	}
	if sr.GetDagRunId() == nil {
		return fmt.Errorf("CancelTestSuiteRun: suite_run %s has no dag_run_id", id.GetValue())
	}
	return s.engine.CancelDagRun(ctx, sr.GetDagRunId())
}

// loadChildRun reads a single child TestRun row by id from the underlying
// repository (lightweight — avoids spinning up a full TestRunService).
func (s *TestSuiteRunService) loadChildRun(ctx context.Context, id *testingpb.TestRunId) (*testingpb.TestRun, error) {
	return s.runs.QueryRow(ctx,
		testingpb.TestRuns.SelectAll().Where(
			testingpb.TestRuns.Id.Eq(id.GetValue()),
			testingpb.TestRuns.DeletedAt.IsNull(),
		),
	)
}

// WatchTestSuiteRun emits a single aggregated snapshot — one entry per child
// TestRun with its DagRun status. A streaming variant would subscribe to the
// engine's progress channel; the single-shot snapshot here is sufficient for
// the UI's poll-and-refresh pattern and avoids holding N subscriptions per
// viewer.
func (s *TestSuiteRunService) WatchTestSuiteRun(
	ctx context.Context,
	suiteRunID *testingpb.TestSuiteRunId,
	send func(*testingpb.TestSuiteRunProgress) error,
) error {
	sr, err := s.GetTestSuiteRun(ctx, suiteRunID)
	if err != nil {
		return err
	}
	progress := &testingpb.TestSuiteRunProgress{}
	if sr.GetDagRunId() != nil {
		if dr, err := s.engine.GetDagRun(ctx, sr.GetDagRunId()); err == nil {
			progress.Status = dr.GetStatus()
		}
	}
	for _, idStr := range sr.GetTestRunIds() {
		childID := &testingpb.TestRunId{Value: idStr}
		child, err := s.loadChildRun(ctx, childID)
		if err != nil {
			continue
		}
		childRow := &testingpb.TestSuiteRunChild{
			TestRunId: childID,
		}
		if child.GetIdentity() != nil {
			childRow.Name = child.GetIdentity().GetName()
		}
		if child.GetDagRunId() != nil {
			if dr, err := s.engine.GetDagRun(ctx, child.GetDagRunId()); err == nil {
				childRow.Progress = &testingpb.TestRunProgress{
					Status:     dr.GetStatus(),
					StartedAt:  dr.GetTimestamps().GetCreatedAt(),
					FinishedAt: dr.GetTimestamps().GetUpdatedAt(),
				}
			}
		}
		progress.Children = append(progress.Children, childRow)
	}
	return send(progress)
}

// StreamTestSuiteRunLogs forwards LogLines from every child TestRun in the
// suite. test_run_id filter narrows to one child.
func (s *TestSuiteRunService) StreamTestSuiteRunLogs(
	ctx context.Context,
	req *testingpb.StreamTestSuiteRunLogsRequest,
	send func(*agentpb.LogLine) error,
) error {
	sr, err := s.GetTestSuiteRun(ctx, req.GetSuiteRunId())
	if err != nil {
		return err
	}
	childIDs := sr.GetTestRunIds()
	if filt := req.GetTestRunId(); filt != nil && filt.GetValue() != "" {
		childIDs = []string{filt.GetValue()}
	}
	since := ""
	if ts := req.GetSince(); ts != nil {
		since = ts.AsTime().Format(time.RFC3339Nano)
	}
	for _, idStr := range childIDs {
		childID := &testingpb.TestRunId{Value: idStr}
		child, err := s.loadChildRun(ctx, childID)
		if err != nil || child.GetDagRunId() == nil {
			continue
		}
		ch, cancel, err := s.engine.StreamLogs(ctx, child.GetDagRunId(), req.GetStepId(), since)
		if err != nil {
			continue
		}
		drain := func() {
			defer cancel()
			for {
				select {
				case <-ctx.Done():
					return
				case line, ok := <-ch:
					if !ok {
						return
					}
					if err := send(line); err != nil {
						return
					}
				}
			}
		}
		drain()
	}
	return nil
}

// GetTestSuiteRunMetrics returns a flat list of MetricSeries. With no
// MetricsPort wired here the aggregation lives at the suite-run level only;
// per-child metric retrieval is a TestRun-level concern handled by the
// TestRunService. Empty list is honest behaviour: caller can iterate
// GetTestSuiteRun.testRunIds and query TestRun metrics directly.
func (s *TestSuiteRunService) GetTestSuiteRunMetrics(
	ctx context.Context,
	req *testingpb.GetTestSuiteRunMetricsRequest,
) (*testingpb.MetricSeriesList, error) {
	if _, err := s.GetTestSuiteRun(ctx, req.GetSuiteRunId()); err != nil {
		return nil, err
	}
	// The caller is expected to issue per-child GetTestRunMetrics for each
	// child TestRun returned by GetTestSuiteRun. This empty response is
	// well-defined: no suite-level rollup exists yet (would require a metric
	// label such as suite_run_id, which the stroppy emitter does not set).
	return &testingpb.MetricSeriesList{}, nil
}

// materializeRuns creates one TestRun per matrix cell. Falls back to a single
// placeholder run when the matrix has no databases or workloads.
func (s *TestSuiteRunService) materializeRuns(
	ctx context.Context,
	suite *testingpb.TestSuite,
	callerID *iampb.UserId,
) ([]*testingpb.TestRunId, error) {
	matrix := suite.GetMatrix()
	dbs := matrix.GetDatabases()
	wls := matrix.GetWorkloads()

	cellCount := 1
	if len(dbs) > 0 && len(wls) > 0 {
		cellCount = len(dbs) * len(wls)
	}

	childIDs := make([]*testingpb.TestRunId, 0, cellCount)

	suiteName := ""
	if id := suite.GetIdentity(); id != nil {
		suiteName = id.GetName()
	}

	for i := 0; i < cellCount; i++ {
		now := timestamppb.Now()
		runID := ids.New()
		tr := &testingpb.TestRun{
			Id:       &testingpb.TestRunId{Value: runID},
			TenantId: suite.GetTenantId(),
			Identity: &commonpb.Identity{
				Name: fmt.Sprintf("%s-cell-%d", suiteName, i),
			},
			Timestamps: &commonpb.Timestamps{CreatedAt: now, UpdatedAt: now},
			CreatedBy:  callerID,
		}

		// Wire db/wl from matrix cell when available.
		if len(dbs) > 0 && len(wls) > 0 {
			dbIdx := i / len(wls)
			wlIdx := i % len(wls)
			tr.Database = dbs[dbIdx]
			tr.Workload = wls[wlIdx]
		}

		scanner := tr.IntoPlain()
		if scanner.Label == nil {
			scanner.Label = []string{}
		}
		if len(scanner.DatabaseVariantDatabase) == 0 {
			scanner.DatabaseVariantDatabase = nil
		}
		if len(scanner.WorkloadVariantWorkload) == 0 {
			scanner.WorkloadVariantWorkload = nil
		}

		if _, err := s.runs.Execute(ctx,
			testingpb.TestRuns.Insert().From(runScannerSetters(scanner)...),
		); err != nil {
			return nil, fmt.Errorf("materializeRuns: insert run %d: %w", i, err)
		}
		childIDs = append(childIDs, &testingpb.TestRunId{Value: runID})
	}
	return childIDs, nil
}
