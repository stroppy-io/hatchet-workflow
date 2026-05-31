package workflows

import (
	"fmt"
	"time"

	wf "go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/deploy/dockerprov"
	"github.com/stroppy-io/stroppy-cloud/internal/deploy/recipe"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	gen "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

// Stage names surfaced via GetRunState.
const (
	stageDeploy      = "deploy"
	stageBuildRecipe = "build_recipe"
	stageTeardown    = "teardown"
)

// Workflows implements gen.TestServiceWorkflows. The 3 child workflows are
// trivial stubs; TestWorkflow is the real orchestration.
type Workflows struct{}

var _ gen.TestServiceWorkflows = Workflows{}

// emptyChild is a stub child workflow whose Execute returns the empty response.
type emptyChild struct{}

// InstallDatabaseWorkflow returns a stub child.
func (Workflows) InstallDatabaseWorkflow(_ wf.Context, _ *gen.InstallDatabaseWorkflowWorkflowInput) (gen.InstallDatabaseWorkflowWorkflow, error) {
	return installDatabaseStub{}, nil
}

// InstallStroppyWorkflow returns a stub child.
func (Workflows) InstallStroppyWorkflow(_ wf.Context, _ *gen.InstallStroppyWorkflowWorkflowInput) (gen.InstallStroppyWorkflowWorkflow, error) {
	return installStroppyStub{}, nil
}

// RunWorkloadWorkflow returns a stub child.
func (Workflows) RunWorkloadWorkflow(_ wf.Context, _ *gen.RunWorkloadWorkflowWorkflowInput) (gen.RunWorkloadWorkflowWorkflow, error) {
	return runWorkloadStub{}, nil
}

type installDatabaseStub struct{}

func (installDatabaseStub) Execute(_ wf.Context) (*gen.InstallDatabaseWorkflowResponse, error) {
	return &gen.InstallDatabaseWorkflowResponse{}, nil
}

type installStroppyStub struct{}

func (installStroppyStub) Execute(_ wf.Context) (*gen.InstallStroppyWorkflowResponse, error) {
	return &gen.InstallStroppyWorkflowResponse{}, nil
}

type runWorkloadStub struct{}

func (runWorkloadStub) Execute(_ wf.Context) (*gen.RunWorkloadWorkflowResponse, error) {
	return &gen.RunWorkloadWorkflowResponse{}, nil
}

// TestWorkflow constructs the orchestration. The generated buildTestWorkflow
// wires GetRunState as the query handler.
func (Workflows) TestWorkflow(_ wf.Context, input *gen.TestWorkflowWorkflowInput) (gen.TestWorkflowWorkflow, error) {
	return &testWorkflow{
		req:    input.Req,
		status: common.Status_STATUS_PENDING,
	}, nil
}

// testWorkflow is the per-run orchestration state. Workflow code is
// single-threaded; the query handler (GetRunState) reads this same goroutine's
// state between events, so no mutex is needed.
type testWorkflow struct {
	req    *gen.TestWorkflowRequest
	stages []*gen.Stage
	status common.Status
}

// Execute walks the stages, updating live state for the GetRunState query.
func (w *testWorkflow) Execute(ctx wf.Context) (*gen.TestWorkflowResponse, error) {
	logger := wf.GetLogger(ctx)
	w.status = common.Status_STATUS_RUNNING

	run := w.req.GetTestRun()
	runID := run.GetId()

	// Server activities run on the default (server) task queue "stroppy-cloud",
	// where the docker SDK lives. Give them a generous timeout.
	serverCtx := wf.WithActivityOptions(ctx, wf.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
	})

	var sa *ServerActivities // registered by func; nil receiver is fine for ExecuteActivity-by-func.

	// Always tear down, even on failure or cancellation. We build the teardown
	// closure now and defer it so every early return still runs it.
	failed := false
	defer func() {
		// Disconnected context so an outer cancellation still tears the stand down.
		dctx, _ := wf.NewDisconnectedContext(ctx)
		dctx = wf.WithActivityOptions(dctx, wf.ActivityOptions{StartToCloseTimeout: 5 * time.Minute})
		w.startStage(ctx, stageTeardown)
		var tr TeardownResult
		if err := wf.ExecuteActivity(dctx, sa.TeardownActivity, &TeardownRequest{RunID: runID}).Get(dctx, &tr); err != nil {
			logger.Error("teardown failed", "err", err)
			w.failStage(ctx, stageTeardown)
		} else {
			w.completeStage(ctx, stageTeardown)
		}
		if failed {
			w.status = common.Status_STATUS_FAILED
		} else {
			w.status = common.Status_STATUS_COMPLETED
		}
	}()

	// 1. Deploy the containers.
	w.startStage(ctx, stageDeploy)
	var deployRes DeployResult
	if err := wf.ExecuteActivity(serverCtx, sa.DeployActivity, &DeployRequest{
		Topology: run.GetTopology(),
		RunID:    runID,
	}).Get(serverCtx, &deployRes); err != nil {
		w.failStage(ctx, stageDeploy)
		failed = true
		return nil, err
	}
	w.completeStage(ctx, stageDeploy)

	// 2. Build the concrete deploy ops (recipe + placeholder substitution).
	w.startStage(ctx, stageBuildRecipe)
	var buildRes BuildResult
	if err := wf.ExecuteActivity(serverCtx, sa.BuildRecipeActivity, &BuildRequest{
		TestRun:     run,
		InstanceIPs: deployRes.InstanceIPs,
	}).Get(serverCtx, &buildRes); err != nil {
		w.failStage(ctx, stageBuildRecipe)
		failed = true
		return nil, err
	}
	w.completeStage(ctx, stageBuildRecipe)

	// 3. Run each machine's plan on ITS OWN agent. Each instance's agent listens
	// on a dedicated task queue (AgentQueue(runID, instanceID)) with exactly one
	// worker, so a session created on that queue can only land on that machine —
	// deterministic routing of every component's steps to the machine it belongs
	// to. Plans are tier-ordered (consensus/DB before proxy before workload).
	for _, plan := range buildRes.Plans {
		if err := w.runInstancePlan(ctx, runID, plan); err != nil {
			failed = true
			return nil, err
		}
	}

	return &gen.TestWorkflowResponse{}, nil
}

