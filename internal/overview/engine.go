// Package overview builds the run Overview projection by reading a Temporal
// TestWorkflow's live state via the generated GetRunState query and mapping the
// returned workflow.RunState onto a monitor.Overview snapshot.
//
// It implements the test_run_overview.OverviewReader port: Get returns a
// one-shot snapshot; Stream pushes a fresh full snapshot every second until the
// run is terminal or the context is cancelled.
package overview

import (
	"context"
	"errors"
	"math"
	"time"

	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_run_overview"
)

// Engine implements the test_run_overview.OverviewReader port.
var _ test_run_overview.OverviewReader = (*Engine)(nil)

// streamInterval is how often Stream re-polls GetRunState and emits a snapshot.
const streamInterval = time.Second

// runStateQuerier is the minimal slice of workflow.TestServiceClient the engine
// depends on. Both the real generated client and test stubs satisfy it, so the
// engine can be unit-tested without a live Temporal server.
type runStateQuerier interface {
	GetRunState(ctx context.Context, workflowID string, runID string) (*workflow.RunState, error)
}

// Engine projects a Temporal TestWorkflow's live RunState onto monitor.Overview.
type Engine struct {
	tc runStateQuerier
}

// New builds an Engine over a Temporal client, wiring the generated
// TestServiceClient as the query source.
func New(c client.Client) *Engine {
	return &Engine{tc: workflow.NewTestServiceClient(c)}
}

// newEngine constructs an Engine over an arbitrary querier (used by tests).
func newEngine(q runStateQuerier) *Engine {
	return &Engine{tc: q}
}

// workflowID derives the deterministic TestWorkflow id for a run from its run id
// (matches the proto id expression "test-run/${! test_run.id }").
func workflowID(runID string) string {
	return "test-run/" + runID
}

// Get returns a one-shot Overview snapshot for the run.
//
// If the workflow has not started yet (or the query otherwise fails), it returns
// a PENDING Overview rather than an error, so the UI can poll before the
// workflow exists.
func (e *Engine) Get(ctx context.Context, runID string) (*monitor.Overview, error) {
	rs, err := e.tc.GetRunState(ctx, workflowID(runID), "")
	if err != nil {
		// Honour context cancellation, but otherwise degrade to PENDING: the
		// workflow may simply not be running yet.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return pendingOverview(runID), nil
	}
	return projectOverview(runID, rs), nil
}

// Stream returns a channel of full Overview snapshots, refreshed every second.
// The first snapshot is sent immediately. The channel is closed when ctx is
// cancelled or the run reaches a terminal state.
func (e *Engine) Stream(ctx context.Context, runID string) (<-chan *monitor.Overview, error) {
	out := make(chan *monitor.Overview)
	go func() {
		defer close(out)

		send := func(ov *monitor.Overview) bool {
			select {
			case <-ctx.Done():
				return false
			case out <- ov:
				return true
			}
		}

		// First snapshot immediately.
		ov, err := e.Get(ctx, runID)
		if err != nil {
			return // ctx cancelled
		}
		if !send(ov) {
			return
		}
		if isTerminal(ov.GetStatus()) {
			return
		}

		ticker := time.NewTicker(streamInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ov, err := e.Get(ctx, runID)
				if err != nil {
					return
				}
				if !send(ov) {
					return
				}
				if isTerminal(ov.GetStatus()) {
					return
				}
			}
		}
	}()
	return out, nil
}

// pendingOverview is the snapshot returned before the workflow exists: just the
// run id, a PENDING status and an empty pipeline.
func pendingOverview(runID string) *monitor.Overview {
	return &monitor.Overview{
		RunId:    runID,
		Status:   common.Status_STATUS_PENDING,
		Pipeline: &monitor.PipelineView{},
		Workers:  []*monitor.WorkerInfo{},
		Timeline: []*monitor.Event{},
	}
}

// projectOverview maps a live RunState onto a full Overview snapshot.
func projectOverview(runID string, rs *workflow.RunState) *monitor.Overview {
	now := time.Now()
	terminal := isTerminal(rs.GetStatus())

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
		if isTerminal(st.GetStatus()) {
			completed++
		}

		// Synthesize timeline events from the stage timings.
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

	// One RUN_STATUS event for the overall run status.
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

	// finished_at is only meaningful once the run itself is terminal.
	var finishedAt *timestamppb.Timestamp
	if terminal {
		finishedAt = latestFinish
	}

	ov := &monitor.Overview{
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
	_ = domain.Worker_KIND_MASTER // workers left empty for the demo; MASTER kind available if needed
	return ov
}

// spanDuration computes the elapsed time of a span: finished-started when the
// span is closed, else now-started while still running. Returns nil when there
// is no start time.
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

// progressPct is round(completed/total*100), 0 when there are no stages.
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

// stageEndKind maps a stage's terminal status to the timeline event kind for its
// completion.
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

// isTerminal reports whether a run/stage status is final.
func isTerminal(s common.Status) bool {
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

// refString returns a pointer to s, or nil for the empty string (so an unset
// node id is left absent on the LogRef oneof).
func refString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// isNotFound reports whether err indicates the workflow does not exist / has not
// started. Kept for callers that want to distinguish not-found from other query
// failures; Get currently degrades all non-context errors to PENDING.
func isNotFound(err error) bool {
	var nf *serviceerror.NotFound
	return errors.As(err, &nf)
}
