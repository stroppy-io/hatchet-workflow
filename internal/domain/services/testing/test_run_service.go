package testing

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/yaroher/ratel/pkg/dml/set"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
)

// safeSelectColsRun lists the columns we can safely scan from the test_runs
// table. DatabaseVariantDatabasePresetId and WorkloadVariantWorkloadPresetId
// are omitted: the generated scanner stores them as plain string (not *string),
// so pgx cannot scan NULL into those targets.
var safeSelectColsRun = []testingpb.TestRunColumnAlias{
	testingpb.TestRunColumnId,
	testingpb.TestRunColumnTenantId,
	testingpb.TestRunColumnCreatedAt,
	testingpb.TestRunColumnUpdatedAt,
	testingpb.TestRunColumnDeletedAt,
	testingpb.TestRunColumnName,
	testingpb.TestRunColumnDescription,
	testingpb.TestRunColumnLabel,
	testingpb.TestRunColumnSuiteRunId,
	testingpb.TestRunColumnTemplateId,
	testingpb.TestRunColumnDagRunId,
	testingpb.TestRunColumnCreatedBy,
	testingpb.TestRunColumnDatabaseVariantDatabase,
	testingpb.TestRunColumnDatabaseVariantCase,
	testingpb.TestRunColumnWorkloadVariantWorkload,
	testingpb.TestRunColumnWorkloadVariantCase,
}

// runScannerSetters returns all setters for a TestRunScanner, converting
// empty-string FK fields to nil so that PostgreSQL stores NULL instead of an
// empty string (which would violate FK constraints).
func runScannerSetters(s *testingpb.TestRunScanner) []set.ValueSetter[testingpb.TestRunColumnAlias] {
	var dbPresetID any = s.DatabaseVariantDatabasePresetId
	if s.DatabaseVariantDatabasePresetId == "" {
		dbPresetID = nil
	}
	var wlPresetID any = s.WorkloadVariantWorkloadPresetId
	if s.WorkloadVariantWorkloadPresetId == "" {
		wlPresetID = nil
	}
	var suiteRunID any = s.SuiteRunId
	if s.SuiteRunId == nil {
		suiteRunID = nil
	}
	var templateID any = s.TemplateId
	if s.TemplateId == nil {
		templateID = nil
	}
	var dagRunID any = s.DagRunId
	if s.DagRunId == nil {
		dagRunID = nil
	}
	var createdBy any = s.CreatedBy
	if s.CreatedBy == nil {
		createdBy = nil
	}
	return []set.ValueSetter[testingpb.TestRunColumnAlias]{
		set.NewSetter[testingpb.TestRunColumnAlias](testingpb.TestRunColumnId, s.Id),
		set.NewSetter[testingpb.TestRunColumnAlias](testingpb.TestRunColumnTenantId, s.TenantId),
		set.NewSetter[testingpb.TestRunColumnAlias](testingpb.TestRunColumnCreatedAt, s.CreatedAt),
		set.NewSetter[testingpb.TestRunColumnAlias](testingpb.TestRunColumnUpdatedAt, s.UpdatedAt),
		set.NewSetter[testingpb.TestRunColumnAlias](testingpb.TestRunColumnDeletedAt, s.DeletedAt),
		set.NewSetter[testingpb.TestRunColumnAlias](testingpb.TestRunColumnName, s.Name),
		set.NewSetter[testingpb.TestRunColumnAlias](testingpb.TestRunColumnDescription, s.Description),
		set.NewSetter[testingpb.TestRunColumnAlias](testingpb.TestRunColumnLabel, s.Label),
		set.NewSetter[testingpb.TestRunColumnAlias](testingpb.TestRunColumnSuiteRunId, suiteRunID),
		set.NewSetter[testingpb.TestRunColumnAlias](testingpb.TestRunColumnTemplateId, templateID),
		set.NewSetter[testingpb.TestRunColumnAlias](testingpb.TestRunColumnDagRunId, dagRunID),
		set.NewSetter[testingpb.TestRunColumnAlias](testingpb.TestRunColumnCreatedBy, createdBy),
		set.NewSetter[testingpb.TestRunColumnAlias](testingpb.TestRunColumnDatabaseVariantDatabasePresetId, dbPresetID),
		set.NewSetter[testingpb.TestRunColumnAlias](testingpb.TestRunColumnDatabaseVariantDatabase, s.DatabaseVariantDatabase),
		set.NewSetter[testingpb.TestRunColumnAlias](testingpb.TestRunColumnDatabaseVariantCase, s.DatabaseVariantCase),
		set.NewSetter[testingpb.TestRunColumnAlias](testingpb.TestRunColumnWorkloadVariantWorkloadPresetId, wlPresetID),
		set.NewSetter[testingpb.TestRunColumnAlias](testingpb.TestRunColumnWorkloadVariantWorkload, s.WorkloadVariantWorkload),
		set.NewSetter[testingpb.TestRunColumnAlias](testingpb.TestRunColumnWorkloadVariantCase, s.WorkloadVariantCase),
	}
}

// TestRunService provides CRUD and lifecycle operations for TestRun entities.
type TestRunService struct {
	*tracing.Entity

	runs    *repository.ProtoRepository[testingpb.TestRunAlias, testingpb.TestRunColumnAlias, *testingpb.TestRunScanner, *testingpb.TestRun]
	txMgr   pgtx.TxManager
	events  eventing.Bus
	catalog CatalogPort
	engine  SystemEnginePort
	builder DagBuilderPort
}

