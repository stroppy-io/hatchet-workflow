// Package suite implements the ui SuiteService: Suite definition + SuiteRun
// orchestration. A SuiteRun is itself a Dag whose nodes dag_ref the per-test Dags
// (H13: SuitePreset.Scheduling -> Dag.Scheduling). RBAC: create/launch/cancel =
// ADMIN, reads = VIEWER.
package suite

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
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"github.com/stroppy-io/stroppy-cloud/internal/runtime"
	"github.com/stroppy-io/stroppy-cloud/internal/services/authz"
	"github.com/stroppy-io/stroppy-cloud/internal/services/svcutil"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

const metadataTenantID = "tenant_id"

// ProviderResolver resolves a tenant's run-scoped deployment.Deployment for a
// TestPreset (baked into each per-test dag via dag.BuildTestDag).
type ProviderResolver interface {
	Resolve(ctx context.Context, tenantID string, preset *domain.TestPreset) (*deployment.Deployment, error)
}

// DagStore persists Dag aggregates.
type DagStore interface {
	SaveDag(ctx context.Context, dag *primitive.Dag) error
	GetDag(ctx context.Context, id string) (*primitive.Dag, error)
}

// SuiteService implements ui.SuiteActions.
type SuiteService struct {
	*tracing.Entity
	suites *repository.ProtoRepository[
		models.SuiteAlias, models.SuiteColumnAlias, *models.SuiteScanner, *models.Suite,
	]
	suiteRuns *repository.ProtoRepository[
		models.SuiteRunAlias, models.SuiteRunColumnAlias, *models.SuiteRunScanner, *models.SuiteRun,
	]
	testRuns *repository.ProtoRepository[
		models.TestRunAlias, models.TestRunColumnAlias, *models.TestRunScanner, *models.TestRun,
	]
	provider ProviderResolver
	store    DagStore
	authz    *authz.Authz
	txm      tx.Trm
}

var _ uiapi.SuiteActions = (*SuiteService)(nil)

// New builds a SuiteService.
func New(logger *xlog.Logger, executor exec.DB, txm tx.Trm, az *authz.Authz, provider ProviderResolver, store DagStore) *SuiteService {
	return &SuiteService{
		Entity:    tracing.NewEntity(logger.AppendName("SuiteService")),
		suites:    repository.NewProtoRepository(repository.NewScannerRepository(models.Suites.Table, executor), models.SuiteConverter),
		suiteRuns: repository.NewProtoRepository(repository.NewScannerRepository(models.SuiteRuns.Table, executor), models.SuiteRunConverter),
		testRuns:  repository.NewProtoRepository(repository.NewScannerRepository(models.TestRuns.Table, executor), models.TestRunConverter),
		provider:  provider,
		store:     store,
		authz:     az,
		txm:       txm,
	}
}

// CreateSuite stores a new suite definition.
func (s *SuiteService) CreateSuite(ctx context.Context, req *uipb.CreateSuiteRequest) (*models.Suite, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CreateSuite",
		func(ctx context.Context, _ trace.Span) (*models.Suite, error) {
			c := svcutil.CallerOf(ctx)
			if err := s.authz.Require(ctx, c, req.GetTenantId(), models.TenantMember_ROLE_ADMIN); err != nil {
				return nil, err
			}
			suite := &models.Suite{
				Entity:      ids.NewEntity(),
				Owned:       &models.Own{OwnerAccountId: c.AccountID, TenantId: req.GetTenantId()},
				Name:        req.Name,
				Description: req.Description,
				Preset:      req.GetPreset(),
			}
			return tx.DoReadCommittedRet(ctx, s.txm, func(ctx context.Context) (*models.Suite, error) {
				if _, err := s.suites.Execute(ctx, models.Suites.Insert().From(suite.IntoPlain().AllSetters()...)); err != nil {
					return nil, status.Errorf(codes.Internal, "insert suite: %v", err)
				}
				return suite, nil
			})
		})
}

