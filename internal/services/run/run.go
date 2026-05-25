// Package run implements the ui RunService: TestRun lifecycle (submit/get/list/
// cancel) plus logs, metrics, comparison and share. Submitting compiles the
// TestPreset into a Dag (planner, C12), persists it (dagstore), and lets the
// DagProcessor run it; run status == Dag.status (H22). Logs/metrics/share are
// delegated to injected clients (their backends — VictoriaLogs/Metrics, share
// store — are not built yet; see run_external.go).
package run

import (
	"context"
	"strings"

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
	agents *repository.ProtoRepository[
		models.AgentAlias,
		models.AgentColumnAlias,
		*models.AgentScanner,
		*models.Agent,
	]
	accounts *repository.ProtoRepository[
		models.AccountAlias,
		models.AccountColumnAlias,
		*models.AccountScanner,
		*models.Account,
	]
	db        exec.DB
	provider  ProviderResolver
	store     DagStore
	logs      LogsClient
	metrics   MetricsClient
	share     ShareStore
	authz     *authz.Authz
	txm       tx.Trm
	canceller Canceller
}

// Canceller is the runtime cancellation API (satisfied by runtime.DagProcessor).
// CancelTestRun delegates to it so the runtime drives the CANCELLING→teardown→
// CANCELLED transition itself — the service never writes the dag status directly.
type Canceller interface {
	Cancel(dagID string)
}

// SetCanceller wires the runtime canceller after construction (the processor is
// built after this service).
func (s *RunService) SetCanceller(c Canceller) { s.canceller = c }

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
		agents: repository.NewProtoRepository(
			repository.NewScannerRepository(models.Agents.Table, executor),
			models.AgentConverter,
		),
		accounts: repository.NewProtoRepository(
			repository.NewScannerRepository(models.Accounts.Table, executor),
			models.AccountConverter,
		),
		db:       executor,
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

// GetTestRunDag returns the run's executing Dag (nodes/edges/status) for the graph view.
func (s *RunService) GetTestRunDag(ctx context.Context, req *uipb.GetTestRunRequest) (*primitive.Dag, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "GetTestRunDag",
		func(ctx context.Context, _ trace.Span) (*primitive.Dag, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			run, err := s.loadRun(ctx, req.GetTenantId().GetValue(), req.GetId().GetValue())
			if err != nil {
				return nil, err
			}
			dag, err := s.store.GetDag(ctx, run.GetDag().GetValue())
			if err != nil {
				return nil, status.Errorf(codes.Internal, "load dag: %v", err)
			}
			if dag == nil {
				return nil, status.Error(codes.NotFound, "dag not found")
			}
			return dag, nil
		})
}

// ListAgents lists the tenant's registered agents (machine == agent); the client
// overlays them onto the run topology by machine_id.
func (s *RunService) ListAgents(ctx context.Context, req *uipb.ListAgentsRequest) (*uipb.ListAgentsResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListAgents",
		func(ctx context.Context, _ trace.Span) (*uipb.ListAgentsResponse, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			size := svcutil.PageSize(req.GetPage())
			q := models.Agents.SelectAll().Where(
				models.Agents.TenantId.Eq(req.GetTenantId().GetValue()),
				models.Agents.DeletedAt.IsNull(),
			)
			if tok := req.GetPage().GetToken(); tok != "" {
				q = q.Where(models.Agents.Id.Lt(tok))
			}
			rows, err := s.agents.Query(ctx, q.OrderByDESC(models.AgentColumnId).Limit(size+1))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "list agents: %v", err)
			}
			items, pageInfo := svcutil.Paginate(rows, size, func(a *models.Agent) string {
				return a.GetId().GetValue()
			})
			return &uipb.ListAgentsResponse{Agents: items, PageInfo: pageInfo}, nil
		})
}

// hydrateStatus loads the run's Dag and sets the run's live status from it. The
// status is authoritative on Dag.status (H22); TestRun.status is the read-time
// mirror (the persisted column is best-effort).
// hydrateStatus sets the run's live status from its Dag (Dag.status, H22) and
// returns the run's execution window (start/finish) for the list duration column,
// or nil when the run has no Dag/timing yet.
func (s *RunService) hydrateStatus(ctx context.Context, run *models.TestRun) *uipb.RunTiming {
	if run.GetDag().GetValue() == "" {
		return nil
	}
	dag, err := s.store.GetDag(ctx, run.GetDag().GetValue())
	if err != nil {
		s.Logger().Warn("hydrate run status: load dag", xlog.String("dag_id", run.GetDag().GetValue()), xlog.Error("error", err))
		return nil
	}
	if dag == nil {
		return nil
	}
	run.Status = dag.GetStatus() // live run status mirrors the Dag (H22)
	ex := dag.GetExecution()
	if ex.GetStartedAt() == nil && ex.GetFinishedAt() == nil {
		return nil
	}
	return &uipb.RunTiming{
		RunId:      &models.TestRunId{Value: run.GetEntity().GetId().GetValue()},
		StartedAt:  ex.GetStartedAt(),
		FinishedAt: ex.GetFinishedAt(),
	}
}

