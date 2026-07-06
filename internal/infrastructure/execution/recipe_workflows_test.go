package execution

import (
	"context"
	"errors"
	"testing"

	"go.temporal.io/sdk/client"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"github.com/stroppy-io/stroppy-cloud/internal/workflows"
)

func TestLaunchRecipeRunStartsWorkflowWithExpectedIDAndInput(t *testing.T) {
	starter := &fakeWorkflowStarter{}
	rw := &RecipeWorkflows{
		client:    starter,
		bootstrap: fakeStandaloneBootstrap{serverAddr: "https://control.example"},
	}
	run := &models.TestRunRecord{
		Entity: &common.Entity{Id: "run-1", TenantId: "tenant-1"},
	}
	bundle := map[string][]byte{"cluster.yaml": []byte("version: 1\n")}

	if err := rw.LaunchRecipeRun(context.Background(), run, bundle); err != nil {
		t.Fatalf("launch recipe run: %v", err)
	}
	if got, want := len(starter.calls), 1; got != want {
		t.Fatalf("ExecuteWorkflow calls = %d, want %d", got, want)
	}
	call := starter.calls[0]
	if got, want := call.options.ID, "run-recipe/run-1"; got != want {
		t.Fatalf("workflow id = %q, want %q", got, want)
	}
	if got, want := call.options.TaskQueue, "stroppy-cloud"; got != want {
		t.Fatalf("task queue = %q, want %q", got, want)
	}
	if got, want := call.workflow, workflows.RunRecipeWorkflowName; got != want {
		t.Fatalf("workflow type = %v, want %q", got, want)
	}
	if len(call.args) != 1 {
		t.Fatalf("args = %d, want 1", len(call.args))
	}
	in, ok := call.args[0].(*workflows.RunRecipeInput)
	if !ok {
		t.Fatalf("arg type = %T, want *workflows.RunRecipeInput", call.args[0])
	}
	if in.RunID != "run-1" {
		t.Fatalf("input.RunID = %q, want run-1", in.RunID)
	}
	if in.TenantID != "tenant-1" {
		t.Fatalf("input.TenantID = %q, want tenant-1", in.TenantID)
	}
	if string(in.Bundle["cluster.yaml"]) != "version: 1\n" {
		t.Fatalf("input.Bundle = %v, want the recipe bundle", in.Bundle)
	}
	if got, want := in.Bootstrap.GetServerAddr(), "https://control.example"; got != want {
		t.Fatalf("input.Bootstrap.ServerAddr = %q, want %q", got, want)
	}
}

func TestLaunchRecipeRunWithoutBootstrapSourceLeavesBootstrapNil(t *testing.T) {
	starter := &fakeWorkflowStarter{}
	rw := &RecipeWorkflows{client: starter}
	run := &models.TestRunRecord{Entity: &common.Entity{Id: "run-1", TenantId: "tenant-1"}}

	if err := rw.LaunchRecipeRun(context.Background(), run, nil); err != nil {
		t.Fatalf("launch recipe run: %v", err)
	}
	in := starter.calls[0].args[0].(*workflows.RunRecipeInput) //nolint:forcetypeassert // test-only
	if in.Bootstrap != nil {
		t.Fatalf("bootstrap = %+v, want nil (no bootstrap source configured)", in.Bootstrap)
	}
}

func TestLaunchRecipeRunPropagatesExecuteWorkflowError(t *testing.T) {
	wantErr := errors.New("temporal unavailable")
	starter := &fakeWorkflowStarter{err: wantErr}
	rw := &RecipeWorkflows{client: starter}
	run := &models.TestRunRecord{Entity: &common.Entity{Id: "run-1", TenantId: "tenant-1"}}

	err := rw.LaunchRecipeRun(context.Background(), run, nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

func TestLaunchRecipeRunMissingRunIDErrors(t *testing.T) {
	starter := &fakeWorkflowStarter{}
	rw := &RecipeWorkflows{client: starter}
	run := &models.TestRunRecord{Entity: &common.Entity{TenantId: "tenant-1"}}

	err := rw.LaunchRecipeRun(context.Background(), run, nil)
	if !errors.Is(err, errRecipeRunMissingID) {
		t.Fatalf("err = %v, want errRecipeRunMissingID", err)
	}
	if len(starter.calls) != 0 {
		t.Fatal("ExecuteWorkflow must not be called for a run with no id")
	}
}

func TestCancelRecipeRunCancelsTheDeterministicWorkflowID(t *testing.T) {
	starter := &fakeWorkflowStarter{}
	rw := &RecipeWorkflows{client: starter}

	if err := rw.CancelRecipeRun(context.Background(), "run-1"); err != nil {
		t.Fatalf("cancel recipe run: %v", err)
	}
	if got, want := len(starter.cancelCalls), 1; got != want {
		t.Fatalf("CancelWorkflow calls = %d, want %d", got, want)
	}
	call := starter.cancelCalls[0]
	if got, want := call.workflowID, "run-recipe/run-1"; got != want {
		t.Fatalf("workflow id = %q, want %q", got, want)
	}
	if call.runID != "" {
		t.Fatalf("temporal run id = %q, want empty (target latest execution)", call.runID)
	}
}

func TestCancelRecipeRunPropagatesCancelWorkflowError(t *testing.T) {
	wantErr := errors.New("temporal unavailable")
	starter := &fakeWorkflowStarter{cancelErr: wantErr}
	rw := &RecipeWorkflows{client: starter}

	err := rw.CancelRecipeRun(context.Background(), "run-1")
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

/*
	===== test doubles =====
*/

type executeWorkflowCall struct {
	options  client.StartWorkflowOptions
	workflow interface{}
	args     []interface{}
}

type cancelWorkflowCall struct {
	workflowID string
	runID      string
}

type fakeWorkflowStarter struct {
	err         error
	cancelErr   error
	calls       []executeWorkflowCall
	cancelCalls []cancelWorkflowCall
}

func (f *fakeWorkflowStarter) ExecuteWorkflow(_ context.Context, options client.StartWorkflowOptions, workflow interface{}, args ...interface{}) (client.WorkflowRun, error) {
	f.calls = append(f.calls, executeWorkflowCall{options: options, workflow: workflow, args: args})
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}

func (f *fakeWorkflowStarter) CancelWorkflow(_ context.Context, workflowID string, runID string) error {
	f.cancelCalls = append(f.cancelCalls, cancelWorkflowCall{workflowID: workflowID, runID: runID})
	return f.cancelErr
}

// fakeStandaloneBootstrap is a stub domsettings.AgentBootstrapSource-shaped
// source (the port's actual name lives in the domain/settings package;
// RecipeWorkflows only needs the AgentBootstrap method). Extracted (verbatim)
// from the deleted test_workflows_test.go, whose TestWorkflows-only test was
// removed along with test_workflows.go; RecipeWorkflows still needs the double.
type fakeStandaloneBootstrap struct {
	serverAddr string
}

func (f fakeStandaloneBootstrap) AgentBootstrap(context.Context) (*workflowpb.AgentBootstrap, error) {
	return &workflowpb.AgentBootstrap{ServerAddr: f.serverAddr}, nil
}
