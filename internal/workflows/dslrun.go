// Package workflows: this file implements the generic Temporal interpreter
// for a compiled YAML-DSL plan (dslpb.CompiledPlan, Task 12's output). It
// replaces the hardcoded deployment/test workflows with a single DAG-walker
// that understands only "job", "needs", "when", "on_group" and the two job
// kinds (steps, service) — every domain-specific behavior (which files to
// write, which image to run) lives in the compiled plan data, not in this
// executor.
package workflows

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/cel-go/cel"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/proto"

	stroppyagent "github.com/stroppy-io/stroppy-cloud/internal/agent"
	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/expr"
	dslnomad "github.com/stroppy-io/stroppy-cloud/internal/dsl/nomad"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

// ExecuteCompiledPlanWorkflowName is the Temporal workflow type this file
// registers under (see RegisterWorkflows in register.go). It is not part of
// a generated *_temporal.pb.go service (CompiledPlan has no proto workflow
// service of its own — see internal/proto/cloud/v1/dsl/compiled.pb.go), so
// unlike the other workflows in this package there is no WorkflowXxxName
// constant generated for it; this one is hand-written to match the same
// naming convention (the bare Go function name, e.g.
// ExecuteDeploymentPlanWorkflowWorkflowName = "ExecuteDeploymentPlanWorkflow").
const ExecuteCompiledPlanWorkflowName = "ExecuteCompiledPlanWorkflow"

// NomadSubmitJobActivityName is the Temporal activity type name Nomad service
// jobs are submitted under. It is registered directly (not through a
// generated service, see cmd/cli/agent_cmd.go's registerNomadActivities) via
// w.RegisterActivity(impl.NomadSubmitJobActivity), so Temporal derives its
// type name from the bound method's short Go name at registration time.
const NomadSubmitJobActivityName = "NomadSubmitJobActivity"

// Job statuses reported in ExecuteCompiledPlanOutput.JobStatuses.
const (
	jobStatusOK      = "ok"
	jobStatusFailed  = "failed"
	jobStatusSkipped = "skipped"
)

// ExecuteCompiledPlanInput is the input to ExecuteCompiledPlanWorkflow: the
// compiled plan, the real machines the provider handed back for each
// machine_groups entry (keyed by group name), the agent task-queue map, and
// the node whose task queue hosts the Nomad activities (there is exactly one
// gateway per run — see internal/agent/nomad_activities.go's package doc).
type ExecuteCompiledPlanInput struct {
	Plan          *dslpb.CompiledPlan
	Machines      map[string][]*deploymentpb.MachineState
	Bootstrap     *workflowpb.AgentBootstrap
	GatewayNodeID string
}

// ExecuteCompiledPlanOutput reports the terminal status of every job in the
// plan: "ok", "failed" or "skipped" (see the package-level scheduling doc on
// ExecuteCompiledPlanWorkflow for what produces each).
type ExecuteCompiledPlanOutput struct {
	JobStatuses map[string]string
}

