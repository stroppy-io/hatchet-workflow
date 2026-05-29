package preset

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/gopherex/pgtx/pkg/tx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

/*
	package preset implements the three tenant-scoped preset CRUD services
	(DatabasePresetService / WorkloadPresetService / TestPresetService) over the
	models.DatabasePresetRecord / WorkloadPresetRecord / TestPresetRecord tables.
	Each kind has its own service because each lives in its own table with its own
	filters and sort keys, but they share identical wiring, validation and
	ownership semantics, so the shared seams (Authn, Tx, ctx helpers) live
	here and the per-kind handlers live in their own concept files.

	===== Dependency interfaces (constructor-injected) =====

	The services own no storage of their own: every side effect goes through one
	of these. Implementations live elsewhere. Persistence methods return
	derrors.ErrNotFound / derrors.ErrConflict so the handler can translate them
	into status codes via utils.MapErr. The kind-specific repos live in the
	matching concept file (database.go / workload.go / test.go).
*/

// Deps bundles every dependency for the constructors. The three services share
// one Deps because they share the same auth/tx seams and only differ in
// which repo they touch.
type Deps struct {
	Authn     utils.Authn
	Databases DatabasePresetRepo
	Workloads WorkloadPresetRepo
	Tests     TestPresetRepo
	Tx        tx.Trm
}

/*
	===== DatabasePresetService =====
*/

type DatabasePresetService struct {
	*api.UnimplementedDatabasePresetServiceServer
	tx.Trm
	d Deps
}

var _ api.DatabasePresetServiceServer = (*DatabasePresetService)(nil)

func NewDatabasePresetService(deps Deps) *DatabasePresetService {
	return &DatabasePresetService{d: deps}
}

/*
	===== WorkloadPresetService =====
*/

type WorkloadPresetService struct {
	*api.UnimplementedWorkloadPresetServiceServer
	tx.Trm
	d Deps
}

var _ api.WorkloadPresetServiceServer = (*WorkloadPresetService)(nil)

func NewWorkloadPresetService(deps Deps) *WorkloadPresetService {
	return &WorkloadPresetService{d: deps}
}

/*
	===== TestPresetService =====
*/

type TestPresetService struct {
	*api.UnimplementedTestPresetServiceServer
	tx.Trm
	d Deps
}

var _ api.TestPresetServiceServer = (*TestPresetService)(nil)

func NewTestPresetService(deps Deps) *TestPresetService {
	return &TestPresetService{d: deps}
}

/*
	===== shared helpers =====

	The three services share identical auth/clock/tx mechanics, so the helpers
	are defined once on Deps (via a thin wrapper) rather than per-service. Each
	service exposes them as methods that forward to its own Deps.
*/

func (d Deps) caller(ctx context.Context) (*iam.AccessClaims, error) {
	c, err := d.Authn.Caller(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return c, nil
}

func (d Deps) now() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// doTx runs fn in a top-level serializable transaction, retrying on transient
// serialization failures/deadlocks (DefaultRetryPolicy). fn is re-run from
// scratch on retry, so it MUST be safe to repeat (DB-only side effects).
func (d Deps) doTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tx.DoSerializable(ctx, d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// doTxRet is doTx for a transaction that returns a value. Same retry semantics:
// fn must be safe to re-run.
func doTxRet[T any](ctx context.Context, d Deps, fn func(ctx context.Context) (T, error)) (T, error) {
	return tx.DoSerializableRet(ctx, d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

/*
	===== shared entity stamping =====

	Create / Clone / Update all converge on the same server-owned fields: the
	server assigns id / tenant_id / author_id / timings and ignores any
	client-supplied value for them (the proto Create doc says so). is_favorite is
	a computed per-caller flag and is never persisted from the request.
*/

// stampNew fills the server-owned fields of a freshly created entity envelope,
// allocating a missing envelope, forcing the request tenant and the caller as
// author, and setting the creation timings. Returns the populated envelope.
func (d Deps) stampNew(e *common.Entity, tenantID, authorID string) *common.Entity {
	if e == nil {
		e = &common.Entity{}
	}
	e.Id = uuid.NewString()
	e.TenantId = tenantID
	e.AuthorId = authorID
	e.IsFavorite = false
	now := d.now()
	e.Timings = &common.Timings{CreatedAt: now, UpdatedAt: now}
	return e
}

// preserveEntity builds the envelope for a wholesale Update: the server-owned
// id / tenant_id / author_id / created_at are carried over from the stored row
// (immutable), the editable name / description are taken from the request, and
// updated_at is bumped to now. is_favorite is per-caller and never persisted.
func preserveEntity(existing, incoming *common.Entity, d Deps) *common.Entity {
	createdAt := d.now()
	if existing.GetTimings().GetCreatedAt() != nil {
		createdAt = existing.GetTimings().GetCreatedAt()
	}
	return &common.Entity{
		Id:          existing.GetId(),
		TenantId:    existing.GetTenantId(),
		Name:        incoming.GetName(),
		Description: incoming.GetDescription(),
		AuthorId:    existing.GetAuthorId(),
		IsFavorite:  false,
		Timings:     &common.Timings{CreatedAt: createdAt, UpdatedAt: d.now()},
	}
}

// ignoreNotFound returns nil for a not-found error (so idempotent deletes are
// no-ops) and maps any other error to a gRPC status. It also passes through an
// already-built gRPC status error unchanged.
func ignoreNotFound(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, derrors.ErrNotFound) {
		return nil
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	return utils.MapErr(err)
}

// cloneName derives the name for a cloned copy: the explicit override when given,
// else "<source name> (copy)".
func cloneName(override, source string) string {
	if override != "" {
		return override
	}
	if source == "" {
		return "(copy)"
	}
	return source + " (copy)"
}
