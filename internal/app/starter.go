package app

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	domain "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_run"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_wizard"
	"github.com/stroppy-io/stroppy-cloud/internal/workflows"
)

// runStarter implements test_wizard.TestRunStarter: the wizard's Finish(start=true)
// path. It mirrors test_run.StartTestRun: mint a fresh record (new id, PENDING,
// summarized), persist it, then launch the TestWorkflow outside any transaction
// (the launch is external IO). The persisted record is the same row the overview
// service later reads, so Exists(...) succeeds for it.
type runStarter struct {
	runs       test_run.TestRunRepo
	summarizer test_run.Summarizer
	launcher   *workflows.Launcher
}

var _ test_wizard.TestRunStarter = (*runStarter)(nil)

func newRunStarter(runs test_run.TestRunRepo, sum test_run.Summarizer, l *workflows.Launcher) *runStarter {
	return &runStarter{runs: runs, summarizer: sum, launcher: l}
}

func (s *runStarter) Start(ctx context.Context, tenantID string, run *domain.TestRun, trigger commonpb.Trigger, inTenant, inGlobal bool) (*models.TestRunRecord, error) {
	spec := proto.Clone(run).(*domain.TestRun)
	runID := uuid.NewString()
	spec.Id = runID

	now := timestamppb.New(time.Now())
	rec := &models.TestRunRecord{
		Entity: &commonpb.Entity{
			Id:       runID,
			TenantId: tenantID,
			Name:     specName(spec),
			AuthorId: demoAccountID,
			Timings:  &commonpb.Timings{CreatedAt: now, UpdatedAt: now},
		},
		Spec:           spec,
		Status:         commonpb.Status_STATUS_PENDING,
		Trigger:        trigger,
		InTenantRating: inTenant,
		InGlobalRating: inGlobal,
		Summary:        s.summarizer.Summarize(spec),
	}

	if err := s.runs.Create(ctx, rec); err != nil {
		return nil, err
	}
	// Launch is external IO: it happens after the record is persisted. A failure
	// surfaces to the caller; the record stays PENDING.
	if err := s.launcher.LaunchTest(ctx, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// specName derives a human label for the run record from the baked workload
// version, falling back to a generic label (mirrors test_run.specName).
func specName(spec *domain.TestRun) string {
	if v := spec.GetWorkload().GetStroppyVersion(); v != "" {
		return "stroppy " + v
	}
	return "test run"
}
