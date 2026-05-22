// Package run implements the ui RunService: TestRun lifecycle (submit/get/list/
// cancel) plus logs, metrics, comparison and share. Submitting compiles the
// TestPreset into a Dag (planner, C12), persists it (dagstore), and lets the
// DagProcessor run it; run status == Dag.status (H22). Logs/metrics/share are
// delegated to injected clients (their backends — VictoriaLogs/Metrics, share
// store — are not built yet; see run_external.go).
package run

import (
	"context"

	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/gopherex/xlog"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	uiapi "github.com/stroppy-io/stroppy-cloud/internal/api/ui"
	dagdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"github.com/stroppy-io/stroppy-cloud/internal/runtime"
	"github.com/stroppy-io/stroppy-cloud/internal/services/authz"
	"github.com/stroppy-io/stroppy-cloud/internal/services/svcutil"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// RunService implements ui.RunActions.
type RunService struct {
	*tracing.Entity
	testRuns *repository.ProtoRepository[
		models.TestRunAlias,
		models.TestRunColumnAlias,
		*models.TestRunScanner,
		*models.TestRun,
	]
	provider ProviderResolver
	store    DagStore
	logs     LogsClient
	metrics  MetricsClient
	share    ShareStore
	authz    *authz.Authz
	txm      tx.Trm
}

var _ uiapi.RunActions = (*RunService)(nil)

// New builds a RunService. planner/provider/store drive the lifecycle;
// logs/metrics/share are injected clients (impls pending — see run_external.go).
func New(
	logger *xlog.Logger,
	executor exec.DB,
	txm tx.Trm,
	az *authz.Authz,
	provider ProviderResolver,
	store DagStore,
	logs LogsClient,
	metrics MetricsClient,
	share ShareStore,
) *RunService {
	return &RunService{
		Entity: tracing.NewEntity(logger.AppendName("RunService")),
		testRuns: repository.NewProtoRepository(
			repository.NewScannerRepository(models.TestRuns.Table, executor),
			models.TestRunConverter,
		),
		provider: provider,
		store:    store,
		logs:     logs,
		metrics:  metrics,
		share:    share,
		authz:    az,
		txm:      txm,
	}
}

// SubmitTestRun compiles the preset into a Dag, persists the Dag + TestRun, and
// returns the run. The DagProcessor picks the Dag up from storage.
func (s *RunService) SubmitTestRun(ctx context.Context, req *uipb.SubmitTestRunRequest) (*models.TestRun, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "SubmitTestRun",
		func(ctx context.Context, _ trace.Span) (*models.TestRun, error) {
			c := svcutil.CallerOf(ctx)
			if err := s.authz.Require(ctx, c, req.GetTenantId(), models.TenantMember_ROLE_ADMIN); err != nil {
				return nil, err
			}
			dep, err := s.provider.Resolve(ctx, req.GetTenantId().GetValue(), req.GetTestPreset())
			if err != nil {
				return nil, status.Errorf(codes.Internal, "resolve deployment: %v", err)
			}
			dag := dagdomain.BuildTestDag(req.GetTestPreset(), dep, dagdomain.Deps{Install: dagdomain.RecipeInstallBuilder{}})
			if dag.Metadata == nil {
				dag.Metadata = map[string]string{}
			}
			dag.Metadata[metadataTenantID] = req.GetTenantId().GetValue()
			if err := runtime.ValidateDag(dag); err != nil {
				return nil, status.Errorf(codes.Internal, "invalid dag: %v", err)
			}
			run := &models.TestRun{
				Entity:      ids.NewEntity(),
				Owned:       &models.Own{OwnerAccountId: c.AccountID, TenantId: req.GetTenantId()},
				Name:        req.Name,
				Description: req.Description,
				TestPreset:  req.GetTestPreset(),
				Dag:         &models.DagId{Value: dag.GetId()},
			}
			return tx.DoReadCommittedRet(ctx, s.txm, func(ctx context.Context) (*models.TestRun, error) {
				if err := s.store.SaveDag(ctx, dag); err != nil {
					return nil, status.Errorf(codes.Internal, "persist dag: %v", err)
				}
				if _, err := s.testRuns.Execute(ctx,
					models.TestRuns.Insert().From(run.IntoPlain().AllSetters()...)); err != nil {
					return nil, status.Errorf(codes.Internal, "insert test run: %v", err)
				}
				return run, nil
			})
		})
}

// GetTestRun returns a run by id, hydrating the run's live status from its Dag
// (the live status lives on Dag.status, H22 — not on the TestRun row).
func (s *RunService) GetTestRun(ctx context.Context, req *uipb.GetTestRunRequest) (*models.TestRun, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "GetTestRun",
		func(ctx context.Context, _ trace.Span) (*models.TestRun, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			run, err := s.loadRun(ctx, req.GetTenantId().GetValue(), req.GetId().GetValue())
			if err != nil {
				return nil, err
			}
			s.hydrateStatus(ctx, run)
			return run, nil
		})
}

