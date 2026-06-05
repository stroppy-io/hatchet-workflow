package execution

import (
	"context"
	"errors"
	"time"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RunPersistenceStore interface {
	RunRecord(ctx context.Context, runID string) (*models.TestRunRecord, error)
	SaveRunRecord(ctx context.Context, run *models.TestRunRecord) error
	SuiteRun(ctx context.Context, suiteRunID string) (*models.SuiteRunRecord, error)
	SaveSuiteRun(ctx context.Context, suiteRun *models.SuiteRunRecord) error
	SuiteRecord(ctx context.Context, tenantID, suiteID string) (*models.SuiteRecord, error)
	SaveSuiteRecord(ctx context.Context, suite *models.SuiteRecord) error
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

	if err := a.store.SaveRunRecord(ctx, rec); err != nil {
		return err
	}
	if rec.GetSuiteRunId() != "" {
		return a.PersistSuiteRun(ctx, rec.GetSuiteRunId(), commonpb.Status_STATUS_UNSPECIFIED)
	}
	return nil
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
	if err := a.store.SaveRunRecord(ctx, rec); err != nil {
		return err
	}
	if rec.GetSuiteRunId() != "" {
		return a.PersistSuiteRun(ctx, rec.GetSuiteRunId(), commonpb.Status_STATUS_UNSPECIFIED)
	}
	return nil
}

func (a *RunPersistenceActivities) AppendRunLogs(ctx context.Context, lines []*monitor.LogLine) error {
	if a == nil || a.logs == nil {
		return nil
	}
	return a.logs.Write(ctx, lines)
}

func (a *RunPersistenceActivities) PersistSuiteRun(ctx context.Context, suiteRunID string, statusHint commonpb.Status) error {
	if a == nil || a.store == nil || suiteRunID == "" {
		return nil
	}
	rec, err := a.store.SuiteRun(ctx, suiteRunID)
	if err != nil {
		return err
	}
	if rec.Summary == nil {
		rec.Summary = &models.SuiteRunRecord_Summary{}
	}

	now := time.Now()
	if err := a.cancelPendingSuiteChildren(ctx, rec, statusHint, now); err != nil {
		return err
	}

	var completed, failed, running, pending uint32
	for _, child := range rec.GetChildren() {
		childRec, err := a.store.RunRecord(ctx, child.GetTestRunId())
		if err != nil {
			return err
		}
		child.Status = childRec.GetStatus()
		switch child.GetStatus() {
		case commonpb.Status_STATUS_COMPLETED:
			completed++
		case commonpb.Status_STATUS_FAILED, commonpb.Status_STATUS_CANCELLED, commonpb.Status_STATUS_SKIPPED:
			failed++
		case commonpb.Status_STATUS_RUNNING,
			commonpb.Status_STATUS_CANCELLING,
			commonpb.Status_STATUS_ALLOCATED,
			commonpb.Status_STATUS_DEPLOYMENT,
			commonpb.Status_STATUS_DEPLOYED:
			running++
		default:
			pending++
		}
	}

	rec.Status = nextRunStatus(rec.GetStatus(), suiteStatus(statusHint, completed, failed, running, pending, uint32(len(rec.GetChildren()))))
	rec.Summary.Total = uint32(len(rec.GetChildren()))
	rec.Summary.Completed = completed
	rec.Summary.Failed = failed
	rec.Summary.Running = running
	rec.Summary.Pending = pending
	rec.Summary.ProgressPct = progressPct(int(completed+failed), len(rec.GetChildren()))
	if rec.GetStatus() != commonpb.Status_STATUS_PENDING && rec.Summary.StartedAt == nil {
		rec.Summary.StartedAt = timestamppb.New(now)
	}
	if isTerminalStatus(rec.GetStatus()) {
		if rec.Summary.FinishedAt == nil {
			rec.Summary.FinishedAt = timestamppb.New(now)
		}
	}
	rec.Summary.Duration = suiteDuration(rec.Summary.GetStartedAt(), rec.Summary.GetFinishedAt(), now)
	touchRecordUpdated(rec.GetEntity(), now)

	if err := a.store.SaveSuiteRun(ctx, rec); err != nil {
		return err
	}
	return a.updateSuiteDefinitionSummary(ctx, rec, now)
}

func (a *RunPersistenceActivities) cancelPendingSuiteChildren(ctx context.Context, rec *models.SuiteRunRecord, statusHint commonpb.Status, now time.Time) error {
	if statusHint != commonpb.Status_STATUS_FAILED && statusHint != commonpb.Status_STATUS_CANCELLED {
		return nil
	}
	for _, child := range rec.GetChildren() {
		childRec, err := a.store.RunRecord(ctx, child.GetTestRunId())
		if err != nil {
			return err
		}
		if childRec.GetStatus() != commonpb.Status_STATUS_PENDING {
			continue
		}
		childRec.Status = commonpb.Status_STATUS_CANCELLED
		if childRec.Summary == nil {
			childRec.Summary = &models.TestRunRecord_Summary{}
		}
		if childRec.Summary.FinishedAt == nil {
			childRec.Summary.FinishedAt = timestamppb.New(now)
		}
		childRec.Summary.Duration = durationpb.New(0)
		touchRecordUpdated(childRec.GetEntity(), now)
		if err := a.store.SaveRunRecord(ctx, childRec); err != nil {
			return err
		}
	}
	return nil
}

func (a *RunPersistenceActivities) updateSuiteDefinitionSummary(ctx context.Context, run *models.SuiteRunRecord, now time.Time) error {
	if run.GetSuiteId() == "" {
		return nil
	}
	suite, err := a.store.SuiteRecord(ctx, run.GetEntity().GetTenantId(), run.GetSuiteId())
	if errors.Is(err, derrors.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if suite.Summary == nil {
		suite.Summary = &models.SuiteRecord_Summary{}
	}

	runAt := run.GetSummary().GetStartedAt()
	if runAt == nil {
		runAt = run.GetEntity().GetTimings().GetCreatedAt()
	}
	if runAt == nil {
		runAt = timestamppb.New(now)
	}
	if last := suite.GetSummary().GetLastRunAt(); last != nil && runAt.AsTime().Before(last.AsTime()) {
		return nil
	}
	suite.Summary.LastRunAt = runAt
	suite.Summary.LastRunStatus = run.GetStatus()
	if suite.Entity != nil {
		touchRecordUpdated(suite.GetEntity(), now)
	}
	return a.store.SaveSuiteRecord(ctx, suite)
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

func suiteStatus(statusHint commonpb.Status, completed, failed, running, pending, total uint32) commonpb.Status {
	if statusHint != commonpb.Status_STATUS_UNSPECIFIED {
		return statusHint
	}
	switch {
	case total == 0:
		return commonpb.Status_STATUS_PENDING
	case running > 0:
		return commonpb.Status_STATUS_RUNNING
	case pending > 0 && completed+failed > 0:
		return commonpb.Status_STATUS_RUNNING
	case pending > 0:
		return commonpb.Status_STATUS_PENDING
	case failed > 0:
		return commonpb.Status_STATUS_FAILED
	default:
		return commonpb.Status_STATUS_COMPLETED
	}
}

func suiteDuration(started, finished *timestamppb.Timestamp, now time.Time) *durationpb.Duration {
	if started == nil {
		return nil
	}
	end := now
	if finished != nil {
		end = finished.AsTime()
	}
	d := end.Sub(started.AsTime())
	if d < 0 {
		d = 0
	}
	return durationpb.New(d)
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
