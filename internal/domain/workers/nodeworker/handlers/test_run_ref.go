package handlers

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	taskspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
)

// TestRunRefPort is the surface TestRunRefHandler needs from the testing
// service. Defined here to avoid an import cycle (testing → nodeworker).
type TestRunRefPort interface {
	LaunchTestRun(ctx context.Context, id *testingpb.TestRunId) (*testingpb.TestRun, error)
	GetTestRun(ctx context.Context, id *testingpb.TestRunId) (*testingpb.TestRun, error)
}

// DagRunPort is the system-engine surface for polling child run status.
type DagRunPort interface {
	GetDagRun(ctx context.Context, id *systempb.DagRunId) (*systempb.DagRun, error)
}

// TestRunRefHandler executes a TestRunRefTask: launches the referenced
// TestRun (idempotent — if a DagRun is already bound, just polls) and waits
// for the child DagRun to reach a terminal state. Propagates child failure
// as handler error.
type TestRunRefHandler struct {
	runs    TestRunRefPort
	engine  DagRunPort
	poll    time.Duration
	maxWait time.Duration
}

// NewTestRunRefHandler constructs a handler. poll = how often to query child
// DagRun status; maxWait caps the total wait per child invocation (cancel
// propagates if the suite is cancelled via ctx anyway).
func NewTestRunRefHandler(runs TestRunRefPort, engine DagRunPort, poll, maxWait time.Duration) *TestRunRefHandler {
	if poll == 0 {
		poll = 2 * time.Second
	}
	if maxWait == 0 {
		maxWait = 12 * time.Hour
	}
	return &TestRunRefHandler{runs: runs, engine: engine, poll: poll, maxWait: maxWait}
}

func (h *TestRunRefHandler) Kind() string { return "TestRunRefTask" }

func (h *TestRunRefHandler) Execute(ctx context.Context, _ *systempb.NodeRun, spec *anypb.Any, _ nodeworker.StateStore) (*anypb.Any, error) {
	var task taskspb.TestRunRefTask
	if err := anypb.UnmarshalTo(spec, &task, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("TestRunRefHandler: unmarshal: %w", err)
	}
	id := task.GetTestRunId()
	if id == nil || id.GetValue() == "" {
		return nil, fmt.Errorf("TestRunRefHandler: empty test_run_id")
	}

	// Ensure the child is launched. LaunchTestRun is idempotent on
	// (test_run_id, attempt) so calling it on an already-running child is a
	// no-op and returns the existing DagRunId.
	tr, err := h.runs.LaunchTestRun(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("TestRunRefHandler: launch %s: %w", id.GetValue(), err)
	}
	dagRunID := tr.GetDagRunId()
	if dagRunID == nil || dagRunID.GetValue() == "" {
		return nil, fmt.Errorf("TestRunRefHandler: launch returned no dag_run_id for %s", id.GetValue())
	}

	deadline := time.Now().Add(h.maxWait)
	ticker := time.NewTicker(h.poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("TestRunRefHandler: child run %s exceeded max wait %s", id.GetValue(), h.maxWait)
			}
			dr, err := h.engine.GetDagRun(ctx, dagRunID)
			if err != nil {
				return nil, fmt.Errorf("TestRunRefHandler: poll child %s: %w", dagRunID.GetValue(), err)
			}
			switch dr.GetStatus() {
			case systempb.DagRunStatus_DAG_RUN_STATUS_SUCCEEDED:
				return nil, nil
			case systempb.DagRunStatus_DAG_RUN_STATUS_FAILED,
				systempb.DagRunStatus_DAG_RUN_STATUS_CANCELLED:
				return nil, fmt.Errorf("TestRunRefHandler: child run %s ended in status %s", id.GetValue(), dr.GetStatus().String())
			}
		}
	}
}
