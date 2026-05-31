package overview

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

// stubQuerier is a fake runStateQuerier that returns a canned RunState (or error).
type stubQuerier struct {
	rs        *workflow.RunState
	err       error
	gotWfID   string
	gotRunID  string
	callCount int
}

func (s *stubQuerier) GetRunState(_ context.Context, workflowID, runID string) (*workflow.RunState, error) {
	s.callCount++
	s.gotWfID = workflowID
	s.gotRunID = runID
	if s.err != nil {
		return nil, s.err
	}
	return s.rs, nil
}

func ts(t time.Time) *timestamppb.Timestamp { return timestamppb.New(t) }

func TestGet_MapsStagesToPipelineNodes(t *testing.T) {
	start := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Second)

	rs := &workflow.RunState{
		Status: common.Status_STATUS_RUNNING,
		Stages: []*workflow.Stage{
			{
				NodeExecutionId: "node-1",
				Name:            "install-db",
				Status:          common.Status_STATUS_COMPLETED,
				StartedAt:       ts(start),
				FinishedAt:      ts(end),
				Attempt:         2,
			},
			{
				NodeExecutionId: "node-2",
				Name:            "run-workload",
				Status:          common.Status_STATUS_RUNNING,
				StartedAt:       ts(end),
				Attempt:         1,
			},
		},
	}
	stub := &stubQuerier{rs: rs}
	e := newEngine(stub)

	ov, err := e.Get(context.Background(), "run-abc")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	if stub.gotWfID != "test-run/run-abc" {
		t.Errorf("workflow id = %q, want %q", stub.gotWfID, "test-run/run-abc")
	}
	if stub.gotRunID != "" {
		t.Errorf("query runID = %q, want empty (latest)", stub.gotRunID)
	}
	if ov.GetRunId() != "run-abc" {
		t.Errorf("RunId = %q, want run-abc", ov.GetRunId())
	}
	if ov.GetStatus() != common.Status_STATUS_RUNNING {
		t.Errorf("Status = %v, want RUNNING", ov.GetStatus())
	}

	roots := ov.GetPipeline().GetRoots()
	if len(roots) != 2 {
		t.Fatalf("got %d pipeline nodes, want 2", len(roots))
	}

	n1 := roots[0]
	if n1.GetNodeExecutionId() != "node-1" || n1.GetName() != "install-db" {
		t.Errorf("node1 id/name = %q/%q", n1.GetNodeExecutionId(), n1.GetName())
	}
	if n1.GetStatus() != common.Status_STATUS_COMPLETED {
		t.Errorf("node1 status = %v, want COMPLETED", n1.GetStatus())
	}
	if n1.GetAttempt() != 2 {
		t.Errorf("node1 attempt = %d, want 2", n1.GetAttempt())
	}
	if got := n1.GetDuration().AsDuration(); got != 30*time.Second {
		t.Errorf("node1 duration = %v, want 30s", got)
	}
	if n1.GetLogRef() == nil || n1.GetLogRef().GetRunId() != "run-abc" || n1.GetLogRef().GetNodeExecutionId() != "node-1" {
		t.Errorf("node1 log_ref = %+v", n1.GetLogRef())
	}

	// node-2 is still running: finished_at unset, duration = now-started (>0).
	n2 := roots[1]
	if n2.GetFinishedAt() != nil {
		t.Errorf("node2 finished_at should be nil while running")
	}
	if n2.GetDuration() == nil || n2.GetDuration().AsDuration() <= 0 {
		t.Errorf("node2 duration should be positive (now-started), got %v", n2.GetDuration())
	}

	// progress: 1 of 2 terminal -> 50.
	if ov.GetProgressPct() != 50 {
		t.Errorf("progress_pct = %d, want 50", ov.GetProgressPct())
	}

	// started_at = earliest stage start.
	if !ov.GetStartedAt().AsTime().Equal(start) {
		t.Errorf("started_at = %v, want %v", ov.GetStartedAt().AsTime(), start)
	}
	// Run not terminal -> finished_at unset.
	if ov.GetFinishedAt() != nil {
		t.Errorf("finished_at should be nil for non-terminal run")
	}
}

