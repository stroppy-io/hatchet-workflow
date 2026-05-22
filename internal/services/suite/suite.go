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
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"github.com/stroppy-io/stroppy-cloud/internal/runtime"
	"github.com/stroppy-io/stroppy-cloud/internal/services/authz"
	"github.com/stroppy-io/stroppy-cloud/internal/services/svcutil"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

const metadataTenantID = "tenant_id"

// Planner compiles a TestPreset (+ deployment params) into a Dag (C12).
type Planner interface {
	Compile(preset *domain.TestPreset, params *dagdomain.DeploymentParams) (*primitive.Dag, error)
}

// ProviderResolver resolves a tenant's deployment params for a topology.
type ProviderResolver interface {
	Resolve(ctx context.Context, tenantID string, topo *domain.Topology) (*dagdomain.DeploymentParams, error)
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
	planner  Planner
	provider ProviderResolver
	store    DagStore
	authz    *authz.Authz
	txm      tx.Trm
}

var _ uiapi.SuiteActions = (*SuiteService)(nil)

// New builds a SuiteService.
func New(logger *xlog.Logger, executor exec.DB, txm tx.Trm, az *authz.Authz, planner Planner, provider ProviderResolver, store DagStore) *SuiteService {
	return &SuiteService{
		Entity:    tracing.NewEntity(logger.AppendName("SuiteService")),
		suites:    repository.NewProtoRepository(repository.NewScannerRepository(models.Suites.Table, executor), models.SuiteConverter),
		suiteRuns: repository.NewProtoRepository(repository.NewScannerRepository(models.SuiteRuns.Table, executor), models.SuiteRunConverter),
		testRuns:  repository.NewProtoRepository(repository.NewScannerRepository(models.TestRuns.Table, executor), models.TestRunConverter),
		planner:   planner,
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
// orchestration Dag of dag_ref nodes; persists the lot and returns the SuiteRun.
//
// TODO(suite): scheduling is coarse — all dag_refs are ready with MaxParallelism=1
// (effectively sequential); explicit seq/parallel ordering + on_node_failure
// mapping from SuitePreset.Scheduling is not fully translated. Reported.
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
				params, err := s.provider.Resolve(ctx, tenant.GetValue(), tp.GetTopology())
				if err != nil {
					return nil, status.Errorf(codes.Internal, "resolve deployment %d: %v", i, err)
				}
				dag, err := s.planner.Compile(tp, params)
				if err != nil {
					return nil, status.Errorf(codes.InvalidArgument, "compile test %d: %v", i, err)
				}
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

// ListSuiteRuns returns the tenant's suite runs.
//
// TODO(suite): page_token pagination ignored. Reported.
func (s *SuiteService) ListSuiteRuns(ctx context.Context, req *uipb.ListSuiteRunsRequest) (*models.SuiteRun_List, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListSuiteRuns",
		func(ctx context.Context, _ trace.Span) (*models.SuiteRun_List, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			runs, err := s.suiteRuns.Query(ctx, models.SuiteRuns.SelectAll().Where(
				models.SuiteRuns.TenantId.Eq(req.GetTenantId().GetValue()),
				models.SuiteRuns.DeletedAt.IsNull(),
			))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "list suite runs: %v", err)
			}
			return &models.SuiteRun_List{SuiteRuns: runs}, nil
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
