package test_run

import (
	"context"
	"time"

	"github.com/gopherex/pgtx/pkg/tx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	domain "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

/*
	===== Dependency interfaces (constructor-injected) =====

	test_run owns no storage, no clock and no workflow engine of its own: every
	side effect goes through one of these ports. Implementations live elsewhere
	and are wired in via TestRunDeps. Persistence methods return
	derrors.ErrNotFound / derrors.ErrConflict so the handler can translate them
	into status codes via utils.MapErr.

	AUTHZ NOTE: the annotated RESOURCE_TEST_RUN / RESOURCE_PRESET permission is
	enforced by the auth interceptor — handlers do NOT re-check it. They DO
	resolve the caller (for author stamping), validate the active tenant_id, and
	verify that every target row exists AND belongs to the request tenant before
	touching it (no cross-tenant reads/writes).
*/

// TestRunRepo persists models.TestRunRecord rows. Every read/write is
// tenant-scoped: the repo MUST filter on tenant_id so an id from another tenant
// is reported as derrors.ErrNotFound, never returned. List applies the request
// filters/facets/sort/paging and returns the opaque next-page token.
type TestRunRepo interface {
	Create(ctx context.Context, run *models.TestRunRecord) error
	// Get fetches one run scoped to tenantID; a row of another tenant -> ErrNotFound.
	Get(ctx context.Context, tenantID, id string) (*models.TestRunRecord, error)
	List(ctx context.Context, query *api.ListTestRunsRequest) (runs []*models.TestRunRecord, nextPageToken string, err error)
	// Update persists a mutated record (status/summary changes).
	Update(ctx context.Context, run *models.TestRunRecord) error
	// Delete removes the run scoped to tenantID; absent -> ErrNotFound.
	Delete(ctx context.Context, tenantID, id string) error
}

// PresetRepo persists models.TestPresetRecord rows for ExtractToPreset.
type PresetRepo interface {
	CreateTestPreset(ctx context.Context, preset *models.TestPresetRecord) error
}

// Summarizer derives the denormalized, queryable Summary projection from a baked
// domain.TestRun spec (db kind/preset, workload, topology, provider). It is the
// single place that knows how to read the baked spec's oneofs, keeping the
// handler free of spec-shape knowledge. Pure: no IO.
type Summarizer interface {
	Summarize(spec *domain.TestRun) *models.TestRunRecord_Summary
}

// Workflows launches and cancels the per-run TestWorkflow. Launch is started
// AFTER the record is committed (it is external IO and must not run inside the
// ambient transaction). Cancel signals an already-running workflow.
type Workflows interface {
	// LaunchTest starts TestWorkflow for the persisted run.
	LaunchTest(ctx context.Context, run *models.TestRunRecord) error
	// CancelTest signals cancellation of a running TestWorkflow. Cancelling a
	// workflow that is not running returns derrors.ErrNotFound (treated as a no-op).
	CancelTest(ctx context.Context, runID string) error
}

// TestRunDeps bundles every dependency for the constructor.
type TestRunDeps struct {
	Authn      utils.Authn
	Runs       TestRunRepo
	Presets    PresetRepo
	Summarizer Summarizer
	Workflows  Workflows
	Tx         tx.Trm
}

type TestRunService struct {
	*api.UnimplementedTestRunServiceServer
	tx.Trm
	d TestRunDeps
}

var _ api.TestRunServiceServer = (*TestRunService)(nil)

func NewTestRunService(deps TestRunDeps) *TestRunService {
	return &TestRunService{UnimplementedTestRunServiceServer: &api.UnimplementedTestRunServiceServer{}, d: deps}
}

/*
	===== helpers =====
*/

func (s *TestRunService) caller(ctx context.Context) (*iam.AccessClaims, error) {
	c, err := s.d.Authn.Caller(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return c, nil
}

func (s *TestRunService) now() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// doTx runs fn in a top-level serializable transaction, retrying on transient
// serialization failures/deadlocks. fn MUST be safe to re-run: DB-only side
// effects, never synchronous external IO.
func (s *TestRunService) doTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tx.DoSerializable(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// doTxRet is doTx for a transaction that returns a value. Same retry semantics.
func doTxRet[T any](ctx context.Context, s *TestRunService, fn func(ctx context.Context) (T, error)) (T, error) {
	return tx.DoSerializableRet(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// isTerminal reports whether a run lifecycle status is final (no further work).
func isTerminal(st commonpb.Status) bool {
	switch st {
	case commonpb.Status_STATUS_COMPLETED,
		commonpb.Status_STATUS_FAILED,
		commonpb.Status_STATUS_SKIPPED,
		commonpb.Status_STATUS_CANCELLED:
		return true
	default:
		return false
	}
}
