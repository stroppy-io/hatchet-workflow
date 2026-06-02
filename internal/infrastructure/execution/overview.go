package execution

import (
	"context"
	"math"
	"time"

	"go.temporal.io/sdk/client"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_run_overview"
)

// streamInterval is how often Stream re-polls GetRunState and emits a snapshot.
const streamInterval = time.Second

// runStateQuerier is the minimal slice of workflowpb.TestServiceClient the reader
// depends on for the live run state. Kept narrow so the reader is unit-testable
// without a live Temporal server.
type runStateQuerier interface {
	GetRunState(ctx context.Context, workflowID string, runID string) (*workflowpb.RunState, error)
}

// SnapshotRunReader loads the persisted run record + parent suite run that the
// snapshot bundles alongside the live workflow projection. It is a read-only,
// tenant-agnostic lookup by id (the service has already authorised the caller).
// Implemented by the storage layer; injected so the overview reader stays an
// execution adapter without owning storage.
type SnapshotRunReader interface {
	// RunRecord returns the persisted TestRunRecord for the run id.
	RunRecord(ctx context.Context, runID string) (*models.TestRunRecord, error)
	// SuiteRun returns the parent SuiteRunRecord when the run belongs to a suite,
	// or (nil, nil) when it is a standalone run.
	SuiteRun(ctx context.Context, suiteRunID string) (*models.SuiteRunRecord, error)
}

// OverviewReader implements test_run_overview.OverviewReader. It assembles the
// snapshot from (a) the persisted run record and its staged topology, (b) the
// live monitor.Overview projected from the Temporal TestWorkflow's RunState, and
// (c) the parent suite-run record when the run belongs to a suite.
type OverviewReader struct {
	tc    runStateQuerier
	store SnapshotRunReader
}

var _ test_run_overview.OverviewReader = (*OverviewReader)(nil)

// NewOverviewReader builds the test_run_overview.OverviewReader adapter over a
// Temporal client (the live RunState source) and a SnapshotRunReader (the
// persisted run/suite-run + topology source). store may be nil, in which case
// the snapshot carries only the live Overview projection.
func NewOverviewReader(c client.Client, store SnapshotRunReader) *OverviewReader {
	return &OverviewReader{tc: workflowpb.NewTestServiceClient(c), store: store}
}

// Get returns a one-shot snapshot for the run. The live Overview degrades to
// PENDING when the workflow has not started yet (the UI polls before it exists);
// only context cancellation is surfaced as an error.
func (r *OverviewReader) Get(ctx context.Context, runID string) (*api.TestRunOverviewSnapshot, error) {
	snap := &api.TestRunOverviewSnapshot{}

	if r.store != nil {
		rec, err := r.store.RunRecord(ctx, runID)
		if err != nil {
			return nil, err
		}
		snap.Run = rec
		snap.Topology = topologyFromRecord(rec)
		if sid := rec.GetSuiteRunId(); sid != "" {
			suiteRun, err := r.store.SuiteRun(ctx, sid)
			if err != nil {
				return nil, err
			}
			snap.SuiteRun = suiteRun
		}
	}

	rs, err := r.tc.GetRunState(ctx, testWorkflowID(runID), "")
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		// The workflow may simply not be running yet: degrade to a PENDING
		// Overview rather than fail the whole snapshot.
		snap.Overview = pendingOverview(runID)
		return snap, nil
	}
	snap.Overview = projectOverview(runID, rs)
	return snap, nil
}

// Stream pushes a fresh full snapshot every tick until ctx is cancelled or the
// run reaches a terminal state, then closes the channel.
func (r *OverviewReader) Stream(ctx context.Context, runID string) (<-chan *api.TestRunOverviewSnapshot, error) {
	out := make(chan *api.TestRunOverviewSnapshot)
	go func() {
		defer close(out)

		send := func(s *api.TestRunOverviewSnapshot) bool {
			select {
			case <-ctx.Done():
				return false
			case out <- s:
				return true
			}
		}

		emit := func() (terminal bool) {
			snap, err := r.Get(ctx, runID)
			if err != nil {
				return true // ctx cancelled or hard store error: stop
			}
			if !send(snap) {
				return true
			}
			return isTerminalStatus(snap.GetOverview().GetStatus())
		}

		if emit() {
			return
		}

		ticker := time.NewTicker(streamInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if emit() {
					return
				}
			}
		}
	}()
	return out, nil
}

// topologyFromRecord assembles the staged topology envelope from a run record:
// the baked spec's topology spec + infrastructure plan, plus the live
// infrastructure state and deployment plan filled on the record as the run
// progresses.
func topologyFromRecord(rec *models.TestRunRecord) *topology.Topology {
	if rec == nil {
		return nil
	}
	spec := rec.GetSpec()
	t := &topology.Topology{
		Spec:                spec.GetTopologySpec(),
		InfrastructurePlan:  spec.GetInfrastructurePlan(),
		InfrastructureState: rec.GetInfrastructureState(),
		DeploymentPlan:      rec.GetDeploymentPlan(),
		Tags:                spec.GetTags(),
	}
	t.State = topologyState(rec)
	return t
}

// topologyState classifies how far the topology has materialized from the
// record's filled artifacts.
func topologyState(rec *models.TestRunRecord) topology.Topology_State {
	switch {
	case rec.GetDeploymentPlan() != nil:
		return topology.Topology_STATE_DEPLOYED
	case rec.GetInfrastructureState() != nil:
		return topology.Topology_STATE_INFRASTRUCTURE_DEPLOYED
	case rec.GetSpec().GetInfrastructurePlan() != nil:
		return topology.Topology_STATE_INFRASTRUCTURE_PLANNED
	case rec.GetSpec().GetTopologySpec() != nil:
		return topology.Topology_STATE_SPEC
	default:
		return topology.Topology_STATE_UNSPECIFIED
	}
}