func TestGet_TerminalRunSetsFinishedAt(t *testing.T) {
	start := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	mid := start.Add(20 * time.Second)
	end := start.Add(60 * time.Second)

	rs := &workflow.RunState{
		Status: common.Status_STATUS_COMPLETED,
		Stages: []*workflow.Stage{
			{NodeExecutionId: "a", Name: "a", Status: common.Status_STATUS_COMPLETED, StartedAt: ts(start), FinishedAt: ts(mid)},
			{NodeExecutionId: "b", Name: "b", Status: common.Status_STATUS_COMPLETED, StartedAt: ts(mid), FinishedAt: ts(end)},
		},
	}
	e := newEngine(&stubQuerier{rs: rs})

	ov, err := e.Get(context.Background(), "r1")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if ov.GetProgressPct() != 100 {
		t.Errorf("progress_pct = %d, want 100", ov.GetProgressPct())
	}
	if ov.GetFinishedAt() == nil || !ov.GetFinishedAt().AsTime().Equal(end) {
		t.Errorf("finished_at = %v, want latest %v", ov.GetFinishedAt(), end)
	}
	if got := ov.GetDuration().AsDuration(); got != 60*time.Second {
		t.Errorf("duration = %v, want 60s", got)
	}

	// Timeline: 2 started + 2 finished + 1 run-status = 5.
	if len(ov.GetTimeline()) != 5 {
		t.Fatalf("timeline len = %d, want 5", len(ov.GetTimeline()))
	}
	var runStatusEvents int
	for _, ev := range ov.GetTimeline() {
		if ev.GetKind() == monitor.Event_KIND_RUN_STATUS {
			runStatusEvents++
			if ev.GetStatus() != common.Status_STATUS_COMPLETED {
				t.Errorf("run-status event status = %v", ev.GetStatus())
			}
		}
	}
	if runStatusEvents != 1 {
		t.Errorf("run-status events = %d, want 1", runStatusEvents)
	}
}

func TestGet_QueryErrorReturnsPending(t *testing.T) {
	e := newEngine(&stubQuerier{err: errors.New("workflow not found")})

	ov, err := e.Get(context.Background(), "missing")
	if err != nil {
		t.Fatalf("Get should not error on query failure, got %v", err)
	}
	if ov.GetStatus() != common.Status_STATUS_PENDING {
		t.Errorf("status = %v, want PENDING", ov.GetStatus())
	}
	if ov.GetRunId() != "missing" {
		t.Errorf("run_id = %q, want missing", ov.GetRunId())
	}
	if len(ov.GetPipeline().GetRoots()) != 0 {
		t.Errorf("pending pipeline should be empty")
	}
}

func TestGet_ContextCancelledPropagates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e := newEngine(&stubQuerier{err: context.Canceled})

	if _, err := e.Get(ctx, "r"); err == nil {
		t.Fatal("expected context error to propagate")
	}
}

func TestStream_FirstSnapshotThenTerminalCloses(t *testing.T) {
	rs := &workflow.RunState{
		Status: common.Status_STATUS_COMPLETED,
		Stages: []*workflow.Stage{
			{NodeExecutionId: "a", Name: "a", Status: common.Status_STATUS_COMPLETED, StartedAt: ts(time.Now()), FinishedAt: ts(time.Now())},
		},
	}
	e := newEngine(&stubQuerier{rs: rs})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := e.Stream(ctx, "r1")
	if err != nil {
		t.Fatalf("Stream error: %v", err)
	}

	first, ok := <-ch
	if !ok {
		t.Fatal("expected at least one snapshot")
	}
	if first.GetStatus() != common.Status_STATUS_COMPLETED {
		t.Errorf("first status = %v, want COMPLETED", first.GetStatus())
	}

	// Terminal status -> channel closes after the first snapshot.
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("channel should be closed after terminal snapshot")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("channel did not close after terminal run")
	}
}

func TestStream_ClosesOnContextCancel(t *testing.T) {
	rs := &workflow.RunState{
		Status: common.Status_STATUS_RUNNING,
		Stages: []*workflow.Stage{
			{NodeExecutionId: "a", Name: "a", Status: common.Status_STATUS_RUNNING, StartedAt: ts(time.Now())},
		},
	}
	e := newEngine(&stubQuerier{rs: rs})

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := e.Stream(ctx, "r1")
	if err != nil {
		t.Fatalf("Stream error: %v", err)
	}

	if _, ok := <-ch; !ok {
		t.Fatal("expected first snapshot")
	}
	cancel()

	select {
	case <-ch:
		// drains then closes
	case <-time.After(3 * time.Second):
		t.Fatal("channel did not close after ctx cancel")
	}
}
