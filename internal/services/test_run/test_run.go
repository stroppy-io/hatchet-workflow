package test_run

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
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

	// A run is always launched as a brand-new record with a server-minted id; the
	// baked spec carries that same id so runtime observations key off it.
	runID := uuid.NewString()
	spec.Id = runID
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
	// transaction. A launch failure surfaces as Internal; the record stays PENDING
	// for a reconciler/retry to pick up.
	if err := s.d.Workflows.LaunchTest(ctx, rec); err != nil {
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
	return &api.GetTestRunResponse{Run: rec}, nil
}

func (s *TestRunService) ListTestRuns(ctx context.Context, req *api.ListTestRunsRequest) (*api.ListTestRunsResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	runs, next, err := s.d.Runs.List(ctx, req)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ListTestRunsResponse{Runs: runs, NextPageToken: next}, nil
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
		if err := s.d.Workflows.CancelTest(ctx, rec.GetEntity().GetId()); err != nil && !errors.Is(err, derrors.ErrNotFound) {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return &api.CancelTestRunResponse{Run: rec}, nil
}

/*
DeleteTestRun removes a run. Idempotent: deleting an absent run is a no-op.
A still-active run is best-effort cancelled first so we never orphan a running
workflow behind a deleted record.
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
	if !isTerminal(rec.GetStatus()) {
		if err := s.d.Workflows.CancelTest(ctx, rec.GetEntity().GetId()); err != nil && !errors.Is(err, derrors.ErrNotFound) {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	if err := s.doTx(ctx, func(ctx context.Context) error {
		return utils.MapErr(derrors.IgnoreNotFound(s.d.Runs.Delete(ctx, req.GetTenantId(), req.GetId())))
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
