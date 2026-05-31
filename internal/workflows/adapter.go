package workflows

import (
	"context"

	"go.temporal.io/sdk/client"

	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	gen "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_run"
)

// Launcher adapts the generated TestService client to the test_run service's
// Workflows port: StartTestRun (after committing the record) launches the
// TestWorkflow; cancellation requests the run by its deterministic workflow id.
type Launcher struct {
	c gen.TestServiceClient
}

var _ test_run.Workflows = (*Launcher)(nil)

// NewLauncher builds a Launcher over a Temporal client.
func NewLauncher(cl client.Client) *Launcher {
	return &Launcher{c: gen.NewTestServiceClient(cl)}
}

// testRunWorkflowID is the deterministic workflow id for a run. It mirrors the
// generated id expression ("test-run/<id>"); we set it explicitly so the id is
// stable and matches CancelTest below.
func testRunWorkflowID(runID string) string { return "test-run/" + runID }

// LaunchTest starts the TestWorkflow for the persisted run, keyed on the
// deterministic per-run workflow id.
func (l *Launcher) LaunchTest(ctx context.Context, run *models.TestRunRecord) error {
	spec := run.GetSpec()
	opts := gen.NewTestWorkflowOptions().WithID(testRunWorkflowID(spec.GetId()))
	_, err := l.c.TestWorkflowAsync(ctx, &gen.TestWorkflowRequest{TestRun: spec}, opts)
	return err
}

// CancelTest requests cancellation of the run's TestWorkflow by its
// deterministic workflow id.
func (l *Launcher) CancelTest(ctx context.Context, runID string) error {
	return l.c.CancelWorkflow(ctx, testRunWorkflowID(runID), "")
}