// NewTestRunService constructs a TestRunService.
func NewTestRunService(
	executor exec.DB,
	txMgr pgtx.TxManager,
	events eventing.Bus,
	catalog CatalogPort,
	engine SystemEnginePort,
	builder DagBuilderPort,
) *TestRunService {
	return &TestRunService{
		Entity: tracing.NewEntity("testing.TestRunService"),
		runs: repository.NewProtoRepository(
			repository.NewScannerRepository(testingpb.TestRuns.Table, executor),
			testingpb.TestRunConverter,
		),
		txMgr:   txMgr,
		events:  events,
		catalog: catalog,
		engine:  engine,
		builder: builder,
	}
}

// CreateTestRun assigns an ID, tenant, caller and timestamps, then INSERTs in a
// serializable transaction.
func (s *TestRunService) CreateTestRun(
	ctx context.Context,
	tenantID *iampb.TenantId,
	callerID *iampb.UserId,
	tr *testingpb.TestRun,
) (*testingpb.TestRun, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "CreateTestRun",
		func(ctx context.Context, _ trace.Span) (*testingpb.TestRun, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr,
				func(ctx context.Context) (*testingpb.TestRun, error) {
					now := timestamppb.Now()
					tr.Id = &testingpb.TestRunId{Value: ids.New()}
					tr.TenantId = tenantID
					tr.CreatedBy = callerID
					tr.Timestamps = &commonpb.Timestamps{CreatedAt: now, UpdatedAt: now}

					scanner := tr.IntoPlain()
					if scanner.Label == nil {
						scanner.Label = []string{}
					}
					// IntoPlain sets []byte{} for unset JSONB columns; PostgreSQL
					// rejects empty bytes as invalid JSON — use nil instead.
					if len(scanner.DatabaseVariantDatabase) == 0 {
						scanner.DatabaseVariantDatabase = nil
					}
					if len(scanner.WorkloadVariantWorkload) == 0 {
						scanner.WorkloadVariantWorkload = nil
					}

					if _, err := s.runs.Execute(ctx,
						testingpb.TestRuns.Insert().From(runScannerSetters(scanner)...),
					); err != nil {
						return nil, err
					}
					return tr, nil
				})
		})
}

// GetTestRun retrieves a single (non-deleted) TestRun by ID.
func (s *TestRunService) GetTestRun(
	ctx context.Context,
	id *testingpb.TestRunId,
) (*testingpb.TestRun, error) {
	p, err := s.runs.QueryRow(ctx,
		testingpb.TestRuns.Select(safeSelectColsRun...).Where(
			testingpb.TestRuns.Id.Eq(id.GetValue()),
			testingpb.TestRuns.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("test_run", id.GetValue()))
		}
		return nil, err
	}
	return p, nil
}

// ListTestRuns returns all non-deleted TestRuns belonging to tenantID.
func (s *TestRunService) ListTestRuns(
	ctx context.Context,
	tenantID *iampb.TenantId,
) ([]*testingpb.TestRun, error) {
	return s.runs.Query(ctx,
		testingpb.TestRuns.Select(safeSelectColsRun...).Where(
			testingpb.TestRuns.TenantId.Eq(tenantID.GetValue()),
			testingpb.TestRuns.DeletedAt.IsNull(),
		),
	)
}

// LaunchTestRun builds a Dag from the TestRun's spec, starts a DagRun, and
// writes the DagRunId back to the TestRun row.
//
// Returns FailedPrecondition if the TestRun was already launched (DagRunId is
// set).
func (s *TestRunService) LaunchTestRun(
	ctx context.Context,
	id *testingpb.TestRunId,
) (*testingpb.TestRun, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "LaunchTestRun",
		func(ctx context.Context, _ trace.Span) (*testingpb.TestRun, error) {
			tr, err := s.GetTestRun(ctx, id)
			if err != nil {
				return nil, err
			}
			if tr.GetDagRunId() != nil {
				return nil, status.Errorf(codes.FailedPrecondition,
					"test_run %s is already launched (dag_run_id=%s)",
					id.GetValue(), tr.GetDagRunId().GetValue())
			}

			dag, err := s.builder.FromTestRun(ctx, tr)
			if err != nil {
				return nil, fmt.Errorf("LaunchTestRun: build dag: %w", err)
			}

			dagRun, err := s.engine.LaunchDag(ctx, dag, map[string]string{
				"test_run_id": id.GetValue(),
				"tenant_id":   tr.GetTenantId().GetValue(),
			})
			if err != nil {
				return nil, fmt.Errorf("LaunchTestRun: launch dag: %w", err)
			}

			// Persist DagRunId back to test_run row.
			dagRunIDVal := dagRun.GetId().GetValue()
			n, err := s.runs.Execute(ctx,
				testingpb.TestRuns.Update().
					Set(set.NewSetter[testingpb.TestRunColumnAlias](
						testingpb.TestRunColumnDagRunId, &dagRunIDVal,
					)).
					Where(testingpb.TestRuns.Id.Eq(id.GetValue())),
			)
			if err != nil {
				return nil, fmt.Errorf("LaunchTestRun: update dag_run_id: %w", err)
			}
			if n == 0 {
				return nil, domainerr.NotFound(domainerr.ResourceInfo("test_run", id.GetValue()))
			}

			tr.DagRunId = dagRun.GetId()
			return tr, nil
		})
}

// CancelTestRun is not yet implemented (requires proto change for cancel_requested).
func (s *TestRunService) CancelTestRun(_ context.Context, _ *testingpb.TestRunId) error {
	return fmt.Errorf("CancelTestRun: not implemented (cancel pending proto change)")
}