// runInstancePlan opens a worker session pinned to the plan's machine (via its
// dedicated task queue), waits for the agent to be online, then runs the
// machine's steps in order — each a separate pipeline node namespaced by instance
// so steps from different machines never collide. The session is always
// completed before returning.
func (w *testWorkflow) runInstancePlan(ctx wf.Context, runID string, plan InstancePlan) error {
	queue := dockerprov.AgentQueue(runID, plan.InstanceID)
	ao := wf.ActivityOptions{
		TaskQueue:           queue,
		StartToCloseTimeout: 30 * time.Minute,
		HeartbeatTimeout:    time.Minute,
	}
	sessCtx, err := wf.CreateSession(wf.WithActivityOptions(ctx, ao), &wf.SessionOptions{
		CreationTimeout:  5 * time.Minute,
		ExecutionTimeout: 60 * time.Minute,
	})
	if err != nil {
		return fmt.Errorf("workflows: create session for %s (%s): %w", plan.InstanceID, plan.Role, err)
	}
	defer wf.CompleteSession(sessCtx)

	if err := gen.EnsureAgentOnlineActivity(sessCtx); err != nil {
		return fmt.Errorf("workflows: agent %s not online: %w", plan.InstanceID, err)
	}

	for _, st := range plan.Steps {
		node := fmt.Sprintf("%s/%s", plan.InstanceID, st.Name)
		w.startStage(ctx, node)
		if err := runStep(sessCtx, st); err != nil {
			w.failStage(ctx, node)
			return fmt.Errorf("workflows: %s step %q: %w", plan.InstanceID, st.Name, err)
		}
		w.completeStage(ctx, node)
	}
	return nil
}

// runStep dispatches one recipe step to its agent activity (in the session),
// rebuilding the common.File / common.Cmd from the step's plain fields.
func runStep(ctx wf.Context, st recipe.Step) error {
	switch st.Kind {
	case recipe.StepWriteFile:
		return gen.WriteFileActivity(ctx, &common.File{
			Info:    &common.File_Info{Path: st.Path, Mode: st.Mode},
			Content: &common.File_Text{Text: st.Content},
		})
	case recipe.StepFetchFile:
		return gen.FetchFileActivity(ctx, &common.File{
			Info:    &common.File_Info{Path: st.Path, Mode: st.Mode},
			Content: &common.File_AsRef_{AsRef: &common.File_AsRef{Uri: st.URI, Checksum: st.Checksum}},
		})
	case recipe.StepCmd:
		return runAgentCmd(ctx, &common.Cmd{Spec: &common.Cmd_Spec{
			Command: &common.Cmd_Spec_Script{Script: &common.Cmd_Script{Text: st.Script, Shell: "/bin/bash"}},
		}})
	default:
		return fmt.Errorf("workflows: unknown step kind %q", st.Kind)
	}
}

// runAgentCmd runs one command on the agent and fails on a non-zero exit code.
func runAgentCmd(ctx wf.Context, c *common.Cmd) error {
	res, err := gen.CallCmdActivity(ctx, c)
	if err != nil {
		return err
	}
	if res.GetExitCode() != 0 {
		return fmt.Errorf("command exited %d: %s", res.GetExitCode(), string(res.GetStderr()))
	}
	return nil
}

// GetRunState is the query handler returning the live run state.
func (w *testWorkflow) GetRunState() (*gen.RunState, error) {
	return &gen.RunState{Status: w.status, Stages: w.stages}, nil
}

// stage returns the existing stage with the given name, or appends a new one.
func (w *testWorkflow) stage(name string) *gen.Stage {
	for _, s := range w.stages {
		if s.GetName() == name {
			return s
		}
	}
	s := &gen.Stage{Name: name, NodeExecutionId: name}
	w.stages = append(w.stages, s)
	return s
}

// startStage marks a stage RUNNING with a deterministic start time.
func (w *testWorkflow) startStage(ctx wf.Context, name string) {
	s := w.stage(name)
	s.Status = common.Status_STATUS_RUNNING
	s.Attempt++
	s.StartedAt = timestamppb.New(wf.Now(ctx))
}

// completeStage marks a stage COMPLETED with a deterministic finish time.
func (w *testWorkflow) completeStage(ctx wf.Context, name string) {
	s := w.stage(name)
	s.Status = common.Status_STATUS_COMPLETED
	s.FinishedAt = timestamppb.New(wf.Now(ctx))
}

// failStage marks a stage FAILED with a deterministic finish time.
func (w *testWorkflow) failStage(ctx wf.Context, name string) {
	s := w.stage(name)
	s.Status = common.Status_STATUS_FAILED
	s.FinishedAt = timestamppb.New(wf.Now(ctx))
}
