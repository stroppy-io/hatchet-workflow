package execution

import (
	"context"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domainpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

// RunPersistenceStore persists a models.Run — SP-E Task 3's cutover: the
// live RunRecipeWorkflow write path now targets Run/run_records instead of
// TestRunRecord/test_run_records (see internal/app/glue.go's
// runtimePersistenceStore for the postgres.RunRepo-backed implementation).
// TestRunRecord/test_run_records are not written by this package anymore,
// though they remain readable (OverviewReader et al.) until SP-E Tasks 4-6
// migrate those readers.
type RunPersistenceStore interface {
	RunRecord(ctx context.Context, runID string) (*models.Run, error)
	SaveRunRecord(ctx context.Context, run *models.Run) error
}

type RunPersistenceActivities struct {
	store RunPersistenceStore
	logs  *RunLogWriter
}

func NewRunPersistenceActivities(store RunPersistenceStore, logs ...*RunLogWriter) *RunPersistenceActivities {
	a := &RunPersistenceActivities{store: store}
	if len(logs) > 0 {
		a.logs = logs[0]
	}
	return a
}

// PersistRunState persists state's stage tree + derived status/progress onto
// the run record. infrastructureState/deploymentPlan are accepted for
// interface stability with RuntimeActivities (register.go) — a recipe run
// (models.Run) produces neither (see runrecipe.go's persist doc: its
// provisioning result is a plain map[string][]*deploymentpb.MachineState,
// not a deployment.InfrastructureState/DeploymentPlan), so both are simply
// ignored/no-op here rather than stored.
func (a *RunPersistenceActivities) PersistRunState(
	ctx context.Context,
	runID string,
	state *workflowpb.RunState,
	_ *deploymentpb.InfrastructureState,
	_ *deploymentpb.DeploymentPlan,
) error {
	if a == nil || a.store == nil || runID == "" {
		return nil
	}
	rec, err := a.store.RunRecord(ctx, runID)
	if err != nil {
		return err
	}

	now := time.Now()
	rec.Status = nextRunStatus(rec.GetStatus(), state.GetStatus())
	if rec.Summary == nil {
		rec.Summary = &models.Run_Summary{}
	}
	applyRunSummary(rec.Summary, state, rec.GetStatus(), now)
	if state != nil {
		rec.RuntimeState = proto.Clone(state).(*workflowpb.RunState)
	}
	touchRecordUpdated(rec.GetEntity(), now)

	return a.store.SaveRunRecord(ctx, rec)
}

// PersistRunSummary merges summary's non-zero recipe-derived facets
// (Provider/NodeCount/TopologyLabel/DbKind/WorkloadName/StroppyVersion —
// see workflows.deriveRunSummary) onto the run record's stored Summary.
// Additive by design (mergeRunSummary below): it never clobbers the
// timing/progress facets PersistRunState's own applyRunSummary maintains on
// every stage transition, nor a field summary itself leaves at its zero
// value (e.g. a recipe whose stroppy service could not be identified keeps
// whatever WorkloadName/StroppyVersion — none yet — the record already
// has).
func (a *RunPersistenceActivities) PersistRunSummary(ctx context.Context, runID string, summary *models.Run_Summary) error {
	if a == nil || a.store == nil || runID == "" || summary == nil {
		return nil
	}
	rec, err := a.store.RunRecord(ctx, runID)
	if err != nil {
		return err
	}
	if rec.Summary == nil {
		rec.Summary = &models.Run_Summary{}
	}
	mergeRunSummary(rec.Summary, summary)
	touchRecordUpdated(rec.GetEntity(), time.Now())

	return a.store.SaveRunRecord(ctx, rec)
}

// mergeRunSummary copies every non-zero field of src onto dst, leaving
// dst's existing value untouched where src is zero — see PersistRunSummary's
// doc for why this must be additive rather than a wholesale overwrite.
func mergeRunSummary(dst, src *models.Run_Summary) {
	if src.GetDbKind() != domainpb.Database_KIND_UNSPECIFIED {
		dst.DbKind = src.GetDbKind()
	}
	if src.GetProvider() != deploymentpb.Provider_PROVIDER_UNSPECIFIED {
		dst.Provider = src.GetProvider()
	}
	if src.GetNodeCount() != 0 {
		dst.NodeCount = src.GetNodeCount()
	}
	if src.GetTopologyLabel() != "" {
		dst.TopologyLabel = src.GetTopologyLabel()
	}
	if src.GetWorkloadName() != "" {
		dst.WorkloadName = src.GetWorkloadName()
	}
	if src.GetStroppyVersion() != "" {
		dst.StroppyVersion = src.GetStroppyVersion()
	}
}

// PersistRecipeTopology replaces the run record's topology field wholesale
// with snapshot — a full-replace write mirroring PersistRunCompiledPlan just
// below (not a merge like PersistRunSummary): RunRecipeWorkflow builds the
// snapshot exactly once, right after ProvisionActivity succeeds (see
// workflows.deriveRecipeTopology), so there is no accreting-facets concern a
// merge would otherwise guard against.
func (a *RunPersistenceActivities) PersistRecipeTopology(ctx context.Context, runID string, snapshot *models.RunTopology) error {
	if a == nil || a.store == nil || runID == "" || snapshot == nil {
		return nil
	}
	rec, err := a.store.RunRecord(ctx, runID)
	if err != nil {
		return err
	}
	now := time.Now()
	rec.Topology = proto.Clone(snapshot).(*models.RunTopology)
	touchRecordUpdated(rec.GetEntity(), now)
	return a.store.SaveRunRecord(ctx, rec)
}

// PersistRunCompiledPlan replaces the run record's compiled_plan field
// wholesale with plan — called exactly once by RunRecipeWorkflow, right
// after CompileRecipeActivity succeeds (see runtime.go's
// persistRunCompiledPlan and runrecipe.go's run). SP-E Task 3: before this
// activity existed, the compiled plan lived only in workflow memory and was
// never durably stored.
func (a *RunPersistenceActivities) PersistRunCompiledPlan(ctx context.Context, runID string, plan *dslpb.CompiledPlan) error {
	if a == nil || a.store == nil || runID == "" || plan == nil {
		return nil
	}
	rec, err := a.store.RunRecord(ctx, runID)
	if err != nil {
		return err
	}
	now := time.Now()
	rec.CompiledPlan = proto.Clone(plan).(*dslpb.CompiledPlan)
	touchRecordUpdated(rec.GetEntity(), now)
	return a.store.SaveRunRecord(ctx, rec)
}

// PersistDeploymentPlan is kept for RuntimeActivities interface stability
// (register.go) but is a documented no-op for a models.Run-backed store: Run
// has no deployment_plan field (see runrecipe.go's persist doc — a recipe
// run's provisioning result is a plain map[string][]*deploymentpb.
// MachineState, not a deployment.DeploymentPlan) and RunRecipeWorkflow never
// calls this activity by name (grep internal/workflows: only the interface/
// registration reference it). Retained rather than removed so
// RuntimeActivities/RegisterActivities need no further churn if a future
// caller starts calling it.
func (a *RunPersistenceActivities) PersistDeploymentPlan(_ context.Context, _ string, _ *deploymentpb.DeploymentPlan) error {
	return nil
}

func (a *RunPersistenceActivities) AppendRunLogs(ctx context.Context, lines []*monitor.LogLine) error {
	if a == nil || a.logs == nil {
		return nil
	}
	return a.logs.Write(ctx, lines)
}

func nextRunStatus(current, incoming commonpb.Status) commonpb.Status {
	if incoming == commonpb.Status_STATUS_UNSPECIFIED {
		return current
	}
	if isTerminalStatus(current) {
		return current
	}
	if current == commonpb.Status_STATUS_CANCELLING && !isTerminalStatus(incoming) {
		return current
	}
	return incoming
}

func applyRunSummary(summary *models.Run_Summary, state *workflowpb.RunState, status commonpb.Status, now time.Time) {
	if summary == nil || state == nil {
		return
	}
	var (
		earliestStart *timestamppb.Timestamp
		latestFinish  *timestamppb.Timestamp
		completed     int
	)
	for _, stage := range state.GetStages() {
		if started := stage.GetStartedAt(); started != nil && (earliestStart == nil || started.AsTime().Before(earliestStart.AsTime())) {
			earliestStart = started
		}
		if finished := stage.GetFinishedAt(); finished != nil && (latestFinish == nil || finished.AsTime().After(latestFinish.AsTime())) {
			latestFinish = finished
		}
		if isTerminalStatus(stage.GetStatus()) {
			completed++
		}
	}
	if earliestStart != nil {
		summary.StartedAt = earliestStart
	}
	if isTerminalStatus(status) {
		summary.FinishedAt = latestFinish
	}
	summary.Duration = spanDuration(summary.GetStartedAt(), summary.GetFinishedAt(), now)
	summary.ProgressPct = progressPct(completed, len(state.GetStages()))
}

func touchRecordUpdated(entity *commonpb.Entity, now time.Time) {
	if entity == nil {
		return
	}
	if entity.Timings == nil {
		entity.Timings = &commonpb.Timings{}
	}
	entity.Timings.UpdatedAt = timestamppb.New(now)
}