// hydrateStatus loads the run's Dag and sets the run's live status from it. The
// status is authoritative on Dag.status (H22); TestRun.status is the read-time
// mirror (the persisted column is best-effort).
func (s *RunService) hydrateStatus(ctx context.Context, run *models.TestRun) {
	if run.GetDag().GetValue() == "" {
		return
	}
	dag, err := s.store.GetDag(ctx, run.GetDag().GetValue())
	if err != nil {
		s.Logger().Warn("hydrate run status: load dag", xlog.String("dag_id", run.GetDag().GetValue()), xlog.Error("error", err))
		return
	}
	if dag == nil {
		return
	}
	run.Status = dag.GetStatus() // live run status mirrors the Dag (H22)
}

// ListTestRuns returns the tenant's runs with cursor pagination (newest-first
// by default).
func (s *RunService) ListTestRuns(ctx context.Context, req *uipb.ListTestRunsRequest) (*uipb.ListTestRunsResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListTestRuns",
		func(ctx context.Context, _ trace.Span) (*uipb.ListTestRunsResponse, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			size := svcutil.PageSize(req.GetPage())
			desc := svcutil.CursorDesc(req.GetOrder())
			q := models.TestRuns.SelectAll().Where(
				models.TestRuns.TenantId.Eq(req.GetTenantId().GetValue()),
				models.TestRuns.DeletedAt.IsNull(),
			)
			// status filtering needs the Dag status (not a run column); use the Dag list or denormalize status — tracked as a schema follow-up.
			// search: match the run name (NullText column → raw ILIKE).
			if req.GetSearch() != "" {
				q = q.Where(models.TestRuns.Name.Raw("ILIKE", "?", "%"+req.GetSearch()+"%"))
			}
			// tags: Tags is serialized JSON of common.Tags (a TEXT column) — match
			// each requested free tag and key=value label as a substring.
			for _, tag := range req.GetTags().GetTags() {
				q = q.Where(models.TestRuns.Tags.ILike("%" + tag + "%"))
			}
			for k, v := range req.GetTags().GetLabels() {
				q = q.Where(models.TestRuns.Tags.ILike("%" + k + "%" + v + "%"))
			}
			if tok := req.GetPage().GetToken(); tok != "" {
				if desc {
					q = q.Where(models.TestRuns.Id.Lt(tok))
				} else {
					q = q.Where(models.TestRuns.Id.Gt(tok))
				}
			}
			if desc {
				q = q.OrderByDESC(models.TestRunColumnId)
			} else {
				q = q.OrderByASC(models.TestRunColumnId)
			}
			rows, err := s.testRuns.Query(ctx, q.Limit(size+1))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "list test runs: %v", err)
			}
			items, pageInfo := svcutil.Paginate(rows, size, func(r *models.TestRun) string {
				return r.GetEntity().GetId().GetValue()
			})
			// Hydrate each run's live status from its Dag (Dag.status, H22). Batch
			// is a per-run GetDag (no bulk-by-ids primitive on DagStore yet).
			for _, run := range items {
				s.hydrateStatus(ctx, run)
			}
			return &uipb.ListTestRunsResponse{TestRuns: items, PageInfo: pageInfo}, nil
		})
}

// CancelTestRun moves the run's Dag to CANCELLING; the executor runs always_run
// teardown and settles the terminal state.
func (s *RunService) CancelTestRun(ctx context.Context, req *uipb.CancelTestRunRequest) (*models.TestRun, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CancelTestRun",
		func(ctx context.Context, _ trace.Span) (*models.TestRun, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_ADMIN); err != nil {
				return nil, err
			}
			run, err := s.loadRun(ctx, req.GetTenantId().GetValue(), req.GetId().GetValue())
			if err != nil {
				return nil, err
			}
			return tx.DoReadCommittedRet(ctx, s.txm, func(ctx context.Context) (*models.TestRun, error) {
				dag, err := s.store.GetDag(ctx, run.GetDag().GetValue())
				if err != nil {
					return nil, status.Errorf(codes.Internal, "load dag: %v", err)
				}
				if dag == nil {
					return nil, status.Error(codes.NotFound, "run dag not found")
				}
				if !isTerminal(dag.GetStatus()) {
					dag.Status = primitive.Status_STATUS_CANCELLING
					if err := s.store.SaveDag(ctx, dag); err != nil {
						return nil, status.Errorf(codes.Internal, "cancel dag: %v", err)
					}
				}
				return run, nil
			})
		})
}

func (s *RunService) loadRun(ctx context.Context, tenantID, runID string) (*models.TestRun, error) {
	run, err := s.testRuns.QueryRow(ctx,
		models.TestRuns.SelectAll().Where(
			models.TestRuns.Id.Eq(runID),
			models.TestRuns.TenantId.Eq(tenantID),
			models.TestRuns.DeletedAt.IsNull(),
		))
	if err != nil {
		return nil, svcutil.NotFound(err, "test run")
	}
	return run, nil
}

const metadataTenantID = "tenant_id"

func isTerminal(st primitive.Status) bool {
	switch st {
	case primitive.Status_STATUS_COMPLETED,
		primitive.Status_STATUS_FAILED,
		primitive.Status_STATUS_CANCELLED:
		return true
	default:
		return false
	}
}
