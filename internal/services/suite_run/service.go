package suite_run

import (
	"context"
	"errors"
	"time"

	"github.com/gopherex/pgtx/pkg/tx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
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
	List(ctx context.Context, req *api.ListSuiteRunsRequest, callerAccountID string) (records []*models.SuiteRunRecord, nextPageToken string, err error)
	Update(ctx context.Context, record *models.SuiteRunRecord) error
}

// SuiteChildRunRepo updates child TestRunRecord rows when suite-level
// cancellation changes durable child status before individual child workflows
// have a chance to persist their final state.
type SuiteChildRunRepo interface {
	Get(ctx context.Context, tenantID, id string) (*models.TestRunRecord, error)
	Update(ctx context.Context, record *models.TestRunRecord) error
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
	Runs      SuiteChildRunRepo
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

func markSuiteDeleted(rec *models.SuiteRunRecord, now *timestamppb.Timestamp) {
	if rec.Entity == nil {
		rec.Entity = &common.Entity{}
	}
	rec.Entity.Timings = touchUpdated(rec.Entity.GetTimings(), now)
	rec.Entity.Timings.DeletedAt = now
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
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	records, next, err := s.d.SuiteRuns.List(ctx, req, c.GetAccountId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ListSuiteRunsResponse{SuiteRuns: records, NextPageToken: next}, nil
}

// CancelSuiteRun requests cancellation of the suite run and its children. It is
// idempotent: cancelling a finished/cancelled suite run is a no-op that returns
// the row unchanged. An in-flight run is flipped to CANCELLING durably before
// the runtime is signalled; if the runtime workflow is already absent, the
// durable state is finalized as CANCELLED.
func (s *SuiteRunService) CancelSuiteRun(ctx context.Context, req *api.CancelSuiteRunRequest) (*api.CancelSuiteRunResponse, error) {
	rec, err := s.markSuiteCancelling(ctx, req.GetTenantId(), req.GetId())
	if err != nil {
		return nil, err
	}
	if rec.GetStatus() == common.Status_STATUS_CANCELLING {
		if err := s.d.Canceller.CancelSuiteRun(ctx, rec.GetEntity().GetId()); err != nil {
			if !errors.Is(err, derrors.ErrNotFound) {
				return nil, utils.MapErr(err)
			}
			rec, err = s.finishCancelledSuite(ctx, req.GetTenantId(), req.GetId())
			if err != nil {
				return nil, err
			}
		}
	}
	return &api.CancelSuiteRunResponse{SuiteRun: rec}, nil
}

func (s *SuiteRunService) markSuiteCancelling(ctx context.Context, tenantID, suiteRunID string) (*models.SuiteRunRecord, error) {
	return doTxRet(ctx, s, func(ctx context.Context) (*models.SuiteRunRecord, error) {
		rec, err := s.getOwned(ctx, tenantID, suiteRunID)
		if err != nil {
			return nil, utils.MapErr(err)
		}
		if isTerminal(rec.GetStatus()) {
			return rec, nil
		}
		if rec.GetStatus() == common.Status_STATUS_CANCELLING {
			now := s.now()
			markSuiteChildren(rec, common.Status_STATUS_CANCELLING)
			if err := s.updateChildRuns(ctx, tenantID, rec, common.Status_STATUS_CANCELLING, now); err != nil {
				return nil, err
			}
			if err := s.d.SuiteRuns.Update(ctx, rec); err != nil {
				return nil, utils.MapErr(err)
			}
			return rec, nil
		}

		now := s.now()
		rec.Status = common.Status_STATUS_CANCELLING
		markSuiteChildren(rec, common.Status_STATUS_CANCELLING)
		if rec.Entity != nil {
			rec.Entity.Timings = touchUpdated(rec.Entity.GetTimings(), now)
		}
		if err := s.updateChildRuns(ctx, tenantID, rec, common.Status_STATUS_CANCELLING, now); err != nil {
			return nil, err
		}
		if err := s.d.SuiteRuns.Update(ctx, rec); err != nil {
			return nil, utils.MapErr(err)
		}
		return rec, nil
	})
}

func (s *SuiteRunService) finishCancelledSuite(ctx context.Context, tenantID, suiteRunID string) (*models.SuiteRunRecord, error) {
	return doTxRet(ctx, s, func(ctx context.Context) (*models.SuiteRunRecord, error) {
		rec, err := s.getOwned(ctx, tenantID, suiteRunID)
		if err != nil {
			return nil, utils.MapErr(err)
		}
		if isTerminal(rec.GetStatus()) {
			return rec, nil
		}

		now := s.now()
		rec.Status = common.Status_STATUS_CANCELLED
		markSuiteChildren(rec, common.Status_STATUS_CANCELLED)
		applySuiteSummary(rec, now)
		if rec.Entity != nil {
			rec.Entity.Timings = touchUpdated(rec.Entity.GetTimings(), now)
		}
		if err := s.updateChildRuns(ctx, tenantID, rec, common.Status_STATUS_CANCELLED, now); err != nil {
			return nil, err
		}
		if err := s.d.SuiteRuns.Update(ctx, rec); err != nil {
			return nil, utils.MapErr(err)
		}
		return rec, nil
	})
}

func (s *SuiteRunService) updateChildRuns(ctx context.Context, tenantID string, suiteRun *models.SuiteRunRecord, status common.Status, now *timestamppb.Timestamp) error {
	if s.d.Runs == nil {
		return nil
	}
	for _, child := range suiteRun.GetChildren() {
		run, err := s.d.Runs.Get(ctx, tenantID, child.GetTestRunId())
		if errors.Is(err, derrors.ErrNotFound) {
			continue
		}
		if err != nil {
			return utils.MapErr(err)
		}
		if isTerminal(run.GetStatus()) {
			continue
		}
		run.Status = status
		if status == common.Status_STATUS_CANCELLED {
			markRunCancelled(run, now)
		} else if run.Entity != nil {
			run.Entity.Timings = touchUpdated(run.Entity.GetTimings(), now)
		}
		if err := s.d.Runs.Update(ctx, run); err != nil {
			return utils.MapErr(err)
		}
	}
	return nil
}

func markSuiteChildren(rec *models.SuiteRunRecord, status common.Status) {
	for _, child := range rec.GetChildren() {
		if isTerminal(child.GetStatus()) {
			continue
		}
		child.Status = status
	}
}

func markRunCancelled(run *models.TestRunRecord, now *timestamppb.Timestamp) {
	run.Status = common.Status_STATUS_CANCELLED
	if run.Summary == nil {
		run.Summary = &models.TestRunRecord_Summary{}
	}
	if run.Summary.StartedAt == nil {
		run.Summary.StartedAt = now
	}
	if run.Summary.FinishedAt == nil {
		run.Summary.FinishedAt = now
	}
	if start := run.Summary.GetStartedAt(); start != nil && now != nil {
		d := now.AsTime().Sub(start.AsTime())
		if d < 0 {
			d = 0
		}
		run.Summary.Duration = durationpb.New(d)
	}
	if run.Entity != nil {
		run.Entity.Timings = touchUpdated(run.Entity.GetTimings(), now)
	}
}

func applySuiteSummary(rec *models.SuiteRunRecord, now *timestamppb.Timestamp) {
	if rec.Summary == nil {
		rec.Summary = &models.SuiteRunRecord_Summary{}
	}
	var completed, failed, running, pending uint32
	for _, child := range rec.GetChildren() {
		switch child.GetStatus() {
		case common.Status_STATUS_COMPLETED:
			completed++
		case common.Status_STATUS_FAILED, common.Status_STATUS_CANCELLED, common.Status_STATUS_SKIPPED:
			failed++
		case common.Status_STATUS_RUNNING,
			common.Status_STATUS_CANCELLING,
			common.Status_STATUS_ALLOCATED,
			common.Status_STATUS_DEPLOYMENT,
			common.Status_STATUS_DEPLOYED:
			running++
		default:
			pending++
		}
	}
	rec.Summary.Total = uint32(len(rec.GetChildren()))
	rec.Summary.Completed = completed
	rec.Summary.Failed = failed
	rec.Summary.Running = running
	rec.Summary.Pending = pending
	rec.Summary.ProgressPct = progressPct(completed+failed, uint32(len(rec.GetChildren())))
	if rec.Summary.StartedAt == nil {
		rec.Summary.StartedAt = now
	}
	if rec.Summary.FinishedAt == nil {
		rec.Summary.FinishedAt = now
	}
	if start := rec.Summary.GetStartedAt(); start != nil && now != nil {
		d := now.AsTime().Sub(start.AsTime())
		if d < 0 {
			d = 0
		}
		rec.Summary.Duration = durationpb.New(d)
	}
}

func progressPct(done, total uint32) uint32 {
	if total == 0 {
		return 0
	}
	return done * 100 / total
}

// DeleteSuiteRun is idempotent: soft-deleting an absent or already-deleted
// suite run is a no-op. Active suite runs are cancelled before being hidden from
// normal list responses so the workflow cannot continue behind a deleted row.
func (s *SuiteRunService) DeleteSuiteRun(ctx context.Context, req *api.DeleteSuiteRunRequest) (*api.DeleteSuiteRunResponse, error) {
	shouldCancel, err := s.markSuiteDeleted(ctx, req.GetTenantId(), req.GetId())
	if err != nil {
		return nil, err
	}
	if shouldCancel {
		if err := s.d.Canceller.CancelSuiteRun(ctx, req.GetId()); err != nil {
			if !errors.Is(err, derrors.ErrNotFound) {
				return nil, utils.MapErr(err)
			}
			if err := s.finalizeDeletedSuite(ctx, req.GetTenantId(), req.GetId()); err != nil {
				return nil, err
			}
		}
	}
	return &api.DeleteSuiteRunResponse{}, nil
}

func (s *SuiteRunService) markSuiteDeleted(ctx context.Context, tenantID, suiteRunID string) (bool, error) {
	return tx.DoSerializableRet(ctx, s.d.Tx, func(ctx context.Context) (bool, error) {
		rec, err := s.getOwned(ctx, tenantID, suiteRunID)
		if errors.Is(err, derrors.ErrNotFound) {
			return false, nil
		}
		if err != nil {
			return false, utils.MapErr(err)
		}
		if rec.GetEntity().GetTimings().GetDeletedAt() != nil {
			return false, nil
		}

		now := s.now()
		shouldCancel := false
		if !isTerminal(rec.GetStatus()) {
			rec.Status = common.Status_STATUS_CANCELLING
			markSuiteChildren(rec, common.Status_STATUS_CANCELLING)
			if err := s.updateChildRuns(ctx, tenantID, rec, common.Status_STATUS_CANCELLING, now); err != nil {
				return false, err
			}
			shouldCancel = true
		}
		markSuiteDeleted(rec, now)
		if err := s.d.SuiteRuns.Update(ctx, rec); err != nil {
			return false, utils.MapErr(err)
		}
		return shouldCancel, nil
	}, tx.WithRetry(tx.DefaultRetryPolicy))
}

func (s *SuiteRunService) finalizeDeletedSuite(ctx context.Context, tenantID, suiteRunID string) error {
	return s.doTx(ctx, func(ctx context.Context) error {
		rec, err := s.getOwned(ctx, tenantID, suiteRunID)
		if errors.Is(err, derrors.ErrNotFound) {
			return nil
		}
		if err != nil {
			return utils.MapErr(err)
		}

		now := s.now()
		if !isTerminal(rec.GetStatus()) {
			rec.Status = common.Status_STATUS_CANCELLED
			markSuiteChildren(rec, common.Status_STATUS_CANCELLED)
			applySuiteSummary(rec, now)
			if err := s.updateChildRuns(ctx, tenantID, rec, common.Status_STATUS_CANCELLED, now); err != nil {
				return err
			}
		}
		markSuiteDeleted(rec, now)
		if err := s.d.SuiteRuns.Update(ctx, rec); err != nil {
			return utils.MapErr(err)
		}
		return nil
	})
}