// ExecuteCompiledPlanWorkflow walks CompiledPlan.Jobs as a DAG and runs each
// job's action (steps or service) once every job it Needs has resolved.
//
// # Scheduling model
//
// This is NOT a fully event-driven DAG scheduler (a job starting the instant
// its last dependency finishes, independent of everything else). It is a
// wave-hybrid, generalizing executeDeploymentPlanWorkflow's priority-wave
// loop from a fixed sequence of waves to a true dependency DAG: each round,
// every not-yet-resolved job whose Needs are ALL already resolved (whether
// ok, failed, or skipped) is collected and run concurrently via workflow.Go;
// the round waits (workflow.Await) for all of them to finish, then repeats.
// A job with no unresolved deps left after round N always runs in round
// N+1 at the latest, so no job waits longer than one extra round versus a
// fully event-driven scheduler — an acceptable v1 trade-off for the
// simplicity of reusing the existing wave/workflow.Go/workflow.Await pattern
// (see executeDeploymentPlanWorkflow) instead of a generic per-edge
// notification mechanism.
//
// # when / skip semantics
//
// A non-empty CompiledJob.When is evaluated once per job (see evaluateWhen)
// against expr.ComponentEnv with `machines`, `matrix`, `inputs` and `target`
// all bound (see baseJobVars) — it is deterministic because every binding is
// a pure function of ExecuteCompiledPlanInput and the job's own compiled
// fields (the workflow's own arguments), so replaying the workflow
// re-derives the exact same bindings and therefore the exact same
// when-decision without any non-deterministic input (no SideEffect needed).
//
// False decisions do not run the job's action; the job is marked "skipped".
// Per the task-16 brief's decision (GitLab-CI rules:-skip semantics), a
// when-skipped job counts as SATISFIED for its dependents: `b: needs: [a]`
// where `a` was when-skipped still runs `b`. A job that FAILS (its action
// errored) is NOT satisfied: every transitive dependent is skipped in
// cascade (each round, a dependent whose needs are all resolved but at least
// one is not-satisfied is marked "skipped" without running its action, and
// is itself recorded as not-satisfied — so the skip keeps propagating
// through the DAG). Independent branches (jobs that do not depend, even
// transitively, on the failed job) are unaffected and run to completion.
//
// The workflow itself only returns a non-nil error if Temporal itself breaks
// the run (e.g. workflow.Await is canceled) — an individual job's activity
// failure surfaces solely as that job's "failed" status in the output.
func ExecuteCompiledPlanWorkflow(ctx workflow.Context, in *ExecuteCompiledPlanInput) (*ExecuteCompiledPlanOutput, error) {
	if in == nil || in.Plan == nil {
		return nil, fmt.Errorf("execute compiled plan: plan is required")
	}

	celEnv, err := expr.ComponentEnv()
	if err != nil {
		return nil, fmt.Errorf("execute compiled plan: build component env: %w", err)
	}
	groupsBinding := compiledMachinesBinding(in.Plan, in.Machines)

	jobs := in.Plan.GetJobs()
	statuses := make(map[string]string, len(jobs))
	satisfied := make(map[string]bool, len(jobs))
	done := make([]bool, len(jobs))

	for {
		readyIdx := make([]int, 0, len(jobs))
		for i, job := range jobs {
			if !done[i] && jobNeedsResolved(job, statuses) {
				readyIdx = append(readyIdx, i)
			}
		}
		if len(readyIdx) == 0 {
			anyLeft := false
			for i := range jobs {
				if !done[i] {
					anyLeft = true
					break
				}
			}
			if !anyLeft {
				break
			}
			// Defensive: a Needs reference that never resolves (a job id
			// missing from the plan, or a genuine cycle) — both are
			// programmatic inconsistencies the compiler is supposed to
			// reject before this ever runs. Mark whatever is left as
			// skipped rather than looping forever.
			for i, job := range jobs {
				if !done[i] {
					statuses[job.GetId()] = jobStatusSkipped
					satisfied[job.GetId()] = false
					done[i] = true
				}
			}
			break
		}

		type roundResult struct {
			status    string
			satisfied bool
		}
		results := make([]roundResult, len(readyIdx))
		finished := 0
		for k, idx := range readyIdx {
			job := jobs[idx]
			allNeedsSatisfied := jobNeedsSatisfied(job, satisfied)
			workflow.Go(ctx, func(gctx workflow.Context) {
				defer func() { finished++ }()
				if !allNeedsSatisfied {
					results[k] = roundResult{status: jobStatusSkipped, satisfied: false}
					return
				}
				status, ok := executeCompiledJob(gctx, in, job, celEnv, groupsBinding)
				results[k] = roundResult{status: status, satisfied: ok}
			})
		}
		if err := workflow.Await(ctx, func() bool { return finished == len(readyIdx) }); err != nil {
			return nil, err
		}
		for k, idx := range readyIdx {
			job := jobs[idx]
			statuses[job.GetId()] = results[k].status
			satisfied[job.GetId()] = results[k].satisfied
			done[idx] = true
		}
	}

	return &ExecuteCompiledPlanOutput{JobStatuses: statuses}, nil
}

// jobNeedsResolved reports whether every job.Needs entry already has an
// entry in statuses (regardless of its value) — i.e. whether job is a
// candidate to run/skip in the current scheduling round.
func jobNeedsResolved(job *dslpb.CompiledJob, statuses map[string]string) bool {
	for _, need := range job.GetNeeds() {
		if _, ok := statuses[need]; !ok {
			return false
		}
	}
	return true
}

// jobNeedsSatisfied reports whether every job.Needs entry resolved to a
// satisfied outcome (ok, or when-skipped — see the package doc on
// ExecuteCompiledPlanWorkflow). A single unsatisfied need (failed, or
// cascade-skipped) means job itself must be cascade-skipped.
func jobNeedsSatisfied(job *dslpb.CompiledJob, satisfied map[string]bool) bool {
	for _, need := range job.GetNeeds() {
		if !satisfied[need] {
			return false
		}
	}
	return true
}

