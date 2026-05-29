package suite

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

/*
	===== Suite runs (StartSuite) =====
*/

// StartSuite expands a suite into a SuiteRunRecord (one child TestRunRecord per
// compatible cell) and launches SuiteWorkflow. Not idempotent: each call mints a
// fresh run. The source is either a stored suite_id (re-run: the definition is
// loaded from the tenant and must exist & belong to it) or a fully-baked suite
// passed directly (CLI path). Rating membership flows request -> suite defaults
// -> platform defaults (tenant true, global false). The expand+persist+launch
// runs in one transaction; the launcher validates expandability first so an
// un-runnable suite fails before any write.
func (s *SuiteService) StartSuite(ctx context.Context, req *api.StartSuiteRequest) (*api.StartSuiteResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}

	run, err := doTxRet(ctx, s, func(ctx context.Context) (*models.SuiteRunRecord, error) {
		spec, suiteID, err := s.resolveSource(ctx, req)
		if err != nil {
			return nil, err
		}
		if err := s.d.Launcher.Validate(ctx, req.GetTenantId(), spec); err != nil {
			return nil, utils.MapErr(err)
		}

		inTenant, inGlobal := resolveRating(req, spec)

		runID := uuid.NewString()
		run := &models.SuiteRunRecord{
			Entity: &common.Entity{
				Id:       runID,
				TenantId: req.GetTenantId(),
				Name:     spec.GetId(),
				AuthorId: c.GetAccountId(),
				Timings: &common.Timings{
					CreatedAt: s.now(),
					UpdatedAt: s.now(),
				},
			},
			SuiteId:     suiteID,
			Status:      common.Status_STATUS_PENDING,
			Trigger:     common.Trigger_TRIGGER_API,
			MaxParallel: req.GetMaxParallel(),
		}
		// Carry the resolved rating onto the spec so the launcher propagates it to
		// every child TestRun it expands.
		spec.DefaultInTenantRating = &inTenant
		spec.DefaultInGlobalRating = &inGlobal

		if err := s.d.Launcher.Launch(ctx, run, spec); err != nil {
			return nil, utils.MapErr(err)
		}
		return run, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.StartSuiteResponse{SuiteRun: run}, nil
}

// resolveSource yields the suite definition to expand and the originating
// suite_id (empty for an inline CLI suite). A stored suite_id is loaded from the
// tenant — NotFound for an absent/cross-tenant id; an inline suite is used
// as-is. Exactly one of the oneof arms must be set (enforced by proto, defended
// here).
func (s *SuiteService) resolveSource(ctx context.Context, req *api.StartSuiteRequest) (*domain.Suite, string, error) {
	switch src := req.GetSource().(type) {
	case *api.StartSuiteRequest_SuiteId:
		rec, err := s.d.Suites.Get(ctx, req.GetTenantId(), src.SuiteId)
		if err != nil {
			return nil, "", utils.MapErr(err)
		}
		if rec.GetSpec() == nil {
			return nil, "", status.Error(codes.FailedPrecondition, "stored suite has no spec to expand")
		}
		return rec.GetSpec(), rec.GetEntity().GetId(), nil
	case *api.StartSuiteRequest_Suite:
		if src.Suite == nil {
			return nil, "", status.Error(codes.InvalidArgument, "inline suite is required")
		}
		return src.Suite, "", nil
	default:
		return nil, "", status.Error(codes.InvalidArgument, "source (suite_id or suite) is required")
	}
}

// resolveRating applies the documented precedence for a run's rating membership:
// explicit request flag, else the suite's default, else the platform default
// (tenant true, global false).
func resolveRating(req *api.StartSuiteRequest, spec *domain.Suite) (inTenant, inGlobal bool) {
	switch {
	case req.InTenantRating != nil:
		inTenant = req.GetInTenantRating()
	case spec.DefaultInTenantRating != nil:
		inTenant = spec.GetDefaultInTenantRating()
	default:
		inTenant = true
	}
	switch {
	case req.InGlobalRating != nil:
		inGlobal = req.GetInGlobalRating()
	case spec.DefaultInGlobalRating != nil:
		inGlobal = spec.GetDefaultInGlobalRating()
	default:
		inGlobal = false
	}
	return inTenant, inGlobal
}