// GetSuite returns a suite by id.
func (s *SuiteService) GetSuite(ctx context.Context, req *uipb.GetSuiteRequest) (*models.Suite, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "GetSuite",
		func(ctx context.Context, _ trace.Span) (*models.Suite, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			suite, err := s.suites.QueryRow(ctx, models.Suites.SelectAll().Where(
				models.Suites.Id.Eq(req.GetSuiteId().GetValue()),
				models.Suites.TenantId.Eq(req.GetTenantId().GetValue()),
				models.Suites.DeletedAt.IsNull(),
			))
			if err != nil {
				return nil, svcutil.NotFound(err, "suite")
			}
			return suite, nil
		})
}

// LaunchSuiteRun compiles every test in the suite into its own Dag and builds an
// orchestration Dag of dag_ref nodes (dag.BuildSuiteDag maps SuitePreset.Scheduling:
// sequential -> chained edges, parallel -> MaxParallelism, + on_node_failure);
// persists the lot and returns the SuiteRun.
func (s *SuiteService) LaunchSuiteRun(ctx context.Context, req *uipb.LaunchSuiteRunRequest) (*models.SuiteRun, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "LaunchSuiteRun",
		func(ctx context.Context, _ trace.Span) (*models.SuiteRun, error) {
			c := svcutil.CallerOf(ctx)
			if err := s.authz.Require(ctx, c, req.GetTenantId(), models.TenantMember_ROLE_ADMIN); err != nil {
				return nil, err
			}
			suite, err := s.suites.QueryRow(ctx, models.Suites.SelectAll().Where(
				models.Suites.Id.Eq(req.GetSuiteId().GetValue()),
				models.Suites.TenantId.Eq(req.GetTenantId().GetValue()),
				models.Suites.DeletedAt.IsNull(),
			))
			if err != nil {
				return nil, svcutil.NotFound(err, "suite")
			}

			tenant := req.GetTenantId()
			suiteRunEntity := ids.NewEntity()
			suiteRunID := &models.SuiteRunId{Value: suiteRunEntity.GetId().GetValue()}

			var testDags []*primitive.Dag
			var testRuns []*models.TestRun
			var testDagIDs []string

			for i, tp := range suite.GetPreset().GetTests() {
				dep, err := s.provider.Resolve(ctx, tenant.GetValue(), tp)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "resolve deployment %d: %v", i, err)
				}
				dag := dagdomain.BuildTestDag(tp, dep, dagdomain.Deps{Install: dagdomain.RecipeInstallBuilder{}})
				if dag.Metadata == nil {
					dag.Metadata = map[string]string{}
				}
				dag.Metadata[metadataTenantID] = tenant.GetValue()
				if err := runtime.ValidateDag(dag); err != nil {
					return nil, status.Errorf(codes.Internal, "invalid dag for test %d: %v", i, err)
				}
				testDags = append(testDags, dag)
				testRuns = append(testRuns, &models.TestRun{
					Entity:     ids.NewEntity(),
					Owned:      &models.Own{OwnerAccountId: c.AccountID, TenantId: tenant},
					TestPreset: tp,
					Dag:        &models.DagId{Value: dag.GetId()},
					SuiteRunId: suiteRunID,
				})
				testDagIDs = append(testDagIDs, dag.GetId())
			}

			// SuiteRun is itself a Dag of dag_ref nodes over the per-test Dags
			// (built in internal/domain/dag).
			orch := dagdomain.BuildSuiteDag(testDagIDs, suite.GetPreset().GetScheduling())
			orch.Metadata = map[string]string{metadataTenantID: tenant.GetValue()}
			orchID := orch.GetId()
			if err := runtime.ValidateDag(orch); err != nil {
				return nil, status.Errorf(codes.Internal, "invalid orchestration dag: %v", err)
			}

			suiteRun := &models.SuiteRun{
				Entity:  suiteRunEntity,
				Owned:   &models.Own{OwnerAccountId: c.AccountID, TenantId: tenant},
				SuiteId: req.GetSuiteId(),
				Dag:     &models.DagId{Value: orchID},
			}

			return tx.DoReadCommittedRet(ctx, s.txm, func(ctx context.Context) (*models.SuiteRun, error) {
				for _, d := range testDags {
					if err := s.store.SaveDag(ctx, d); err != nil {
						return nil, status.Errorf(codes.Internal, "persist test dag: %v", err)
					}
				}
				if err := s.store.SaveDag(ctx, orch); err != nil {
					return nil, status.Errorf(codes.Internal, "persist orchestration dag: %v", err)
				}
				if _, err := s.suiteRuns.Execute(ctx, models.SuiteRuns.Insert().From(suiteRun.IntoPlain().AllSetters()...)); err != nil {
					return nil, status.Errorf(codes.Internal, "insert suite run: %v", err)
				}
				for _, tr := range testRuns {
					if _, err := s.testRuns.Execute(ctx, models.TestRuns.Insert().From(tr.IntoPlain().AllSetters()...)); err != nil {
						return nil, status.Errorf(codes.Internal, "insert test run: %v", err)
					}
				}
				return suiteRun, nil
			})
		})
}

