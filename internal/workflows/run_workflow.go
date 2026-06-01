package workflows

import (
	"encoding/json"
	"fmt"
	"time"

	wf "go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	gen "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

// AgentQueue is the per-machine Temporal task queue an agent worker polls. The
// target id already encodes run + role + index, so one agent ↔ one queue and a
// workflow session on that queue lands on exactly that machine.
func AgentQueue(targetID string) string { return "stroppy-agent-" + targetID }

// Workflows implements gen.RunWorkflowServiceWorkflows.
type Workflows struct{}

var _ gen.RunWorkflowServiceWorkflows = Workflows{}

// RunWorkflow constructs the per-run orchestration. The generated wrapper wires
// GetRunWorkflowState as the live query handler.
func (Workflows) RunWorkflow(_ wf.Context, input *gen.RunWorkflowWorkflowInput) (gen.RunWorkflowWorkflow, error) {
	return &runWorkflow{req: input.Req, status: common.Status_STATUS_PENDING}, nil
}

// runWorkflow is the per-run orchestration state. Workflow code is
// single-threaded; the query handler reads this goroutine's state between
// events, so no mutex is needed.
type runWorkflow struct {
	req    *gen.RunConfig
	stages []*gen.Stage
	status common.Status
}

// Stage names surfaced via GetRunWorkflowState.
const (
	stageDeploy      = "deploy"
	stageBuildRecipe = "build_recipe"
	stageTeardown    = "teardown"
)

// Execute runs the full pipeline: deploy machines -> build the recipe -> run each
// machine's command plan on its own agent (tier-ordered) -> teardown.
func (w *runWorkflow) Execute(ctx wf.Context) error {
	logger := wf.GetLogger(ctx)
	w.status = common.Status_STATUS_RUNNING
	runID := w.req.GetId()

	// No HeartbeatTimeout: the server activities (terraform apply/teardown) run a
	// single long blocking call and don't heartbeat. A heartbeat deadline would
	// make Temporal retry mid-apply, and the retry collides on the terraform
	// state lock held by the still-running first attempt. StartToCloseTimeout
	// alone bounds them.
	serverCtx := wf.WithActivityOptions(ctx, wf.ActivityOptions{StartToCloseTimeout: 30 * time.Minute})

	var sa *ServerActivities // nil receiver: ExecuteActivity-by-func resolves the registered method.

	// 1. Deploy machines (network + containers/VMs). Returns the targets +
	// endpoint + teardown handles.
	w.startStage(ctx, stageDeploy)
	var dep gen.Deployment
	if err := wf.ExecuteActivity(serverCtx, sa.DeployMachinesActivity, w.req).Get(serverCtx, &dep); err != nil {
		w.failStage(ctx, stageDeploy)
		w.status = common.Status_STATUS_FAILED
		return err
	}
	w.completeStage(ctx, stageDeploy)

	// Always tear down (even on failure/cancel) using the deploy handles.
	defer func() {
		dctx, _ := wf.NewDisconnectedContext(ctx)
		dctx = wf.WithActivityOptions(dctx, wf.ActivityOptions{StartToCloseTimeout: 30 * time.Minute})
		w.startStage(ctx, stageTeardown)
		if err := wf.ExecuteActivity(dctx, sa.TeardownActivity, &TeardownRequest{Config: w.req, Deployment: &dep}).Get(dctx, nil); err != nil {
			logger.Error("teardown failed", "err", err)
			w.failStage(ctx, stageTeardown)
		} else {
			w.completeStage(ctx, stageTeardown)
		}
		if w.status != common.Status_STATUS_FAILED {
			w.status = common.Status_STATUS_COMPLETED
		}
	}()

	// 2. Build the per-machine command plans (reusing the run recipe in collect mode).
	w.startStage(ctx, stageBuildRecipe)
	var plans []InstancePlan
	if err := wf.ExecuteActivity(serverCtx, sa.BuildRecipeActivity, &BuildRequest{Config: w.req, Deployment: &dep}).Get(serverCtx, &plans); err != nil {
		w.failStage(ctx, stageBuildRecipe)
		w.status = common.Status_STATUS_FAILED
		return err
	}
	w.completeStage(ctx, stageBuildRecipe)

	// 3. Run each machine's plan on ITS OWN agent (tier-ordered).
	for _, plan := range plans {
		if err := w.runInstancePlan(ctx, runID, plan); err != nil {
			w.status = common.Status_STATUS_FAILED
			return err
		}
	}
	return nil
}

// runInstancePlan opens a worker session pinned to the plan's machine (its
// dedicated task queue), waits for the agent online, then runs the machine's
// commands in order. The session is always completed before returning.
func (w *runWorkflow) runInstancePlan(ctx wf.Context, runID string, plan InstancePlan) error {
	ao := wf.ActivityOptions{
		TaskQueue:           AgentQueue(plan.TargetID),
		StartToCloseTimeout: 60 * time.Minute,
		HeartbeatTimeout:    time.Minute,
	}
	sessCtx, err := wf.CreateSession(wf.WithActivityOptions(ctx, ao), &wf.SessionOptions{
		CreationTimeout:  5 * time.Minute,
		ExecutionTimeout: 120 * time.Minute,
	})
	if err != nil {
		return fmt.Errorf("workflows: create session for %s (%s): %w", plan.TargetID, plan.Role, err)
	}
	defer wf.CompleteSession(sessCtx)

	if err := gen.EnsureAgentOnlineActivity(sessCtx); err != nil {
		return fmt.Errorf("workflows: agent %s not online: %w", plan.TargetID, err)
	}

	for i, cmd := range plan.Commands {
		label := cmd.Label
		if label == "" {
			label = string(cmd.Action)
		}
		node := fmt.Sprintf("%s/%s", plan.TargetID, label)
		w.startStage(ctx, node)
		if err := runAgentCommand(sessCtx, cmd); err != nil {
			w.failStage(ctx, node)
			return fmt.Errorf("workflows: %s cmd %d (%s): %w", plan.TargetID, i, label, err)
		}
		w.completeStage(ctx, node)
	}
	return nil
}

// runAgentCommand dispatches one collected agent.Command to its agent activity,
// translating the primitive into the proto request. start_daemon becomes a
// systemd-run CallCmd so the process survives the (stateless) activity.
//
// cmd.Config arrives as a generic map[string]any after the InstancePlan is
// serialized through Temporal (agent.Command.Config is `any`), so we switch on
// cmd.Action and re-decode the config into the concrete type rather than relying
// on a Go type assertion.
func runAgentCommand(ctx wf.Context, cmd agent.Command) error {
	switch cmd.Action {
	case agent.ActionRunCmd:
		var cfg agent.RunCmdConfig
		if err := decodeConfig(cmd.Config, &cfg); err != nil {
			return err
		}
		return runCmd(ctx, cfg.Script)
	case agent.ActionWriteFile:
		var cfg agent.WriteFileConfig
		if err := decodeConfig(cmd.Config, &cfg); err != nil {
			return err
		}
		mkdir := cfg.MkdirParents == nil || *cfg.MkdirParents
		return gen.WriteFileActivity(ctx, &common.File{
			Info: &common.File_Info{
				Path:          cfg.Path,
				Mode:          cfg.Mode,
				Owner:         ownerUser(cfg.Owner),
				CreateParents: mkdir,
			},
			Append:  cfg.Append,
			Content: &common.File_Text{Text: cfg.Content},
		})
	case agent.ActionStartDaemon:
		var cfg agent.StartDaemonConfig
		if err := decodeConfig(cmd.Config, &cfg); err != nil {
			return err
		}
		return runCmd(ctx, systemdRunScript(cfg))
	default:
		return fmt.Errorf("workflows: unsupported command action %q", cmd.Action)
	}
}

// decodeConfig re-marshals the generic config payload into the concrete type.
func decodeConfig(v any, target any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("workflows: marshal command config: %w", err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("workflows: decode command config into %T: %w", target, err)
	}
	return nil
}

// runCmd runs a bash script on the agent and fails on a non-zero exit code.
func runCmd(ctx wf.Context, script string) error {
	res, err := gen.CallCmdActivity(ctx, &common.Cmd{Spec: &common.Cmd_Spec{
		Command: &common.Cmd_Spec_Script{Script: &common.Cmd_Script{Text: script, Shell: "/bin/bash"}},
	}})
	if err != nil {
		return err
	}
	if res.GetExitCode() != 0 {
		return fmt.Errorf("command exited %d: %s", res.GetExitCode(), string(res.GetStderr()))
	}
	return nil
}

// systemdRunScript turns a tracked-daemon request into a systemd-run command so
// the process survives the activity (stateless agent; lifecycle via teardown).
func systemdRunScript(cfg agent.StartDaemonConfig) string {
	s := "systemctl reset-failed " + cfg.Name + " 2>/dev/null; systemd-run --unit=" + cfg.Name
	for k, v := range cfg.Env {
		s += fmt.Sprintf(" --setenv=%s=%s", k, v)
	}
	s += " -- " + cfg.Bin
	for _, a := range cfg.Args {
		s += " " + a
	}
	return s
}

// ownerUser splits "user" or "user:group" and returns the user part for the
// proto File.Info.owner (group is implied by the agent's chown).
func ownerUser(owner string) string {
	if owner == "" {
		return ""
	}
	for i := 0; i < len(owner); i++ {
		if owner[i] == ':' {
			return owner[:i]
		}
	}
	return owner
}

// GetRunWorkflowState is the live query handler.
func (w *runWorkflow) GetRunWorkflowState() (*gen.RunState, error) {
	return &gen.RunState{Status: w.status, Stages: w.stages}, nil
}

func (w *runWorkflow) stage(name string) *gen.Stage {
	for _, s := range w.stages {
		if s.GetName() == name {
			return s
		}
	}
	s := &gen.Stage{Name: name}
	w.stages = append(w.stages, s)
	return s
}

func (w *runWorkflow) startStage(ctx wf.Context, name string) {
	s := w.stage(name)
	s.Status = common.Status_STATUS_RUNNING
	s.Attempt++
	s.StartedAt = timestamppb.New(wf.Now(ctx))
}

func (w *runWorkflow) completeStage(ctx wf.Context, name string) {
	s := w.stage(name)
	s.Status = common.Status_STATUS_COMPLETED
	s.FinishedAt = timestamppb.New(wf.Now(ctx))
}

func (w *runWorkflow) failStage(ctx wf.Context, name string) {
	s := w.stage(name)
	s.Status = common.Status_STATUS_FAILED
	s.FinishedAt = timestamppb.New(wf.Now(ctx))
}