// executeCompiledJob evaluates job.When (if any) and, when it passes, runs
// job's action (steps or service). It returns the job's terminal status and
// whether it counts as "satisfied" for its dependents (see the package doc).
func executeCompiledJob(
	ctx workflow.Context,
	in *ExecuteCompiledPlanInput,
	job *dslpb.CompiledJob,
	celEnv *cel.Env,
	groupsBinding map[string]any,
) (string, bool) {
	vars := baseJobVars(job, groupsBinding)

	proceed, err := evaluateWhen(celEnv, vars, job.GetWhen())
	if err != nil {
		workflow.GetLogger(ctx).Error("compiled plan job: when evaluation failed", "job_id", job.GetId(), "error", err)
		return jobStatusFailed, false
	}
	if !proceed {
		return jobStatusSkipped, true
	}

	var execErr error
	switch job.GetAction().(type) {
	case *dslpb.CompiledJob_Steps:
		execErr = executeStepsJob(ctx, in, job, celEnv, vars)
	case *dslpb.CompiledJob_Service:
		execErr = executeServiceJob(ctx, in, job, celEnv, vars)
	default:
		execErr = fmt.Errorf("job %q: neither steps nor service action is set", job.GetId())
	}
	if execErr != nil {
		workflow.GetLogger(ctx).Error("compiled plan job failed", "job_id", job.GetId(), "error", execErr)
		return jobStatusFailed, false
	}
	return jobStatusOK, true
}

// baseJobVars builds the CEL bindings shared by a job's When and every
// ${{ }} interpolation it triggers — the same four names expr.ComponentEnv
// declares (Task 1's graph.Validate already typechecked every `requires:`/
// `when:` expression against exactly these):
//
//   - `machines`: every machine_groups entry, keyed by name, as an
//     expr.MachineGroupView (compiledMachinesBinding).
//   - `matrix`: the job's own, already matrix-expanded instance values — see
//     dslpb.CompiledJob.Matrix's doc: matrix expansion happened at compile
//     time, one CompiledJob per instance, so there is nothing left to expand
//     here.
//   - `inputs`: the job's resolved component inputs (empty for a job that
//     did not originate from an include component). Scalar inputs
//     (CompiledJob.ResolvedInputs) bind as their compile-time-typed
//     google.protobuf.Value, converted via Value.AsInterface() into the
//     matching native Go type (float64/string/bool/nil/[]any/
//     map[string]any) that cel-go's default type adapter recognizes;
//     machine_group-typed inputs (CompiledJob.InputGroups) bind as
//     the referenced group's expr.MachineGroupView, built by groupView from
//     the SAME groupsBinding `machines` uses — so `inputs.nodes` and
//     `machines.<group>` are identical views for the same group, and
//     `inputs.nodes.machines[0].ip` resolves exactly as graph.Validate
//     typechecked it (expr.ComponentEnv declares inputs as
//     map[string]dyn, so a native scalar next to a native MachineGroupView
//     value in the same map is legal).
//   - `target`: the target_group's expr.MachineGroupView, left UNBOUND when
//     CompiledJob.TargetGroup is empty (a component with zero or multiple
//     machine_group inputs — see its doc). An expression referencing
//     `target` on such a job was never accepted by graph.Validate in the
//     first place, so there is nothing to bind it to.
//
// Scalar inputs are typed at runtime (see ResolvedInputs' doc): both
// `${{ inputs.count }}` interpolation and a `when: inputs.count > 0` against
// an int-typed InputSpec work — CompiledJob.ResolvedInputs carries a
// google.protobuf.Value per scalar input, and AsInterface() converts it to
// the native Go type (float64/string/bool/...) cel-go's default type
// adapter binds correctly, matching what expr.ComponentEnv's dyn typing
// already let Task 1's compile-time check assume.
func baseJobVars(job *dslpb.CompiledJob, groupsBinding map[string]any) map[string]any {
	matrixBinding := job.GetMatrix()
	if matrixBinding == nil {
		matrixBinding = map[string]string{}
	}

	inputs := make(map[string]any, len(job.GetResolvedInputs())+len(job.GetInputGroups()))
	for name, val := range job.GetResolvedInputs() {
		inputs[name] = val.AsInterface()
	}
	for name, group := range job.GetInputGroups() {
		inputs[name] = groupView(groupsBinding, group)
	}

	vars := map[string]any{
		"machines": groupsBinding,
		"matrix":   matrixBinding,
		"inputs":   inputs,
	}
	if tg := job.GetTargetGroup(); tg != "" {
		vars["target"] = groupView(groupsBinding, tg)
	}
	return vars
}

