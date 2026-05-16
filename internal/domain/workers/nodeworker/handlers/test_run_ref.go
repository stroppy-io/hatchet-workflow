// TestRunRefHandler is a SKELETON. Full implementation requires the testing
// service (creates an import cycle with system package). It returns an error
// to make the skeleton status visible at runtime.
package handlers

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	taskspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks"
)

// TestRunRefHandler spawns a child DagRun for a TestRunRefTask (skeleton).
// Full implementation would call testRunSvc.LaunchTestRun and poll for completion
// without creating an import cycle.
type TestRunRefHandler struct{}

func NewTestRunRefHandler() *TestRunRefHandler {
	return &TestRunRefHandler{}
}

func (h *TestRunRefHandler) Kind() string { return "TestRunRefTask" }

func (h *TestRunRefHandler) Execute(ctx context.Context, node *systempb.NodeRun, spec *anypb.Any, state nodeworker.StateStore) (*anypb.Any, error) {
	var task taskspb.TestRunRefTask
	if err := anypb.UnmarshalTo(spec, &task, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("TestRunRefHandler: unmarshal: %w", err)
	}
	testRunID := task.GetTestRunId().GetValue()
	return nil, fmt.Errorf("TestRunRefHandler: not fully implemented (skeleton) — test_run_id=%s", testRunID)
}
