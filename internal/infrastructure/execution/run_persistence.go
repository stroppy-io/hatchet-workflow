package execution

import (
	"context"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domainpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

type RunPersistenceStore interface {
	RunRecord(ctx context.Context, runID string) (*models.TestRunRecord, error)
	SaveRunRecord(ctx context.Context, run *models.TestRunRecord) error
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

func (a *RunPersistenceActivities) PersistRunState(
	ctx context.Context,
	runID string,
	state *workflowpb.RunState,
	infrastructureState *deploymentpb.InfrastructureState,
	deploymentPlan *deploymentpb.DeploymentPlan,
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
		rec.Summary = &models.TestRunRecord_Summary{}
	}
	applyRunSummary(rec.Summary, state, rec.GetStatus(), now)
	if state != nil {
		rec.RuntimeState = proto.Clone(state).(*workflowpb.RunState)
	}
	if infrastructureState != nil {
		rec.InfrastructureState = proto.Clone(infrastructureState).(*deploymentpb.InfrastructureState)
	}
	if deploymentPlan != nil {
		rec.DeploymentPlan = proto.Clone(deploymentPlan).(*deploymentpb.DeploymentPlan)
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
func (a *RunPersistenceActivities) PersistRunSummary(ctx context.Context, runID string, summary *models.TestRunRecord_Summary) error {
	if a == nil || a.store == nil || runID == "" || summary == nil {
		return nil
	}
	rec, err := a.store.RunRecord(ctx, runID)
	if err != nil {
		return err
	}
	if rec.Summary == nil {
		rec.Summary = &models.TestRunRecord_Summary{}
	}
	mergeRunSummary(rec.Summary, summary)
	touchRecordUpdated(rec.GetEntity(), time.Now())

	return a.store.SaveRunRecord(ctx, rec)
}

// mergeRunSummary copies every non-zero field of src onto dst, leaving
// dst's existing value untouched where src is zero — see PersistRunSummary's
// doc for why this must be additive rather than a wholesale overwrite.
func mergeRunSummary(dst, src *models.TestRunRecord_Summary) {
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

func (a *RunPersistenceActivities) PersistDeploymentPlan(ctx context.Context, runID string, deploymentPlan *deploymentpb.DeploymentPlan) error {
	if a == nil || a.store == nil || runID == "" || deploymentPlan == nil {
		return nil
	}
	rec, err := a.store.RunRecord(ctx, runID)
	if err != nil {
		return err
	}
	now := time.Now()
	rec.DeploymentPlan = proto.Clone(deploymentPlan).(*deploymentpb.DeploymentPlan)
	touchRecordUpdated(rec.GetEntity(), now)
	return a.store.SaveRunRecord(ctx, rec)
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

func applyRunSummary(summary *models.TestRunRecord_Summary, state *workflowpb.RunState, status commonpb.Status, now time.Time) {
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