// groupView looks up group's expr.MachineGroupView in groupsBinding (built
// by compiledMachinesBinding — the same map the `machines` binding uses, so
// `inputs.<name>`/`target` and `machines.<group>` are identical views for the
// same group name), falling back to an empty view (Count 0, no Machines) if
// the name is missing. A missing group means CompiledJob.InputGroups/
// TargetGroup pointed at a machine_groups entry the plan does not define —
// graph.Validate is expected to reject that inconsistency at compile time, so
// this fallback only keeps runtime evaluation total (no map-lookup panic)
// rather than papering over an expected case.
func groupView(groupsBinding map[string]any, group string) any {
	if v, ok := groupsBinding[group]; ok {
		return v
	}
	return expr.MachineGroupView{}
}

// evaluateWhen evaluates a job's When expression (empty = always true).
func evaluateWhen(celEnv *cel.Env, vars map[string]any, when string) (bool, error) {
	if when == "" {
		return true, nil
	}
	val, err := expr.Eval(celEnv, when, vars)
	if err != nil {
		return false, fmt.Errorf("when %q: %w", when, err)
	}
	proceed, ok := val.(bool)
	if !ok {
		return false, fmt.Errorf("when %q: expected a bool result, got %T", when, val)
	}
	return proceed, nil
}

// compiledMachinesBinding builds the `machines` CEL binding: one
// expr.MachineGroupView per CompiledPlan.MachineGroups entry, with real
// per-machine IPs pulled from ExecuteCompiledPlanInput.Machines (the
// provider's actual output) and CPU/RAM/disk taken from the compiled group
// spec (MachineState carries no resource sizing of its own).
func compiledMachinesBinding(plan *dslpb.CompiledPlan, machines map[string][]*deploymentpb.MachineState) map[string]any {
	groups := make(map[string]any, len(plan.GetMachineGroups()))
	for _, mg := range plan.GetMachineGroups() {
		states := machines[mg.GetName()]
		views := make([]expr.MachineView, 0, len(states))
		for _, state := range states {
			views = append(views, machineViewFor(mg, state))
		}
		groups[mg.GetName()] = expr.MachineGroupView{
			Count:    int64(len(views)),
			Machines: views,
		}
	}
	return groups
}

// machineViewFor builds the expr.MachineView for one real machine, given the
// compiled group spec it belongs to (for CPU/RAM/disk — mirrors
// internal/dsl/graph/domain.go's buildView, minus the real IP that only
// exists once the provider has actually run).
func machineViewFor(mg *dslpb.MachineGroup, state *deploymentpb.MachineState) expr.MachineView {
	ramGb := int64(mg.GetRamMb() / 1024) //nolint:gosec // machine RAM/GB values are far below MaxInt64.
	cpu := int64(mg.GetCpu())
	var diskGb int64
	var disks []expr.DiskView
	if ds := mg.GetDisks(); len(ds) > 0 {
		diskGb = int64(ds[0].GetSizeGb()) //nolint:gosec // machine disk/GB values are far below MaxInt64.
		disks = []expr.DiskView{{Path: machineDiskDevice(state), SizeGb: diskGb}}
	}
	return expr.MachineView{
		IP:     machinePrivateAddress(state),
		RAMGb:  ramGb,
		CPU:    cpu,
		DiskGb: diskGb,
		Disks:  disks,
	}
}

// machinePrivateAddress returns a machine's "private" endpoint address (the
// convention privateEndpoints in deployment.go always names the
// agent-reachable address), falling back to the first endpoint if none is
// named "private", or "" if the machine has no endpoints at all.
func machinePrivateAddress(state *deploymentpb.MachineState) string {
	for _, ep := range state.GetEndpoints() {
		if ep.GetName() == "private" {
			return ep.GetAddress()
		}
	}
	if eps := state.GetEndpoints(); len(eps) > 0 {
		return eps[0].GetAddress()
	}
	return ""
}

