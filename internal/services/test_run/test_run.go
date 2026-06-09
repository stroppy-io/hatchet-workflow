package test_run

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	packagecatalog "github.com/stroppy-io/stroppy-cloud/internal/domain/packages"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	domain "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

/*
StartTestRun launches a run. Two sources (oneof):
  - run:         a fully-baked domain.TestRun (CLI / wizard finish) — persist a
    brand-new record and start it.
  - test_run_id: re-run an existing record's baked spec as a NEW run.

Either way a fresh record (new id, PENDING, freshly stamped) is persisted and
TestWorkflow is launched. Not idempotent: each call mints a new run.
*/
func (s *TestRunService) StartTestRun(ctx context.Context, req *api.StartTestRunRequest) (*api.StartTestRunResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}

	// Resolve the baked spec to run from the requested source.
	var spec *domain.TestRun
	switch {
	case req.GetRun() != nil:
		spec = cloneSpec(req.GetRun())
	case req.GetTestRunId() != "":
		src, err := s.d.Runs.Get(ctx, req.GetTenantId(), req.GetTestRunId())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		if src.GetSpec() == nil {
			return nil, status.Error(codes.FailedPrecondition, "source run has no spec to re-run")
		}
		spec = cloneSpec(src.GetSpec())
	default:
		return nil, status.Error(codes.InvalidArgument, "run or test_run_id is required")
	}
	if err := packagecatalog.MaterializeTestRunPackages(ctx, req.GetTenantId(), spec, s.d.Packages); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// A run is always launched as a brand-new record with a server-minted id; the
	// baked spec carries that same id so runtime observations key off it.
	runID := uuid.NewString()
	spec.Id = runID
	if err := spec.ValidateAll(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	// Trigger defaults to API for a request that supplies a run directly,
	// otherwise MANUAL (a UI/CLI re-run). Suite-child runs are created by the
	// suite service, never here, so suite_run_id stays empty.
	trigger := commonpb.Trigger_TRIGGER_MANUAL
	if req.GetRun() != nil {
		trigger = commonpb.Trigger_TRIGGER_API
	}

	rec := &models.TestRunRecord{
		Entity: &commonpb.Entity{
			Id:       runID,
			TenantId: req.GetTenantId(),
			Name:     specName(spec),
			AuthorId: c.GetAccountId(),
			Timings:  &commonpb.Timings{CreatedAt: s.now(), UpdatedAt: s.now()},
		},
		Spec:           spec,
		Status:         commonpb.Status_STATUS_PENDING,
		Trigger:        trigger,
		InTenantRating: ratingOrDefault(req.InTenantRating, true),
		InGlobalRating: ratingOrDefault(req.InGlobalRating, false),
		Summary:        s.d.Summarizer.Summarize(spec),
	}

	if err := s.doTx(ctx, func(ctx context.Context) error {
		return utils.MapErr(s.d.Runs.Create(ctx, rec))
	}); err != nil {
		return nil, err
	}

	// Launch is external IO: it MUST happen after the record commits, outside the
	// transaction. If launch fails, close the already-visible record as FAILED so
	// list/overview do not expose an unrecoverable PENDING run forever.
	if err := s.d.Workflows.LaunchTest(ctx, rec); err != nil {
		if ferr := s.finishFailedRun(ctx, req.GetTenantId(), rec.GetEntity().GetId()); ferr != nil {
			return nil, ferr
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &api.StartTestRunResponse{Run: rec}, nil
}

func (s *TestRunService) GetTestRun(ctx context.Context, req *api.GetTestRunRequest) (*api.GetTestRunResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	rec, err := s.d.Runs.Get(ctx, req.GetTenantId(), req.GetId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	// A soft-deleted run is hidden from clients, consistent with ListTestRuns
	// (the repo Get still returns it for internal delete/idempotency checks).
	if rec.GetEntity().GetTimings().GetDeletedAt() != nil {
		return nil, utils.MapErr(derrors.ErrNotFound)
	}
	return &api.GetTestRunResponse{Run: rec}, nil
}

func (s *TestRunService) ListTestRuns(ctx context.Context, req *api.ListTestRunsRequest) (*api.ListTestRunsResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	runs, next, err := s.d.Runs.List(ctx, req, c.GetAccountId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ListTestRunsResponse{Runs: runs, NextPageToken: next}, nil
}

// ListTestRunFacets returns distinct values for the run-list filters using the
// same tenant and entity-filter semantics as ListTestRuns.
func (s *TestRunService) ListTestRunFacets(ctx context.Context, req *api.ListTestRunFacetsRequest) (*api.ListTestRunFacetsResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := s.d.Runs.ListFacets(ctx, req, c.GetAccountId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return resp, nil
}

/*
CancelTestRun requests cancellation: a live run moves to CANCELLING and the
workflow is signalled (it flips to CANCELLED when it actually stops).
Idempotent: cancelling an absent run, or one already in a terminal /
cancelling state, is a no-op that returns the current record.
*/
func (s *TestRunService) CancelTestRun(ctx context.Context, req *api.CancelTestRunRequest) (*api.CancelTestRunResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	rec, err := doTxRet(ctx, s, func(ctx context.Context) (*models.TestRunRecord, error) {
		rec, err := s.d.Runs.Get(ctx, req.GetTenantId(), req.GetId())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		// Already finished or already cancelling -> nothing to do.
		if isTerminal(rec.GetStatus()) || rec.GetStatus() == commonpb.Status_STATUS_CANCELLING {
			return rec, nil
		}
		rec.Status = commonpb.Status_STATUS_CANCELLING
		touchUpdated(rec, s.now())
		if err := s.d.Runs.Update(ctx, rec); err != nil {
			return nil, utils.MapErr(err)
		}
		return rec, nil
	})
	if err != nil {
		return nil, err
	}
	// Signal the workflow outside the transaction; a not-running workflow is a no-op.
	if rec.GetStatus() == commonpb.Status_STATUS_CANCELLING {
		if err := s.d.Workflows.CancelTest(ctx, rec.GetEntity().GetId()); err != nil {
			if !errors.Is(err, derrors.ErrNotFound) {
				return nil, status.Error(codes.Internal, err.Error())
			}
			rec, err = s.finishCancelledRun(ctx, req.GetTenantId(), req.GetId())
			if err != nil {
				return nil, err
			}
		}
	}
	return &api.CancelTestRunResponse{Run: rec}, nil
}

func (s *TestRunService) finishCancelledRun(ctx context.Context, tenantID, runID string) (*models.TestRunRecord, error) {
	return doTxRet(ctx, s, func(ctx context.Context) (*models.TestRunRecord, error) {
		rec, err := s.d.Runs.Get(ctx, tenantID, runID)
		if err != nil {
			return nil, utils.MapErr(err)
		}
		if isTerminal(rec.GetStatus()) {
			return rec, nil
		}
		markCancelled(rec, s.now())
		if err := s.d.Runs.Update(ctx, rec); err != nil {
			return nil, utils.MapErr(err)
		}
		return rec, nil
	})
}

func (s *TestRunService) finishFailedRun(ctx context.Context, tenantID, runID string) error {
	return s.doTx(ctx, func(ctx context.Context) error {
		rec, err := s.d.Runs.Get(ctx, tenantID, runID)
		if err != nil {
			return utils.MapErr(err)
		}
		if isTerminal(rec.GetStatus()) {
			return nil
		}
		markFailed(rec, s.now())
		return utils.MapErr(s.d.Runs.Update(ctx, rec))
	})
}

/*
DeleteTestRun soft-deletes a run. Idempotent: deleting an absent or already
deleted run is a no-op. A still-active run is best-effort cancelled first so we
never hide a row while leaving its workflow running unnoticed.
*/
func (s *TestRunService) DeleteTestRun(ctx context.Context, req *api.DeleteTestRunRequest) (*api.DeleteTestRunResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	rec, err := s.d.Runs.Get(ctx, req.GetTenantId(), req.GetId())
	if errors.Is(err, derrors.ErrNotFound) {
		return &api.DeleteTestRunResponse{}, nil
	}
	if err != nil {
		return nil, utils.MapErr(err)
	}
	cancelMissing := false
	if !isTerminal(rec.GetStatus()) {
		if err := s.d.Workflows.CancelTest(ctx, rec.GetEntity().GetId()); err != nil {
			if errors.Is(err, derrors.ErrNotFound) {
				cancelMissing = true
			} else {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
	}
	if err := s.doTx(ctx, func(ctx context.Context) error {
		current, err := s.d.Runs.Get(ctx, req.GetTenantId(), req.GetId())
		if errors.Is(err, derrors.ErrNotFound) {
			return nil
		}
		if err != nil {
			return utils.MapErr(err)
		}
		if current.GetEntity().GetTimings().GetDeletedAt() != nil {
			return nil
		}
		now := s.now()
		if !isTerminal(current.GetStatus()) {
			if cancelMissing {
				markCancelled(current, now)
			} else {
				current.Status = commonpb.Status_STATUS_CANCELLING
				touchUpdated(current, now)
			}
		}
		markDeleted(current, now)
		if err := s.d.Runs.Update(ctx, current); err != nil {
			return utils.MapErr(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return &api.DeleteTestRunResponse{}, nil
}

/*
ExtractToPreset promotes a run's baked database+workload into a reusable,
tenant-owned TestPresetRecord. Gated by RESOURCE_PRESET (interceptor). Not
idempotent — each call mints a new preset.
*/
func (s *TestRunService) ExtractToPreset(ctx context.Context, req *api.ExtractToPresetRequest) (*api.ExtractToPresetResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	rec, err := s.d.Runs.Get(ctx, req.GetTenantId(), req.GetId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	spec := rec.GetSpec()
	if spec == nil || spec.GetDatabase() == nil || spec.GetWorkload() == nil {
		return nil, status.Error(codes.FailedPrecondition, "run has no database+workload to extract")
	}

	name := req.GetName()
	if name == "" {
		name = derivePresetName(rec)
	}
	presetID := uuid.NewString()
	preset := &models.TestPresetRecord{
		Entity: &commonpb.Entity{
			Id:       presetID,
			TenantId: req.GetTenantId(),
			Name:     name,
			AuthorId: c.GetAccountId(),
			Timings:  &commonpb.Timings{CreatedAt: s.now(), UpdatedAt: s.now()},
		},
		Test: &domain.Test{
			Database: spec.GetDatabase(),
			Workload: spec.GetWorkload(),
			Tags:     spec.GetTags(),
		},
		IsSystem: false,
	}
	if err := s.doTx(ctx, func(ctx context.Context) error {
		return utils.MapErr(s.d.Presets.CreateTestPreset(ctx, preset))
	}); err != nil {
		return nil, err
	}
	return &api.ExtractToPresetResponse{Preset: preset}, nil
}