// GetSuiteRun returns a suite run by id.
func (s *SuiteService) GetSuiteRun(ctx context.Context, req *uipb.GetSuiteRunRequest) (*models.SuiteRun, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "GetSuiteRun",
		func(ctx context.Context, _ trace.Span) (*models.SuiteRun, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			return s.loadSuiteRun(ctx, req.GetTenantId().GetValue(), req.GetSuiteRunId().GetValue())
		})
}

// ListSuiteRuns returns the tenant's suite runs with cursor pagination
// (newest-first by default), optionally scoped to a single suite.
func (s *SuiteService) ListSuiteRuns(ctx context.Context, req *uipb.ListSuiteRunsRequest) (*uipb.ListSuiteRunsResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListSuiteRuns",
		func(ctx context.Context, _ trace.Span) (*uipb.ListSuiteRunsResponse, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			size := svcutil.PageSize(req.GetPage())
			desc := svcutil.CursorDesc(req.GetOrder())
			q := models.SuiteRuns.SelectAll().Where(
				models.SuiteRuns.TenantId.Eq(req.GetTenantId().GetValue()),
				models.SuiteRuns.DeletedAt.IsNull(),
			)
			if req.GetSuiteId() != nil {
				q = q.Where(models.SuiteRuns.SuiteId.Eq(req.GetSuiteId().GetValue()))
			}
			// status filtering needs the Dag status (not a run column); use the Dag list or denormalize status — tracked as a schema follow-up.
			// tags: Tags is serialized JSON of common.Tags (a TEXT column) — match
			// each requested free tag and key=value label as a substring.
			for _, tag := range req.GetTags().GetTags() {
				q = q.Where(models.SuiteRuns.Tags.ILike("%" + tag + "%"))
			}
			for k, v := range req.GetTags().GetLabels() {
				q = q.Where(models.SuiteRuns.Tags.ILike("%" + k + "%" + v + "%"))
			}
			if tok := req.GetPage().GetToken(); tok != "" {
				if desc {
					q = q.Where(models.SuiteRuns.Id.Lt(tok))
				} else {
					q = q.Where(models.SuiteRuns.Id.Gt(tok))
				}
			}
			if desc {
				q = q.OrderByDESC(models.SuiteRunColumnId)
			} else {
				q = q.OrderByASC(models.SuiteRunColumnId)
			}
			rows, err := s.suiteRuns.Query(ctx, q.Limit(size+1))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "list suite runs: %v", err)
			}
			items, pageInfo := svcutil.Paginate(rows, size, func(r *models.SuiteRun) string {
				return r.GetEntity().GetId().GetValue()
			})
			return &uipb.ListSuiteRunsResponse{SuiteRuns: items, PageInfo: pageInfo}, nil
		})
}