// machineDiskDevice returns the device path a provider stamped onto a
// machine's disk (I2: dslrun used to hardcode DiskView.Path to "", making
// `mkfs ${{ machine.disks[0].path }}`-style recipe steps non-functional).
// Convention (see internal/infrastructure/provider/terraform.go and
// docker.go): the provider labels the machine's "private" endpoint with
// disk_device — mirroring machinePrivateAddress's own endpoint lookup, so a
// disk device is scoped to the same endpoint object as the machine's
// reachable address, consistent with how managed-service endpoints already
// carry endpoint-specific facts (e.g. deployment.go's database_path label).
// Falls back to the first endpoint if none is named "private", or "" if the
// machine has no endpoints, or the label is absent (docker containers have
// no block device — see docker.go).
func machineDiskDevice(state *deploymentpb.MachineState) string {
	for _, ep := range state.GetEndpoints() {
		if ep.GetName() == "private" {
			return ep.GetLabels()["disk_device"]
		}
	}
	if eps := state.GetEndpoints(); len(eps) > 0 {
		return eps[0].GetLabels()["disk_device"]
	}
	return ""
}

// machineGroupByName looks up a compiled machine group by name; a nil result
// is safe to pass to machineViewFor (proto getters on a nil *MachineGroup
// return zero values).
func machineGroupByName(plan *dslpb.CompiledPlan, name string) *dslpb.MachineGroup {
	for _, mg := range plan.GetMachineGroups() {
		if mg.GetName() == name {
			return mg
		}
	}
	return nil
}

// serviceByName looks up a compiled service spec by name.
func serviceByName(plan *dslpb.CompiledPlan, name string) *dslpb.ServiceSpec {
	for _, svc := range plan.GetServices() {
		if svc.GetName() == name {
			return svc
		}
	}
	return nil
}

// executeStepsJob runs a steps job: every step in job.GetSteps() runs on
// EVERY machine of job.OnGroup, sequentially per machine, with all machines
// running concurrently — mirroring executeDeploymentPlanWorkflow's
// workflow.Go + finished-counter + workflow.Await wave pattern, generalized
// from "one wave of independent components" to "one job's machine fan-out".
func executeStepsJob(ctx workflow.Context, in *ExecuteCompiledPlanInput, job *dslpb.CompiledJob, celEnv *cel.Env, vars map[string]any) error {
	groupName := job.GetOnGroup()
	machines := in.Machines[groupName]
	if len(machines) == 0 {
		return fmt.Errorf("job %q: on_group %q has no machines", job.GetId(), groupName)
	}
	mg := machineGroupByName(in.Plan, groupName)
	steps := job.GetSteps().GetSteps()

	finished := 0
	var jobErr error
	for _, machine := range machines {
		workflow.Go(ctx, func(gctx workflow.Context) {
			defer func() { finished++ }()
			if err := executeStepsOnMachine(gctx, in.Bootstrap, mg, machine, steps, job.GetId(), celEnv, vars); err != nil {
				if jobErr == nil {
					jobErr = err
				}
			}
		})
	}
	if err := workflow.Await(ctx, func() bool { return finished == len(machines) }); err != nil {
		return err
	}
	return jobErr
}

// executeStepsOnMachine runs job's step list, in order, on one machine: it
// ensures the agent is online, then executes each DslStep — agent steps
// reuse executeAgentStep (deployment.go) after ${{ }} interpolation, wait
// steps are lowered to a CallCmd curl-retry loop (see waitStepToAgentStep).
func executeStepsOnMachine(
	ctx workflow.Context,
	bootstrap *workflowpb.AgentBootstrap,
	mg *dslpb.MachineGroup,
	machine *deploymentpb.MachineState,
	steps []*dslpb.DslStep,
	jobID string,
	celEnv *cel.Env,
	vars map[string]any,
) error {
	taskQueue, err := agentTaskQueue(bootstrap, machine.GetNodeId())
	if err != nil {
		return err
	}
	activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           taskQueue,
		StartToCloseTimeout: 60 * time.Minute,
		HeartbeatTimeout:    time.Minute,
	})
	if err := executeActivityNoResult(activityCtx, workflowpb.EnsureAgentOnlineActivityActivityName); err != nil {
		return fmt.Errorf("agent %s is not online: %w", machine.GetNodeId(), err)
	}

	machineVars := withMachineBinding(vars, machineViewFor(mg, machine))
	for idx, dslStep := range steps {
		agentStep, err := resolveDslStep(celEnv, machineVars, jobID, idx, dslStep)
		if err != nil {
			return fmt.Errorf("job %q step %d: %w", jobID, idx, err)
		}
		if agentStep == nil {
			continue
		}
		if err := executeAgentStep(activityCtx, agentStep); err != nil {
			return fmt.Errorf("job %q step %d: %w", jobID, idx, err)
		}
	}
	return nil
}