// countQuery returns the total row count for a filtered SELECT, ignoring its
// order/offset/limit, by wrapping it in count(*). Pass the query BEFORE adding
// pagination clauses.
func (s *RunService) countQuery(ctx context.Context, q interface{ Build() (string, []any) }) (uint64, error) {
	inner, args := q.Build()
	// Build() terminates the statement with ";"; strip it so the query nests in a
	// count(*) subquery without a syntax error.
	inner = strings.TrimRight(strings.TrimSpace(inner), ";")
	var total int64 // count(*) is bigint
	if err := s.db.QueryRow(ctx, "SELECT count(*) FROM ("+inner+") AS sub", args...).Scan(&total); err != nil {
		return 0, err
	}
	if total < 0 {
		total = 0
	}
	return uint64(total), nil
}

// ListTestRuns returns the tenant's runs with OFFSET pagination (newest-first by
// default) so the UI can render page numbers and jump to any page. Status is the
// run's Dag.status, so the status filter is a subquery over the dags table (whose
// status column is kept in sync); duration comes from each Dag's execution window.
func (s *RunService) ListTestRuns(ctx context.Context, req *uipb.ListTestRunsRequest) (*uipb.ListTestRunsResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListTestRuns",
		func(ctx context.Context, _ trace.Span) (*uipb.ListTestRunsResponse, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			size := svcutil.PageSize(req.GetPage())
			offset := svcutil.Offset(req.GetPage())
			q := models.TestRuns.SelectAll().Where(
				models.TestRuns.TenantId.Eq(req.GetTenantId().GetValue()),
				models.TestRuns.DeletedAt.IsNull(),
			)
			// status lives on the Dag (run.status is best-effort): filter via a
			// subquery over the dags table, whose status column is synced on save.
			if st := req.GetStatus(); st != primitive.Status_STATUS_UNSPECIFIED {
				dagIDs := models.Dags.Select(models.DagColumnId).Where(
					models.Dags.TenantId.Eq(req.GetTenantId().GetValue()),
					models.Dags.Status.In(st.String()),
					models.Dags.DeletedAt.IsNull(),
				)
				q = q.Where(models.TestRuns.Dag.InOf(dagIDs))
			}
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

			// Total (ignoring pagination) for the page-number UI — wrap the filtered
			// query in count(*) before adding order/offset/limit.
			total, err := s.countQuery(ctx, q)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "count test runs: %v", err)
			}

			// Sort by a native column (status isn't one — it's Dag-derived); id is the
			// stable tiebreaker so paging is deterministic.
			sortCol := models.TestRunColumnCreatedAt
			if req.GetSortField() == uipb.ListTestRunsRequest_SORT_FIELD_NAME {
				sortCol = models.TestRunColumnName
			}
			if req.GetOrder() == models.SortOrder_SORT_ORDER_ASC {
				q = q.OrderByASC(sortCol, models.TestRunColumnId)
			} else {
				q = q.OrderByDESC(sortCol, models.TestRunColumnId)
			}

			items, err := s.testRuns.Query(ctx, q.Offset(offset).Limit(size))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "list test runs: %v", err)
			}
			pageInfo := &models.PageInfo{
				Total:   &total,
				HasMore: uint64(offset+len(items)) < total,
			}
			// Hydrate each run's live status + execution timing from its Dag (H22).
			// Per-run GetDag (no bulk-by-ids primitive on DagStore yet).
			timings := make([]*uipb.RunTiming, 0, len(items))
			for _, run := range items {
				if t := s.hydrateStatus(ctx, run); t != nil {
					timings = append(timings, t)
				}
			}
			owners, err := s.loadOwners(ctx, items)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "load run owners: %v", err)
			}
			return &uipb.ListTestRunsResponse{TestRuns: items, PageInfo: pageInfo, Owners: owners, Timings: timings}, nil
		})
}

// loadOwners fetches the distinct owner accounts referenced by the page in a
// single WHERE id IN (...) query (no N+1), so the UI can show author name/email.
func (s *RunService) loadOwners(ctx context.Context, runs []*models.TestRun) ([]*models.Account, error) {
	ids := make([]string, 0, len(runs))
	seen := make(map[string]struct{}, len(runs))
	for _, run := range runs {
		id := run.GetOwned().GetOwnerAccountId().GetValue()
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return s.accounts.Query(ctx, models.Accounts.SelectAll().Where(
		models.Accounts.Id.In(ids...),
		models.Accounts.DeletedAt.IsNull(),
	))
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
			if s.canceller == nil {
				return nil, status.Error(codes.Unimplemented, "runtime canceller not wired")
			}
			// Hand the cancellation to the runtime: it forces the dag to CANCELLING and
			// runs always_run teardown itself. We never write the dag status here — that
			// would race the executor's snapshots and leave deployments un-torn-down.
			s.canceller.Cancel(run.GetDag().GetValue())
			return run, nil
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
