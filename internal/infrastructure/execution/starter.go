package execution

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	packagecatalog "github.com/stroppy-io/stroppy-cloud/internal/domain/packages"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	domain "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_run"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_wizard"
)

// CallerSource resolves the acting account id for author stamping inside the
// wizard's Start path (the wizard service has already authenticated the caller;
// the starter only needs the id). Implemented by the integration layer.
type CallerSource interface {
	AccountID(ctx context.Context) (string, error)
}

// TestRunStarter implements test_wizard.TestRunStarter: the wizard's
// Finish(start=true) path. It mirrors test_run.StartTestRun — mint a fresh record
// (new id, PENDING, summarized), persist it through the run repo, then return a
// post-commit TestWorkflow starter. It deliberately reuses the same TestRunRepo / Summarizer /
// Workflows ports the TestRun service uses, so the started run is observable by
// the overview service exactly like an API-started one.
//
// Start participates in the ambient ctx transaction for the persist; the launch
// is returned as a post-commit callback. Temporal must not be called from the
// wizard service's serializable transaction callback.
type TestRunStarter struct {
	runs       test_run.TestRunRepo
	summarizer test_run.Summarizer
	workflows  test_run.Workflows
	caller     CallerSource
	packages   packagecatalog.PackageRecordGetter
}

var _ test_wizard.TestRunStarter = (*TestRunStarter)(nil)

// NewTestRunStarter builds the test_wizard.TestRunStarter adapter from the same
// run-persistence, summarization and workflow ports the TestRun service uses.
// caller may be nil (the run is then stamped with an empty author id).
func NewTestRunStarter(runs test_run.TestRunRepo, summarizer test_run.Summarizer, workflows test_run.Workflows, caller CallerSource, packages packagecatalog.PackageRecordGetter) *TestRunStarter {
	return &TestRunStarter{runs: runs, summarizer: summarizer, workflows: workflows, caller: caller, packages: packages}
}

// Start persists a brand-new run record for the baked spec and returns its
// post-commit TestWorkflow starter. The spec is cloned and given the
// server-minted run id so runtime observations key off it.
func (s *TestRunStarter) Start(
	ctx context.Context,
	tenantID string,
	run *domain.TestRun,
	trigger commonpb.Trigger,
	inTenant, inGlobal bool,
) (*models.TestRunRecord, func(context.Context) error, error) {
	spec := proto.Clone(run).(*domain.TestRun)
	runID := uuid.NewString()
	spec.Id = runID
	if err := packagecatalog.MaterializeTestRunPackages(ctx, tenantID, spec, s.packages); err != nil {
		return nil, nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := spec.ValidateAll(); err != nil {
		return nil, nil, status.Error(codes.InvalidArgument, err.Error())
	}

	authorID := ""
	if s.caller != nil {
		id, err := s.caller.AccountID(ctx)
		if err != nil {
			return nil, nil, err
		}
		authorID = id
	}

	now := timestamppb.New(time.Now())
	rec := &models.TestRunRecord{
		Entity: &commonpb.Entity{
			Id:       runID,
			TenantId: tenantID,
			Name:     specName(spec),
			AuthorId: authorID,
			Timings:  &commonpb.Timings{CreatedAt: now, UpdatedAt: now},
		},
		Spec:           spec,
		Status:         commonpb.Status_STATUS_PENDING,
		Trigger:        trigger,
		InTenantRating: inTenant,
		InGlobalRating: inGlobal,
		Summary:        s.summarizer.Summarize(spec),
	}

	// Persist inside the ambient transaction.
	if err := s.runs.Create(ctx, rec); err != nil {
		return nil, nil, err
	}
	start := func(ctx context.Context) error {
		if err := s.workflows.LaunchTest(ctx, rec); err != nil {
			if ferr := s.finishFailed(ctx, rec); ferr != nil {
				return ferr
			}
			return err
		}
		return nil
	}
	return rec, start, nil
}

func (s *TestRunStarter) finishFailed(ctx context.Context, rec *models.TestRunRecord) error {
	now := timestamppb.New(time.Now())
	rec.Status = commonpb.Status_STATUS_FAILED
	if rec.Summary == nil {
		rec.Summary = &models.TestRunRecord_Summary{}
	}
	if rec.Summary.StartedAt == nil {
		rec.Summary.StartedAt = now
	}
	if rec.Summary.FinishedAt == nil {
		rec.Summary.FinishedAt = now
	}
	if start := rec.Summary.GetStartedAt(); start != nil {
		d := now.AsTime().Sub(start.AsTime())
		if d < 0 {
			d = 0
		}
		rec.Summary.Duration = durationpb.New(d)
	}
	if rec.Entity != nil {
		if rec.Entity.Timings == nil {
			rec.Entity.Timings = &commonpb.Timings{}
		}
		rec.Entity.Timings.UpdatedAt = now
	}
	return s.runs.Update(ctx, rec)
}

// specName derives a human label for the run record from the baked workload
// version, falling back to a generic label (mirrors test_run.specName).
func specName(spec *domain.TestRun) string {
	if v := spec.GetWorkload().GetStroppyVersion(); v != "" {
		return "stroppy " + v
	}
	return "test run"
}