// withMachineBinding returns a copy of base with `machine` bound to view —
// the per-machine CEL binding for on_group step interpolation (base is
// re-used, unmodified, by every machine's goroutine, so this must not
// mutate it).
func withMachineBinding(base map[string]any, view expr.MachineView) map[string]any {
	out := make(map[string]any, len(base)+1)
	for k, v := range base {
		out[k] = v
	}
	out["machine"] = view
	return out
}

// resolveDslStep turns one DslStep into the deployment.AgentStep to actually
// execute: an agent step is cloned and interpolated in place; a wait step has
// no agent-side wait activity (see the package doc / task-16 brief) and is
// instead lowered to a CallCmd curl-retry loop via waitStepToAgentStep.
func resolveDslStep(celEnv *cel.Env, vars map[string]any, jobID string, idx int, dslStep *dslpb.DslStep) (*deploymentpb.AgentStep, error) {
	switch step := dslStep.GetStep().(type) {
	case *dslpb.DslStep_Agent:
		return interpolateAgentStep(celEnv, vars, step.Agent)
	case *dslpb.DslStep_Wait:
		httpURL, err := interpolateString(celEnv, vars, step.Wait.GetHttp())
		if err != nil {
			return nil, fmt.Errorf("wait.http: %w", err)
		}
		return waitStepToAgentStep(jobID, idx, httpURL, step.Wait.GetTimeout()), nil
	default:
		return nil, fmt.Errorf("no step action set")
	}
}

// waitStepToAgentStep lowers a WaitStep to a CallCmd AgentStep that polls
// url with curl until it succeeds or timeout elapses. No agent-side wait
// activity exists (Task 15's Nomad/agent activities have no HTTP poller), so
// this is the pragmatic v1 choice the brief calls for: a curl-retry shell
// loop, one attempt every 5s, for timeout/5s attempts (minimum 1).
func waitStepToAgentStep(jobID string, idx int, url, timeout string) *deploymentpb.AgentStep {
	const pollInterval = 5 * time.Second
	d, err := time.ParseDuration(timeout)
	if err != nil || d <= 0 {
		d = time.Minute
	}
	attempts := int(d / pollInterval)
	if attempts < 1 {
		attempts = 1
	}
	script := fmt.Sprintf(
		"set -e\nfor i in $(seq 1 %d); do\n  if curl -fsS -o /dev/null %s; then exit 0; fi\n  sleep 5\ndone\necho 'wait timed out: %s' >&2\nexit 1\n",
		attempts, deploymentbuilder.ShellQuote(url), strings.ReplaceAll(url, "'", ""),
	)
	return &deploymentpb.AgentStep{
		Id: fmt.Sprintf("%s/wait/%d", jobID, idx),
		Action: &deploymentpb.AgentStep_CallCmd{
			CallCmd: &common.Cmd{
				Spec: &common.Cmd_Spec{
					Command: &common.Cmd_Spec_Script{
						Script: &common.Cmd_Script{Text: script, Shell: "/bin/sh"},
					},
					ExpectedExitCodes: []int32{0},
				},
			},
		},
	}
}