// ListSuites returns the tenant's suite definitions with cursor pagination
// (newest-first by default), optionally filtered by cron presence.
func (s *SuiteService) ListSuites(ctx context.Context, req *uipb.ListSuitesRequest) (*uipb.ListSuitesResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListSuites",
		func(ctx context.Context, _ trace.Span) (*uipb.ListSuitesResponse, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			size := svcutil.PageSize(req.GetPage())
			desc := svcutil.CursorDesc(req.GetOrder())
			q := models.Suites.SelectAll().Where(
				models.Suites.TenantId.Eq(req.GetTenantId().GetValue()),
				models.Suites.DeletedAt.IsNull(),
			)
			if req.HasCron != nil {
				// Cron is a NOT NULL text column: empty string == no schedule.
				if req.GetHasCron() {
					q = q.Where(models.Suites.Cron.Neq(""))
				} else {
					q = q.Where(models.Suites.Cron.Eq(""))
				}
			}
			// search: match the suite name (NullText column → raw ILIKE).
			if req.GetSearch() != "" {
				q = q.Where(models.Suites.Name.Raw("ILIKE", "?", "%"+req.GetSearch()+"%"))
			}
			// tags: Tags is serialized JSON of common.Tags (a TEXT column) — match
			// each requested free tag and key=value label as a substring.
			for _, tag := range req.GetTags().GetTags() {
				q = q.Where(models.Suites.Tags.ILike("%" + tag + "%"))
			}
			for k, v := range req.GetTags().GetLabels() {
				q = q.Where(models.Suites.Tags.ILike("%" + k + "%" + v + "%"))
			}
			if tok := req.GetPage().GetToken(); tok != "" {
				if desc {
					q = q.Where(models.Suites.Id.Lt(tok))
				} else {
					q = q.Where(models.Suites.Id.Gt(tok))
				}
			}
			if desc {
				q = q.OrderByDESC(models.SuiteColumnId)
			} else {
				q = q.OrderByASC(models.SuiteColumnId)
			}
			rows, err := s.suites.Query(ctx, q.Limit(size+1))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "list suites: %v", err)
			}
			items, pageInfo := svcutil.Paginate(rows, size, func(su *models.Suite) string {
				return su.GetEntity().GetId().GetValue()
			})
			return &uipb.ListSuitesResponse{Suites: items, PageInfo: pageInfo}, nil
		})
}

// CancelSuiteRun moves the suite run's orchestration Dag to CANCELLING.
func (s *SuiteService) CancelSuiteRun(ctx context.Context, req *uipb.CancelSuiteRunRequest) (*models.SuiteRun, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CancelSuiteRun",
		func(ctx context.Context, _ trace.Span) (*models.SuiteRun, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_ADMIN); err != nil {
				return nil, err
			}
			sr, err := s.loadSuiteRun(ctx, req.GetTenantId().GetValue(), req.GetSuiteRunId().GetValue())
			if err != nil {
				return nil, err
			}
			return tx.DoReadCommittedRet(ctx, s.txm, func(ctx context.Context) (*models.SuiteRun, error) {
				dag, err := s.store.GetDag(ctx, sr.GetDag().GetValue())
				if err != nil {
					return nil, status.Errorf(codes.Internal, "load orchestration dag: %v", err)
				}
				if dag != nil && !isTerminal(dag.GetStatus()) {
					dag.Status = primitive.Status_STATUS_CANCELLING
					if err := s.store.SaveDag(ctx, dag); err != nil {
						return nil, status.Errorf(codes.Internal, "cancel orchestration dag: %v", err)
					}
				}
				return sr, nil
			})
		})
}

func (s *SuiteService) loadSuiteRun(ctx context.Context, tenantID, suiteRunID string) (*models.SuiteRun, error) {
	sr, err := s.suiteRuns.QueryRow(ctx, models.SuiteRuns.SelectAll().Where(
		models.SuiteRuns.Id.Eq(suiteRunID),
		models.SuiteRuns.TenantId.Eq(tenantID),
		models.SuiteRuns.DeletedAt.IsNull(),
	))
	if err != nil {
		return nil, svcutil.NotFound(err, "suite run")
	}
	return sr, nil
}

func isTerminal(st primitive.Status) bool {
	switch st {
	case primitive.Status_STATUS_COMPLETED, primitive.Status_STATUS_FAILED, primitive.Status_STATUS_CANCELLED:
		return true
	default:
		return false
	}
}