// pendingOverview is the snapshot returned before the workflow exists: just the
// run id, a PENDING status and empty collections.
func pendingOverview(runID string) *monitor.Overview {
	return &monitor.Overview{
		RunId:    runID,
		Status:   common.Status_STATUS_PENDING,
		Pipeline: &monitor.PipelineView{},
		Workers:  []*monitor.WorkerInfo{},
		Timeline: []*monitor.Event{},
	}
}

// projectOverview maps a live RunState onto a full Overview snapshot: one
// pipeline node + timeline events per stage, overall progress and timing.
func projectOverview(runID string, rs *workflowpb.RunState) *monitor.Overview {
	now := time.Now()
	terminal := isTerminalStatus(rs.GetStatus())

	roots := make([]*monitor.PipelineNode, 0, len(rs.GetStages()))
	timeline := make([]*monitor.Event, 0, len(rs.GetStages())+1)

	var (
		earliestStart *timestamppb.Timestamp
		latestFinish  *timestamppb.Timestamp
		completed     int
	)

	for _, st := range rs.GetStages() {
		started := st.GetStartedAt()
		finished := st.GetFinishedAt()

		roots = append(roots, &monitor.PipelineNode{
			NodeExecutionId: st.GetNodeExecutionId(),
			Name:            st.GetName(),
			Status:          st.GetStatus(),
			StartedAt:       started,
			FinishedAt:      finished,
			Duration:        spanDuration(started, finished, now),
			LogRef: &monitor.LogRef{
				RunId:           runID,
				NodeExecutionId: refString(st.GetNodeExecutionId()),
			},
			Attempt: st.GetAttempt(),
		})

		if started != nil && (earliestStart == nil || started.AsTime().Before(earliestStart.AsTime())) {
			earliestStart = started
		}
		if finished != nil && (latestFinish == nil || finished.AsTime().After(latestFinish.AsTime())) {
			latestFinish = finished
		}
		if isTerminalStatus(st.GetStatus()) {
			completed++
		}

		if started != nil {
			timeline = append(timeline, &monitor.Event{
				At:              started,
				Kind:            monitor.Event_KIND_STAGE_STARTED,
				Message:         "stage started: " + st.GetName(),
				NodeExecutionId: st.GetNodeExecutionId(),
				Status:          common.Status_STATUS_RUNNING,
			})
		}
		if finished != nil {
			timeline = append(timeline, &monitor.Event{
				At:              finished,
				Kind:            stageEndKind(st.GetStatus()),
				Message:         "stage finished: " + st.GetName(),
				NodeExecutionId: st.GetNodeExecutionId(),
				Status:          st.GetStatus(),
			})
		}
	}

	runAt := earliestStart
	if runAt == nil {
		runAt = timestamppb.New(now)
	}
	timeline = append(timeline, &monitor.Event{
		At:      runAt,
		Kind:    monitor.Event_KIND_RUN_STATUS,
		Message: "run status: " + rs.GetStatus().String(),
		Status:  rs.GetStatus(),
	})

	var finishedAt *timestamppb.Timestamp
	if terminal {
		finishedAt = latestFinish
	}

	return &monitor.Overview{
		RunId:       runID,
		Status:      rs.GetStatus(),
		StartedAt:   earliestStart,
		FinishedAt:  finishedAt,
		Duration:    spanDuration(earliestStart, finishedAt, now),
		ProgressPct: progressPct(completed, len(rs.GetStages())),
		Pipeline:    &monitor.PipelineView{Roots: roots},
		Workers:     []*monitor.WorkerInfo{},
		Timeline:    timeline,
	}
}

// spanDuration computes a span's elapsed time: finished-started when closed, else
// now-started while running. Returns nil with no start.
func spanDuration(start, finish *timestamppb.Timestamp, now time.Time) *durationpb.Duration {
	if start == nil {
		return nil
	}
	end := now
	if finish != nil {
		end = finish.AsTime()
	}
	d := end.Sub(start.AsTime())
	if d < 0 {
		d = 0
	}
	return durationpb.New(d)
}

// progressPct is round(completed/total*100), clamped to [0,100], 0 with no stages.
func progressPct(completed, total int) uint32 {
	if total <= 0 {
		return 0
	}
	pct := math.Round(float64(completed) / float64(total) * 100)
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return uint32(pct)
}

// stageEndKind maps a stage's terminal status to its completion event kind.
func stageEndKind(s common.Status) monitor.Event_Kind {
	switch s {
	case common.Status_STATUS_COMPLETED:
		return monitor.Event_KIND_STAGE_COMPLETED
	case common.Status_STATUS_FAILED:
		return monitor.Event_KIND_STAGE_FAILED
	case common.Status_STATUS_RETRY_WAIT:
		return monitor.Event_KIND_STAGE_RETRYING
	default:
		return monitor.Event_KIND_STAGE_COMPLETED
	}
}

// isTerminalStatus reports whether a run/stage status is final.
func isTerminalStatus(s common.Status) bool {
	switch s {
	case common.Status_STATUS_COMPLETED,
		common.Status_STATUS_FAILED,
		common.Status_STATUS_CANCELLED,
		common.Status_STATUS_SKIPPED:
		return true
	default:
		return false
	}
}

// refString returns a pointer to s, or nil for the empty string (leaving an unset
// node id absent on the LogRef oneof).
func refString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