// executeServiceJob runs a service job: it resolves the named ServiceSpec,
// evaluates every ${{ }} in its env against vars, merges job.With (also
// evaluated) into that env copy verbatim as extra env vars, builds a Nomad
// job for svc.OnGroup's real machines, and submits it via
// NomadSubmitJobActivity on the gateway node's task queue.
func executeServiceJob(ctx workflow.Context, in *ExecuteCompiledPlanInput, job *dslpb.CompiledJob, celEnv *cel.Env, vars map[string]any) error {
	svcName := job.GetService()
	svc := serviceByName(in.Plan, svcName)
	if svc == nil {
		return fmt.Errorf("job %q: service %q is not defined in the plan", job.GetId(), svcName)
	}
	machines := in.Machines[svc.GetOnGroup()]
	if len(machines) == 0 {
		return fmt.Errorf("job %q: service %q on_group %q has no machines", job.GetId(), svcName, svc.GetOnGroup())
	}

	evaluatedSvc, ok := proto.Clone(svc).(*dslpb.ServiceSpec)
	if !ok {
		return fmt.Errorf("job %q: service %q: unexpected clone type", job.GetId(), svcName)
	}
	env := make(map[string]string, len(svc.GetEnv())+len(job.GetWith()))
	for k, v := range svc.GetEnv() {
		nv, err := interpolateString(celEnv, vars, v)
		if err != nil {
			return fmt.Errorf("job %q: service %q env %q: %w", job.GetId(), svcName, k, err)
		}
		env[k] = nv
	}
	for k, v := range job.GetWith() {
		nv, err := interpolateString(celEnv, vars, v)
		if err != nil {
			return fmt.Errorf("job %q: with %q: %w", job.GetId(), k, err)
		}
		env[k] = nv
	}
	evaluatedSvc.Env = env

	nodes := make([]dslnomad.NodeRef, 0, len(machines))
	for _, machine := range machines {
		nodes = append(nodes, dslnomad.NodeRef{
			NodeID:    machine.GetNodeId(),
			PrivateIP: machinePrivateAddress(machine),
		})
	}

	nomadJob, err := dslnomad.BuildJob(evaluatedSvc, nodes)
	if err != nil {
		return fmt.Errorf("job %q: build nomad job for service %q: %w", job.GetId(), svcName, err)
	}
	jobJSON, err := json.Marshal(nomadJob)
	if err != nil {
		return fmt.Errorf("job %q: marshal nomad job for service %q: %w", job.GetId(), svcName, err)
	}

	// waitTimeout reuses the service's own health.timeout (already a
	// compile-validated time.ParseDuration string — see internal/dsl/lower's
	// health lowering) as the budget NomadSubmitJobActivity polls
	// allocations for. A service with no health block (waitTimeout == 0)
	// leaves NomadSubmitJobActivity's own default (5m,
	// defaultNomadWaitTimeout) unchanged — this is additive, not a behavior
	// change for existing recipes (F3).
	waitTimeout, err := parseHealthTimeout(svc.GetHealth().GetTimeout())
	if err != nil {
		return fmt.Errorf("job %q: service %q: health.timeout: %w", job.GetId(), svcName, err)
	}

	taskQueue, err := agentTaskQueue(in.Bootstrap, in.GatewayNodeID)
	if err != nil {
		return fmt.Errorf("job %q: gateway task queue: %w", job.GetId(), err)
	}
	// activityTimeout must exceed waitTimeout (NomadSubmitJobActivity's own
	// internal poll deadline) or Temporal cancels the activity call before
	// that deadline is ever reached, silently truncating a longer configured
	// health.timeout. 5 minutes of headroom covers job registration +
	// scheduling overhead beyond the alloc-running wait itself.
	activityTimeout := 10 * time.Minute
	if waitTimeout > 0 {
		activityTimeout = waitTimeout + 5*time.Minute
	}
	activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           taskQueue,
		StartToCloseTimeout: activityTimeout,
		HeartbeatTimeout:    time.Minute,
	})
	var out stroppyagent.NomadSubmitJobOutput
	if err := workflow.ExecuteActivity(activityCtx, NomadSubmitJobActivityName, &stroppyagent.NomadSubmitJobInput{
		JobJSON:     jobJSON,
		WaitTimeout: waitTimeout,
	}).Get(activityCtx, &out); err != nil {
		return fmt.Errorf("job %q: nomad submit %q: %w", job.GetId(), svcName, err)
	}
	return nil
}

// parseHealthTimeout parses a ServiceSpec's health.timeout into the wait
// budget NomadSubmitJobActivity should poll for. Empty (no health block on
// this service) returns zero, which NomadSubmitJobActivity treats as "use
// its own default" (defaultNomadWaitTimeout, 5m).
//
// Deferred (SP-F carryover, see
// docs/superpowers/specs/2026-07-08-sp-f-execution-shapeup.md §3 F3, Variant
// B): decoupling submit from wait into two separate DAG-visible steps (a new
// ast/dslpb step type, built on the already-existing-but-unused
// NomadJobStatusActivity, internal/agent/nomad_activities.go) so a Run's
// per-job status (SP-E) can show "waiting for healthy" distinct from
// "submitted", and so an operator can wait longer without re-submitting the
// job. Deferred until live testing shows this Variant A's single-activity-
// timeout budget is insufficient for a real slow-starting database.
func parseHealthTimeout(raw string) (time.Duration, error) {
	if raw == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parse duration %q: %w", raw, err)
	}
	return d, nil
}

