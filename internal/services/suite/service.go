package suite

import (
	"context"
	"time"

	"github.com/gopherex/pgtx/pkg/tx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

/*
	===== Dependency interfaces (constructor-injected) =====

	suite is the tenant-scoped API over models.SuiteRecord — the reusable suite
	DEFINITION. It owns no storage or orchestration of its own: every side effect
	goes through one of these ports. Implementations live elsewhere and are wired
	in via SuiteDeps. Persistence methods return derrors.ErrNotFound /
	derrors.ErrConflict so handlers can translate them into status codes via
	utils.MapErr.

	AUTHZ NOTE: RBAC (RESOURCE_SUITE / RESOURCE_SUITE_RUN) is enforced by the auth
	interceptor reading the proto annotations — handlers do NOT re-check the
	annotated permission. Handlers DO: resolve caller identity (author/clone
	ownership), validate the active tenant_id, verify the target suite exists AND
	belongs to that tenant, honor the documented idempotency, run multi-write flows
	inside doTx, and return correct gRPC codes via utils.MapErr.
*/

// SuiteRepo persists models.SuiteRecord rows (the reusable definition). Every
// method is tenant-scoped: a row is addressed by (tenantID, id) so a caller can
// never read or mutate another tenant's suite. Get returns derrors.ErrNotFound
// when the (tenantID,id) pair has no row; Create returns derrors.ErrConflict on
// a duplicate id.
type SuiteRepo interface {
	Create(ctx context.Context, suite *models.SuiteRecord) error
	Get(ctx context.Context, tenantID, id string) (*models.SuiteRecord, error)
	List(ctx context.Context, query SuiteListQuery) (suites []*models.SuiteRecord, nextPageToken string, err error)
	// Update wholesale-replaces an existing row (addressed by
	// suite.entity.tenant_id + suite.entity.id). Returns derrors.ErrNotFound if
	// the row is gone.
	Update(ctx context.Context, suite *models.SuiteRecord) error
	Delete(ctx context.Context, tenantID, id string) error
}

// SuiteListQuery is the resolved, tenant-scoped query the handler hands to the
// repo for ListSuites — proto filters/facets/sort/page flattened into a plain
// struct so the repo never depends on the request type.
type SuiteListQuery struct {
	TenantID        string
	Filter          *common.EntityFilter
	Providers       []deployment.Provider
	ScheduleEnabled *bool
	Sort            *api.ListSuitesRequest_Sort
	PageSize        uint32
	PageToken       string
	// CallerID scopes the computed, per-caller entity.is_favorite flag and the
	// filter.favorites_only join (favorites are per-account, see common.Entity).
	CallerID string
}

// SuiteRunLauncher expands a fully-baked suite definition into a
// models.SuiteRunRecord (one child models.TestRunRecord per compatible cell),
// persists them and launches SuiteWorkflow. It runs inside the ambient ctx
// transaction so the run rows commit atomically with the surrounding writes; the
// workflow handle itself is started transactionally (an outbox row a relay picks
// up), never via synchronous IO while the transaction is open. Validate reports
// a derrors error (Invalid / FailedPrecondition) when the suite cannot expand
// into at least one runnable cell so the handler can surface a precise code
// before any write.
type SuiteRunLauncher interface {
	// Validate checks the suite expands into >=1 runnable cell and that every
	// referenced preset exists and is compatible with the chosen provider. It
	// performs no writes.
	Validate(ctx context.Context, tenantID string, spec *domain.Suite) error
	// Launch expands + persists the SuiteRunRecord and its children and starts
	// SuiteWorkflow. The handler pre-fills the run's identity/tenant/trigger/
	// timing/rating; the launcher fills the expanded children + summary.
	Launch(ctx context.Context, run *models.SuiteRunRecord, spec *domain.Suite) error
}

// SuiteDeps bundles every dependency for the constructor.
type SuiteDeps struct {
	Authn    utils.Authn
	Suites   SuiteRepo
	Launcher SuiteRunLauncher
	Tx       tx.Trm
}

type SuiteService struct {
	*api.UnimplementedSuiteServiceServer
	tx.Trm
	d SuiteDeps
}

var _ api.SuiteServiceServer = (*SuiteService)(nil)

func NewSuiteService(deps SuiteDeps) *SuiteService {
	return &SuiteService{UnimplementedSuiteServiceServer: &api.UnimplementedSuiteServiceServer{}, d: deps}
}

/*
	===== helpers =====
*/

func (s *SuiteService) caller(ctx context.Context) (*iam.AccessClaims, error) {
	c, err := s.d.Authn.Caller(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return c, nil
}

func (s *SuiteService) now() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// doTx runs fn in a top-level serializable transaction, retrying on transient
// serialization failures/deadlocks (DefaultRetryPolicy). fn is re-run from
// scratch on retry, so it MUST be safe to repeat: DB-only side effects (outbox
// rows included) and never synchronous external IO.
func (s *SuiteService) doTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tx.DoSerializable(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// doTxRet is doTx for a transaction that returns a value. Same retry semantics:
// fn must be safe to re-run.
func doTxRet[T any](ctx context.Context, s *SuiteService, fn func(ctx context.Context) (T, error)) (T, error) {
	return tx.DoSerializableRet(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}
