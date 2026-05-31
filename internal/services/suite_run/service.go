package suite_run

import (
	"context"
	"errors"
	"time"

	"github.com/gopherex/pgtx/pkg/tx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

/*
	===== Dependency interfaces (constructor-injected) =====

	The service owns no storage or workflow runtime of its own: every side effect
	goes through one of these ports. Implementations live elsewhere and are wired
	in via SuiteRunDeps. Persistence methods return derrors.ErrNotFound /
	derrors.ErrConflict so the handler can translate them into status codes via
	utils.MapErr.
*/

// SuiteRunRepo persists models.SuiteRunRecord rows (the suite EXECUTION started
// via SuiteAPI.StartSuite). Rows are tenant-scoped via Entity.tenant_id; the
// handler is responsible for verifying ownership. List performs the rich
// facet/sort/paginate filtering described by the request and returns the page
// plus the opaque next-page token.
type SuiteRunRepo interface {
	Get(ctx context.Context, id string) (*models.SuiteRunRecord, error)
	List(ctx context.Context, req *api.ListSuiteRunsRequest) (records []*models.SuiteRunRecord, nextPageToken string, err error)
	Update(ctx context.Context, record *models.SuiteRunRecord) error
	Delete(ctx context.Context, id string) error
}

// SuiteRunCanceller requests cancellation of the running suite workflow and its
// child test workflows. It MUST be idempotent for the runtime layer: signalling
// an already-finished workflow is a no-op. It participates in the ambient ctx
// transaction so the status flip and the cancellation signal (an outbox row)
// commit atomically with the surrounding writes — it MUST NOT block on
// synchronous external IO while the transaction is open.
type SuiteRunCanceller interface {
	CancelSuiteRun(ctx context.Context, suiteRunID string) error
}

// SuiteRunDeps bundles every dependency for the constructor.
type SuiteRunDeps struct {
	Authn     utils.Authn
	SuiteRuns SuiteRunRepo
	Canceller SuiteRunCanceller
	Tx        tx.Trm
}

type SuiteRunService struct {
	*api.UnimplementedSuiteRunServiceServer
	tx.Trm
	d SuiteRunDeps
}

var _ api.SuiteRunServiceServer = (*SuiteRunService)(nil)

func NewSuiteRunService(deps SuiteRunDeps) *SuiteRunService {
	return &SuiteRunService{UnimplementedSuiteRunServiceServer: &api.UnimplementedSuiteRunServiceServer{}, d: deps}
}

/*
	===== helpers =====
*/

func (s *SuiteRunService) caller(ctx context.Context) (*iam.AccessClaims, error) {
	c, err := s.d.Authn.Caller(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return c, nil
}

func (s *SuiteRunService) now() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// doTx runs fn in a top-level serializable transaction, retrying on transient
// serialization failures/deadlocks (DefaultRetryPolicy). fn is re-run from
// scratch on retry, so it MUST be safe to repeat: DB-only side effects (outbox
// rows included) and never synchronous external IO.
func (s *SuiteRunService) doTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tx.DoSerializable(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// doTxRet is doTx for a transaction that returns a value. Same retry semantics:
// fn must be safe to re-run.
func doTxRet[T any](ctx context.Context, s *SuiteRunService, fn func(ctx context.Context) (T, error)) (T, error) {
	return tx.DoSerializableRet(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// getOwned loads a suite run by id and asserts it belongs to tenantID. A row in
// another tenant is reported as NotFound so a caller cannot probe cross-tenant
// ids.
func (s *SuiteRunService) getOwned(ctx context.Context, tenantID, id string) (*models.SuiteRunRecord, error) {
	rec, err := s.d.SuiteRuns.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if rec.GetEntity().GetTenantId() != tenantID {
		return nil, derrors.ErrNotFound
	}
	return rec, nil
}

// touchUpdated stamps the updated_at timing, preserving the rest. A nil Timings
// is materialized so the stamp is never dropped.
func touchUpdated(t *common.Timings, now *timestamppb.Timestamp) *common.Timings {
	if t == nil {
		t = &common.Timings{}
	}
	t.UpdatedAt = now
	return t
}

// isTerminal reports whether a suite run has already reached a final state, so
// cancellation is a no-op (idempotency).
func isTerminal(st common.Status) bool {
	switch st {
	case common.Status_STATUS_COMPLETED,
		common.Status_STATUS_FAILED,
		common.Status_STATUS_SKIPPED,
		common.Status_STATUS_CANCELLED:
		return true
	default:
		return false
	}
}

/*
	===== handlers =====
*/

func (s *SuiteRunService) GetSuiteRun(ctx context.Context, req *api.GetSuiteRunRequest) (*api.GetSuiteRunResponse, error) {
	rec, err := s.getOwned(ctx, req.GetTenantId(), req.GetId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.GetSuiteRunResponse{SuiteRun: rec}, nil
}

func (s *SuiteRunService) ListSuiteRuns(ctx context.Context, req *api.ListSuiteRunsRequest) (*api.ListSuiteRunsResponse, error) {
	records, next, err := s.d.SuiteRuns.List(ctx, req)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ListSuiteRunsResponse{SuiteRuns: records, NextPageToken: next}, nil
}

// CancelSuiteRun requests cancellation of the suite run and its children. It is
// idempotent: cancelling a finished/cancelled suite run is a no-op that returns
// the row unchanged. An in-flight run is flipped to CANCELLING and the runtime
// is signalled, both inside one transaction.
func (s *SuiteRunService) CancelSuiteRun(ctx context.Context, req *api.CancelSuiteRunRequest) (*api.CancelSuiteRunResponse, error) {
	rec, err := doTxRet(ctx, s, func(ctx context.Context) (*models.SuiteRunRecord, error) {
		rec, err := s.getOwned(ctx, req.GetTenantId(), req.GetId())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		// Idempotent: a run that already finished (or is already being
		// cancelled) needs no further action.
		if isTerminal(rec.GetStatus()) || rec.GetStatus() == common.Status_STATUS_CANCELLING {
			return rec, nil
		}
		if err := s.d.Canceller.CancelSuiteRun(ctx, rec.GetEntity().GetId()); err != nil {
			return nil, utils.MapErr(err)
		}
		rec.Status = common.Status_STATUS_CANCELLING
		if rec.Entity != nil {
			rec.Entity.Timings = touchUpdated(rec.Entity.GetTimings(), s.now())
		}
		if err := s.d.SuiteRuns.Update(ctx, rec); err != nil {
			return nil, utils.MapErr(err)
		}
		return rec, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.CancelSuiteRunResponse{SuiteRun: rec}, nil
}

// DeleteSuiteRun is idempotent: deleting an absent suite run is a no-op.
func (s *SuiteRunService) DeleteSuiteRun(ctx context.Context, req *api.DeleteSuiteRunRequest) (*api.DeleteSuiteRunResponse, error) {
	if err := s.doTx(ctx, func(ctx context.Context) error {
		rec, err := s.getOwned(ctx, req.GetTenantId(), req.GetId())
		if errors.Is(err, derrors.ErrNotFound) {
			return nil
		}
		if err != nil {
			return utils.MapErr(err)
		}
		return utils.MapErr(derrors.IgnoreNotFound(s.d.SuiteRuns.Delete(ctx, rec.GetEntity().GetId())))
	}); err != nil {
		return nil, err
	}
	return &api.DeleteSuiteRunResponse{}, nil
}