// interpolateAgentStep clones step and replaces every ${{ expr }} marker
// reachable from its known string fields (CreateDir/WriteFile/FetchFile
// path, WriteFile inline text, FetchFile URL reference, CallCmd script/argv/
// cwd/env values) with the fmt-formatted result of evaluating expr against
// vars. This is an enumerated, known-field walk rather than a generic proto
// reflection walker — the same pragmatic v1 scope the brief calls for wait
// steps, since AgentStep's oneof shapes are small and fixed.
func interpolateAgentStep(celEnv *cel.Env, vars map[string]any, step *deploymentpb.AgentStep) (*deploymentpb.AgentStep, error) {
	cloned, ok := proto.Clone(step).(*deploymentpb.AgentStep)
	if !ok {
		return nil, fmt.Errorf("agent step %q: unexpected clone type", step.GetId())
	}

	var err error
	switch action := cloned.GetAction().(type) {
	case *deploymentpb.AgentStep_CreateDir:
		if info := action.CreateDir.GetInfo(); info != nil {
			if info.Path, err = interpolateString(celEnv, vars, info.Path); err != nil {
				return nil, err
			}
		}
	case *deploymentpb.AgentStep_WriteFile:
		if info := action.WriteFile.GetInfo(); info != nil {
			if info.Path, err = interpolateString(celEnv, vars, info.Path); err != nil {
				return nil, err
			}
		}
		switch content := action.WriteFile.GetContent().(type) {
		case *common.File_Text:
			if content.Text, err = interpolateString(celEnv, vars, content.Text); err != nil {
				return nil, err
			}
		case *common.File_AsRef_:
			if content.AsRef.Uri, err = interpolateString(celEnv, vars, content.AsRef.Uri); err != nil {
				return nil, err
			}
		}
	case *deploymentpb.AgentStep_FetchFile:
		if info := action.FetchFile.GetInfo(); info != nil {
			if info.Path, err = interpolateString(celEnv, vars, info.Path); err != nil {
				return nil, err
			}
		}
		if ref := action.FetchFile.GetAsRef(); ref != nil {
			if ref.Uri, err = interpolateString(celEnv, vars, ref.Uri); err != nil {
				return nil, err
			}
		}
	case *deploymentpb.AgentStep_CallCmd:
		spec := action.CallCmd.GetSpec()
		if spec == nil {
			break
		}
		if spec.Cwd, err = interpolateString(celEnv, vars, spec.Cwd); err != nil {
			return nil, err
		}
		for k, v := range spec.Env {
			nv, ierr := interpolateString(celEnv, vars, v)
			if ierr != nil {
				return nil, ierr
			}
			spec.Env[k] = nv
		}
		switch cmd := spec.GetCommand().(type) {
		case *common.Cmd_Spec_Argv:
			for i, a := range cmd.Argv.GetArgs() {
				na, ierr := interpolateString(celEnv, vars, a)
				if ierr != nil {
					return nil, ierr
				}
				cmd.Argv.Args[i] = na
			}
		case *common.Cmd_Spec_Script:
			if cmd.Script.Text, err = interpolateString(celEnv, vars, cmd.Script.Text); err != nil {
				return nil, err
			}
		}
	}
	return cloned, nil
}

// dslInterpMarker matches ${{ ... }} interpolation markers, duplicating
// internal/dsl/expr's own (unexported) pattern rather than exporting it from
// a package this task does not otherwise touch. Keep in sync with
// expr.Extract's interpBraces if that pattern ever changes.
var dslInterpMarker = regexp.MustCompile(`(?s)\$\{\{(.*?)\}\}`)

// interpolateString replaces every ${{ expr }} marker in s with the
// fmt-formatted result of evaluating expr (via expr.Eval) against celEnv/
// vars. Strings with no markers are returned unchanged without compiling
// anything.
func interpolateString(celEnv *cel.Env, vars map[string]any, s string) (string, error) {
	if !strings.Contains(s, "${{") {
		return s, nil
	}
	var evalErr error
	out := dslInterpMarker.ReplaceAllStringFunc(s, func(match string) string {
		if evalErr != nil {
			return match
		}
		sub := dslInterpMarker.FindStringSubmatch(match)
		body := strings.TrimSpace(sub[1])
		val, err := expr.Eval(celEnv, body, vars)
		if err != nil {
			evalErr = fmt.Errorf("eval %q: %w", body, err)
			return match
		}
		return fmt.Sprintf("%v", val)
	})
	if evalErr != nil {
		return "", evalErr
	}
	return out, nil
}
