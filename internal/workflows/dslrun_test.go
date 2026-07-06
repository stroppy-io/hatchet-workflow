package workflows

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"

	stroppyagent "github.com/stroppy-io/stroppy-cloud/internal/agent"
	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

// --- stub activity functions (real signatures, for env registration) -------
//
// env.OnActivity(name, ...) requires the environment to already know the
// activity's Go signature (see go.temporal.io/sdk internal TestWorkflowEnvironment.
// OnActivity: the string branch panics unless the name is registered), so
// every activity ExecuteCompiledPlanWorkflow can call is registered here
// under its real production name with a tiny stub body the mock always
// intercepts before it runs.

func stubCallCmd(context.Context, *common.Cmd) (*common.Cmd_Result, error) { return nil, nil }
func stubCreateDir(context.Context, *common.Dir) error                     { return nil }
func stubWriteFile(context.Context, *common.File) error                    { return nil }
func stubFetchFile(context.Context, *common.File) error                    { return nil }
func stubEnsureAgentOnline(context.Context) error                          { return nil }

func stubNomadSubmitJob(context.Context, *stroppyagent.NomadSubmitJobInput) (*stroppyagent.NomadSubmitJobOutput, error) {
	return nil, nil
}

// activityQueueRecord captures which task queue an activity executed on.
type activityQueueRecord struct {
	name      string
	taskQueue string
}

// recorder captures activity invocations in completion order across
// possibly-concurrent workflow.Go goroutines (mock callbacks run on their
// own goroutines per the SDK's OnActivity doc), so every append is
// mutex-guarded.
type recorder struct {
	mu          sync.Mutex
	events      []string
	queueChecks []activityQueueRecord
}

func (r *recorder) add(event string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *recorder) addQueueCheck(name string, taskQueue string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queueChecks = append(r.queueChecks, activityQueueRecord{
		name:      name,
		taskQueue: taskQueue,
	})
}

func (r *recorder) indexOf(event string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, e := range r.events {
		if e == event {
			return i
		}
	}
	return -1
}

func (r *recorder) contains(event string) bool {
	return r.indexOf(event) >= 0
}

func registerCompiledPlanActivityStubs(env *testsuite.TestWorkflowEnvironment) {
	env.RegisterActivityWithOptions(stubCallCmd, activity.RegisterOptions{Name: workflowpb.CallCmdActivityActivityName})
	env.RegisterActivityWithOptions(stubCreateDir, activity.RegisterOptions{Name: workflowpb.CreateDirActivityActivityName})
	env.RegisterActivityWithOptions(stubWriteFile, activity.RegisterOptions{Name: workflowpb.WriteFileActivityActivityName})
	env.RegisterActivityWithOptions(stubFetchFile, activity.RegisterOptions{Name: workflowpb.FetchFileActivityActivityName})
	env.RegisterActivityWithOptions(stubEnsureAgentOnline, activity.RegisterOptions{Name: workflowpb.EnsureAgentOnlineActivityActivityName})
	env.RegisterActivityWithOptions(stubNomadSubmitJob, activity.RegisterOptions{Name: NomadSubmitJobActivityName})
}

// machineState builds a MachineState with a single "private" endpoint —
// mirrors deployment.go's privateEndpoints convention that
// machinePrivateAddress (dslrun.go) reads from.
func machineState(nodeID, ip string) *deploymentpb.MachineState {
	return &deploymentpb.MachineState{
		NodeId: nodeID,
		Endpoints: []*deploymentpb.Endpoint{
			{Name: "private", Address: ip},
		},
	}
}

func stepsJob(id string, needs []string, onGroup string, when string, matrix map[string]string, steps ...*deploymentpb.AgentStep) *dslpb.CompiledJob {
	dslSteps := make([]*dslpb.DslStep, 0, len(steps))
	for _, s := range steps {
		dslSteps = append(dslSteps, &dslpb.DslStep{Step: &dslpb.DslStep_Agent{Agent: s}})
	}
	return &dslpb.CompiledJob{
		Id:      id,
		Needs:   needs,
		OnGroup: onGroup,
		When:    when,
		Matrix:  matrix,
		Action:  &dslpb.CompiledJob_Steps{Steps: &dslpb.StepList{Steps: dslSteps}},
	}
}

func serviceJob(id string, needs []string, service string, with map[string]string) *dslpb.CompiledJob {
	return &dslpb.CompiledJob{
		Id:     id,
		Needs:  needs,
		With:   with,
		Action: &dslpb.CompiledJob_Service{Service: service},
	}
}

// buildTestPlan assembles one CompiledPlan exercising every documented
// scheduling/eval behaviour in a single DAG:
//
//	a (steps)  -> b (service), a -> c (steps, fails)
//	[b, c]     -> d (steps): c failed, so d must be cascade-skipped
//	e (steps, when=false, skipped) -> f (steps): when-skip still satisfies f
//	bench_insert / bench_select: two matrix instances of the same job body,
//	  each resolving ${{ matrix.workload }} to its own instance value.
func buildTestPlan() *dslpb.CompiledPlan {
	return &dslpb.CompiledPlan{
		MachineGroups: []*dslpb.MachineGroup{
			{Name: "app", Count: 1, Cpu: 2, RamMb: 2048},
			{Name: "svc", Count: 1, Cpu: 4, RamMb: 4096, Disks: []*dslpb.DiskSpec{{SizeGb: 10}}},
		},
		Services: []*dslpb.ServiceSpec{
			{
				Name:    "echo",
				OnGroup: "svc",
				Image:   "busybox:latest",
				Env: map[string]string{
					"NODE_IP": "${{ machines.svc.machines[0].ip }}",
				},
			},
		},
		Jobs: []*dslpb.CompiledJob{
			stepsJob("a", nil, "app", "", nil, deploymentbuilder.CallCmdStep("a/0", 0, "echo a")),
			serviceJob("b", []string{"a"}, "echo", map[string]string{"EXTRA": "1"}),
			stepsJob("c", []string{"a"}, "app", "", nil, deploymentbuilder.CallCmdStep("c/0", 0, "echo c")),
			stepsJob("d", []string{"b", "c"}, "app", "", nil, deploymentbuilder.CallCmdStep("d/0", 0, "echo d")),
			stepsJob("e", nil, "app", "1 == 2", nil, deploymentbuilder.CallCmdStep("e/0", 0, "echo e")),
			stepsJob("f", []string{"e"}, "app", "", nil, deploymentbuilder.CallCmdStep("f/0", 0, "echo f")),
			stepsJob("bench_insert", nil, "app", "", map[string]string{"workload": "insert"},
				deploymentbuilder.CallCmdStep("bench_insert/0", 0, "run ${{ matrix.workload }}")),
			stepsJob("bench_select", nil, "app", "", map[string]string{"workload": "select"},
				deploymentbuilder.CallCmdStep("bench_select/0", 0, "run ${{ matrix.workload }}")),
		},
	}
}

func buildTestInput() *ExecuteCompiledPlanInput {
	return &ExecuteCompiledPlanInput{
		Plan: buildTestPlan(),
		Machines: map[string][]*deploymentpb.MachineState{
			"app": {machineState("app-1", "10.0.0.1")},
			"svc": {machineState("svc-1", "10.0.0.2")},
		},
		Bootstrap: &workflowpb.AgentBootstrap{
			AgentTaskQueues: map[string]string{
				"app-1": "tq-app-1",
				"svc-1": "tq-svc-1",
				"gw-1":  "tq-gateway",
			},
		},
		GatewayNodeID: "gw-1",
	}
}

func TestExecuteCompiledPlanWorkflow(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env, DefaultOptions())
	registerCompiledPlanActivityStubs(env)

	rec := &recorder{}
	var nomadCalls int
	var lastJobJSON []byte
	var nomadMu sync.Mutex

	env.OnActivity(workflowpb.EnsureAgentOnlineActivityActivityName, mock.Anything).Return(nil)

	env.OnActivity(workflowpb.CallCmdActivityActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, cmd *common.Cmd) (*common.Cmd_Result, error) {
			text := cmd.GetSpec().GetScript().GetText()
			rec.add("cmd:" + text)
			// Record which task queue this activity was dispatched on.
			info := activity.GetInfo(ctx)
			rec.addQueueCheck(workflowpb.CallCmdActivityActivityName, info.TaskQueue)
			if strings.Contains(text, "echo c") {
				return &common.Cmd_Result{ExitCode: 1, Stderr: []byte("boom")}, nil
			}
			return &common.Cmd_Result{ExitCode: 0}, nil
		},
	)

	env.OnActivity(NomadSubmitJobActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, in *stroppyagent.NomadSubmitJobInput) (*stroppyagent.NomadSubmitJobOutput, error) {
			nomadMu.Lock()
			nomadCalls++
			lastJobJSON = in.JobJSON
			nomadMu.Unlock()
			rec.add("nomad:echo")
			// Record which task queue this activity was dispatched on.
			info := activity.GetInfo(ctx)
			rec.addQueueCheck(NomadSubmitJobActivityName, info.TaskQueue)
			return &stroppyagent.NomadSubmitJobOutput{JobID: "echo", EvalID: "eval-1"}, nil
		},
	)

	env.ExecuteWorkflow(ExecuteCompiledPlanWorkflowName, buildTestInput())

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	var resp ExecuteCompiledPlanOutput
	if err := env.GetWorkflowResult(&resp); err != nil {
		t.Fatalf("get workflow result: %v", err)
	}

	want := map[string]string{
		"a":            jobStatusOK,
		"b":            jobStatusOK,
		"c":            jobStatusFailed,
		"d":            jobStatusSkipped,
		"e":            jobStatusSkipped,
		"f":            jobStatusOK,
		"bench_insert": jobStatusOK,
		"bench_select": jobStatusOK,
	}
	for id, wantStatus := range want {
		if got := resp.JobStatuses[id]; got != wantStatus {
			t.Errorf("job %q status = %q, want %q", id, got, wantStatus)
		}
	}
	if len(resp.JobStatuses) != len(want) {
		t.Errorf("job statuses = %v, want exactly %v", resp.JobStatuses, want)
	}

	// b invoked NomadSubmitJobActivity exactly once.
	if nomadCalls != 1 {
		t.Fatalf("nomad submit calls = %d, want 1", nomadCalls)
	}

	// c failed -> d is cascade-skipped: its step must never have run.
	if rec.contains("cmd:echo d") {
		t.Fatal("job d ran its step despite a failed dependency (c)")
	}
	// e's when=false skip must not have run its step either.
	if rec.contains("cmd:echo e") {
		t.Fatal("job e ran its step despite when=false")
	}
	// f (needs e, when-skipped) DID run - when-skip counts as satisfied.
	if !rec.contains("cmd:echo f") {
		t.Fatal("job f (dependent of a when-skipped job) did not run")
	}

	// Ordering: a's step precedes both the nomad submit (b) and c's step,
	// which in turn precede nothing from d (d never ran at all - see above).
	aIdx := rec.indexOf("cmd:echo a")
	nomadIdx := rec.indexOf("nomad:echo")
	cIdx := rec.indexOf("cmd:echo c")
	if aIdx < 0 || nomadIdx < 0 || cIdx < 0 {
		t.Fatalf("expected events missing from recorder: %v", rec.events)
	}
	if aIdx > nomadIdx || aIdx > cIdx {
		t.Fatalf("job a did not precede its dependents b/c: events = %v", rec.events)
	}

	// Matrix instances: two distinct executions, each resolving its own
	// ${{ matrix.workload }} value.
	if !rec.contains("cmd:run insert") {
		t.Fatal("bench_insert did not resolve matrix.workload to \"insert\"")
	}
	if !rec.contains("cmd:run select") {
		t.Fatal("bench_select did not resolve matrix.workload to \"select\"")
	}

	// The service job's env ${{ }} resolved against the real machine IP from
	// MachineState (not the compile-time placeholder).
	var nomadJob api.Job
	if err := json.Unmarshal(lastJobJSON, &nomadJob); err != nil {
		t.Fatalf("unmarshal submitted nomad job: %v", err)
	}
	if len(nomadJob.TaskGroups) != 1 || len(nomadJob.TaskGroups[0].Tasks) != 1 {
		t.Fatalf("unexpected nomad job shape: %+v", nomadJob)
	}
	task := nomadJob.TaskGroups[0].Tasks[0]
	if got := task.Env["NODE_IP"]; got != "10.0.0.2" {
		t.Fatalf("service env NODE_IP = %q, want %q", got, "10.0.0.2")
	}
	if got := task.Env["EXTRA"]; got != "1" {
		t.Fatalf("service env EXTRA (from job.with) = %q, want %q", got, "1")
	}

	// Assert task queue routing: every activity must have been dispatched to the
	// correct queue based on which node/machine it ran on.
	// - Steps jobs (CallCmd activities) dispatch to their node's per-node queue
	// - Service jobs (NomadSubmitJobActivity) dispatch to the gateway node's queue
	callCmdQueues := []string{}
	nomadQueues := []string{}
	for _, qc := range rec.queueChecks {
		if qc.name == workflowpb.CallCmdActivityActivityName {
			callCmdQueues = append(callCmdQueues, qc.taskQueue)
		} else if qc.name == NomadSubmitJobActivityName {
			nomadQueues = append(nomadQueues, qc.taskQueue)
		}
	}

	// All CallCmd activities should run on the "app" node's queue (app-1 -> tq-app-1)
	// Jobs that ran: a, c, f, bench_insert, bench_select (5 total)
	// Job d never ran (skipped due to c failure), job e never ran (when=false)
	expectedCallCmdQueues := 5
	if len(callCmdQueues) != expectedCallCmdQueues {
		t.Errorf("expected %d CallCmd activities, got %d: %v", expectedCallCmdQueues, len(callCmdQueues), callCmdQueues)
	}
	for i, q := range callCmdQueues {
		if q != "tq-app-1" {
			t.Errorf("CallCmd activity %d dispatched to queue %q, want %q", i, q, "tq-app-1")
		}
	}

	// NomadSubmitJobActivity should run exactly once on the gateway node's queue
	if len(nomadQueues) != 1 {
		t.Errorf("expected 1 NomadSubmitJob activity, got %d: %v", len(nomadQueues), nomadQueues)
	}
	if len(nomadQueues) > 0 && nomadQueues[0] != "tq-gateway" {
		t.Errorf("NomadSubmitJob activity dispatched to queue %q, want %q", nomadQueues[0], "tq-gateway")
	}
}

// machineStateWithDiskDevice builds a MachineState whose "private" endpoint
// carries the disk_device label — the convention machineViewFor (dslrun.go)
// reads DiskView.Path from, mirroring how the provider packages
// (terraform.go, docker.go) stamp it after provisioning.
func machineStateWithDiskDevice(nodeID, ip, diskDevice string) *deploymentpb.MachineState {
	return &deploymentpb.MachineState{
		NodeId: nodeID,
		Endpoints: []*deploymentpb.Endpoint{
			{
				Name:    "private",
				Address: ip,
				Labels:  map[string]string{"disk_device": diskDevice},
			},
		},
	}
}

// TestExecuteCompiledPlanWorkflowDiskDevicePath is the I2 regression lock:
// machineViewFor (dslrun.go) used to hardcode DiskView.Path to "" regardless
// of what the provider actually provisioned, so any recipe step interpolating
// ${{ machine.disks[0].path }} (e.g. postgres-ha's prep-disks `mkfs.ext4`
// step) resolved to an empty device — non-functional. This asserts the
// resolved script carries the disk_device value stamped on the machine's
// private endpoint by the provider.
func TestExecuteCompiledPlanWorkflowDiskDevicePath(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env, DefaultOptions())
	registerCompiledPlanActivityStubs(env)

	var capturedScript string
	env.OnActivity(workflowpb.CallCmdActivityActivityName, mock.Anything, mock.Anything).Return(
		func(_ context.Context, cmd *common.Cmd) (*common.Cmd_Result, error) {
			capturedScript = cmd.GetSpec().GetScript().GetText()
			return &common.Cmd_Result{ExitCode: 0}, nil
		},
	)

	plan := &dslpb.CompiledPlan{
		MachineGroups: []*dslpb.MachineGroup{
			{Name: "db", Count: 1, Cpu: 2, RamMb: 2048, Disks: []*dslpb.DiskSpec{{SizeGb: 20}}},
		},
		Jobs: []*dslpb.CompiledJob{
			stepsJob("prep-disks", nil, "db", "", nil,
				deploymentbuilder.CallCmdStep("prep-disks/0", 0, "mkfs.ext4 ${{ machine.disks[0].path }}")),
		},
	}

	input := &ExecuteCompiledPlanInput{
		Plan: plan,
		Machines: map[string][]*deploymentpb.MachineState{
			"db": {machineStateWithDiskDevice("db-1", "10.0.0.5", "/dev/vdb")},
		},
		Bootstrap: &workflowpb.AgentBootstrap{
			AgentTaskQueues: map[string]string{"db-1": "tq-db-1"},
		},
	}

	env.ExecuteWorkflow(ExecuteCompiledPlanWorkflowName, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	if !strings.Contains(capturedScript, "/dev/vdb") {
		t.Fatalf("resolved mkfs script = %q, want it to contain the provisioned disk device %q (not empty)", capturedScript, "/dev/vdb")
	}
}

// TestExecuteCompiledPlanWorkflowInputsGroupBinding is the I1 closure lock
// for machine_group-typed inputs: a job whose CompiledJob.InputGroups maps
// the component's "nodes" input to the plan's "db" machine group must
// resolve `${{ inputs.nodes.machines[0].ip }}` to db's real provisioned IP —
// exactly what graph.Validate typechecked `inputs.nodes` as (a
// MachineGroupView) against expr.ComponentEnv, closing the gap where
// baseJobVars never bound `inputs` at all.
func TestExecuteCompiledPlanWorkflowInputsGroupBinding(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env, DefaultOptions())
	registerCompiledPlanActivityStubs(env)

	var capturedScript string
	env.OnActivity(workflowpb.CallCmdActivityActivityName, mock.Anything, mock.Anything).Return(
		func(_ context.Context, cmd *common.Cmd) (*common.Cmd_Result, error) {
			capturedScript = cmd.GetSpec().GetScript().GetText()
			return &common.Cmd_Result{ExitCode: 0}, nil
		},
	)

	plan := &dslpb.CompiledPlan{
		MachineGroups: []*dslpb.MachineGroup{
			{Name: "db", Count: 1, Cpu: 2, RamMb: 2048},
		},
		Jobs: []*dslpb.CompiledJob{
			{
				Id:          "use-nodes",
				OnGroup:     "db",
				InputGroups: map[string]string{"nodes": "db"},
				Action: &dslpb.CompiledJob_Steps{Steps: &dslpb.StepList{Steps: []*dslpb.DslStep{
					{Step: &dslpb.DslStep_Agent{Agent: deploymentbuilder.CallCmdStep(
						"use-nodes/0", 0, "echo ${{ inputs.nodes.machines[0].ip }}",
					)}},
				}}},
			},
		},
	}

	input := &ExecuteCompiledPlanInput{
		Plan: plan,
		Machines: map[string][]*deploymentpb.MachineState{
			"db": {machineState("db-1", "10.0.0.7")},
		},
		Bootstrap: &workflowpb.AgentBootstrap{
			AgentTaskQueues: map[string]string{"db-1": "tq-db-1"},
		},
	}

	env.ExecuteWorkflow(ExecuteCompiledPlanWorkflowName, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	var resp ExecuteCompiledPlanOutput
	if err := env.GetWorkflowResult(&resp); err != nil {
		t.Fatalf("get workflow result: %v", err)
	}
	if got := resp.JobStatuses["use-nodes"]; got != jobStatusOK {
		t.Fatalf("job status = %q, want %q", got, jobStatusOK)
	}

	if !strings.Contains(capturedScript, "10.0.0.7") {
		t.Fatalf("resolved script = %q, want it to contain the input group's real IP %q", capturedScript, "10.0.0.7")
	}
}

// TestExecuteCompiledPlanWorkflowScalarInputInterpolation locks the scalar
// half of I1: CompiledJob.ResolvedInputs binds `inputs.<name>` as a string,
// so `${{ inputs.count }}` interpolation resolves — but (documented
// limitation, see baseJobVars) a `when:` comparing a scalar input against a
// number is NOT exercised here because it does not work at runtime (CEL
// compares the bound string to an int and errors); this test only asserts
// the interpolation case that graph.Validate and the runtime actually agree
// on.
func TestExecuteCompiledPlanWorkflowScalarInputInterpolation(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env, DefaultOptions())
	registerCompiledPlanActivityStubs(env)

	var capturedScript string
	env.OnActivity(workflowpb.CallCmdActivityActivityName, mock.Anything, mock.Anything).Return(
		func(_ context.Context, cmd *common.Cmd) (*common.Cmd_Result, error) {
			capturedScript = cmd.GetSpec().GetScript().GetText()
			return &common.Cmd_Result{ExitCode: 0}, nil
		},
	)

	plan := &dslpb.CompiledPlan{
		MachineGroups: []*dslpb.MachineGroup{
			{Name: "app", Count: 1, Cpu: 2, RamMb: 2048},
		},
		Jobs: []*dslpb.CompiledJob{
			{
				Id:             "use-count",
				OnGroup:        "app",
				ResolvedInputs: map[string]string{"count": "3"},
				Action: &dslpb.CompiledJob_Steps{Steps: &dslpb.StepList{Steps: []*dslpb.DslStep{
					{Step: &dslpb.DslStep_Agent{Agent: deploymentbuilder.CallCmdStep(
						"use-count/0", 0, "echo ${{ inputs.count }}",
					)}},
				}}},
			},
		},
	}

	input := &ExecuteCompiledPlanInput{
		Plan: plan,
		Machines: map[string][]*deploymentpb.MachineState{
			"app": {machineState("app-1", "10.0.0.1")},
		},
		Bootstrap: &workflowpb.AgentBootstrap{
			AgentTaskQueues: map[string]string{"app-1": "tq-app-1"},
		},
	}

	env.ExecuteWorkflow(ExecuteCompiledPlanWorkflowName, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	var resp ExecuteCompiledPlanOutput
	if err := env.GetWorkflowResult(&resp); err != nil {
		t.Fatalf("get workflow result: %v", err)
	}
	if got := resp.JobStatuses["use-count"]; got != jobStatusOK {
		t.Fatalf("job status = %q, want %q", got, jobStatusOK)
	}

	if !strings.Contains(capturedScript, "echo 3") {
		t.Fatalf("resolved script = %q, want it to contain the interpolated scalar input %q", capturedScript, "echo 3")
	}
}

// TestExecuteCompiledPlanWorkflowTargetBinding locks the `target` half of I1:
// CompiledJob.TargetGroup binds `target` to that group's MachineGroupView —
// `${{ target.count }}` resolves to the real number of provisioned machines
// in the group.
func TestExecuteCompiledPlanWorkflowTargetBinding(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env, DefaultOptions())
	registerCompiledPlanActivityStubs(env)

	var capturedScript string
	env.OnActivity(workflowpb.CallCmdActivityActivityName, mock.Anything, mock.Anything).Return(
		func(_ context.Context, cmd *common.Cmd) (*common.Cmd_Result, error) {
			capturedScript = cmd.GetSpec().GetScript().GetText()
			return &common.Cmd_Result{ExitCode: 0}, nil
		},
	)

	plan := &dslpb.CompiledPlan{
		MachineGroups: []*dslpb.MachineGroup{
			{Name: "db", Count: 3, Cpu: 2, RamMb: 2048},
		},
		Jobs: []*dslpb.CompiledJob{
			{
				Id:          "use-target",
				OnGroup:     "db",
				TargetGroup: "db",
				Action: &dslpb.CompiledJob_Steps{Steps: &dslpb.StepList{Steps: []*dslpb.DslStep{
					{Step: &dslpb.DslStep_Agent{Agent: deploymentbuilder.CallCmdStep(
						"use-target/0", 0, "echo ${{ target.count }}",
					)}},
				}}},
			},
		},
	}

	input := &ExecuteCompiledPlanInput{
		Plan: plan,
		Machines: map[string][]*deploymentpb.MachineState{
			"db": {
				machineState("db-1", "10.0.0.10"),
				machineState("db-2", "10.0.0.11"),
				machineState("db-3", "10.0.0.12"),
			},
		},
		Bootstrap: &workflowpb.AgentBootstrap{
			AgentTaskQueues: map[string]string{
				"db-1": "tq-db-1",
				"db-2": "tq-db-1",
				"db-3": "tq-db-1",
			},
		},
	}

	env.ExecuteWorkflow(ExecuteCompiledPlanWorkflowName, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	var resp ExecuteCompiledPlanOutput
	if err := env.GetWorkflowResult(&resp); err != nil {
		t.Fatalf("get workflow result: %v", err)
	}
	if got := resp.JobStatuses["use-target"]; got != jobStatusOK {
		t.Fatalf("job status = %q, want %q", got, jobStatusOK)
	}

	if !strings.Contains(capturedScript, "echo 3") {
		t.Fatalf("resolved script = %q, want it to contain the target group's real count %q", capturedScript, "echo 3")
	}
}

// TestExecuteCompiledPlanWorkflowMissingInputEvalError confirms the
// existing when/eval error path handles a `${{ inputs.missing }}` reference
// to an input the job's ResolvedInputs does not carry gracefully: the job
// fails (its status is "failed"), the workflow itself completes without
// panicking.
func TestExecuteCompiledPlanWorkflowMissingInputEvalError(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env, DefaultOptions())
	registerCompiledPlanActivityStubs(env)

	env.OnActivity(workflowpb.CallCmdActivityActivityName, mock.Anything, mock.Anything).Return(
		&common.Cmd_Result{ExitCode: 0}, nil,
	)

	plan := &dslpb.CompiledPlan{
		MachineGroups: []*dslpb.MachineGroup{
			{Name: "app", Count: 1, Cpu: 2, RamMb: 2048},
		},
		Jobs: []*dslpb.CompiledJob{
			{
				Id:      "use-missing",
				OnGroup: "app",
				Action: &dslpb.CompiledJob_Steps{Steps: &dslpb.StepList{Steps: []*dslpb.DslStep{
					{Step: &dslpb.DslStep_Agent{Agent: deploymentbuilder.CallCmdStep(
						"use-missing/0", 0, "echo ${{ inputs.missing }}",
					)}},
				}}},
			},
		},
	}

	input := &ExecuteCompiledPlanInput{
		Plan: plan,
		Machines: map[string][]*deploymentpb.MachineState{
			"app": {machineState("app-1", "10.0.0.1")},
		},
		Bootstrap: &workflowpb.AgentBootstrap{
			AgentTaskQueues: map[string]string{"app-1": "tq-app-1"},
		},
	}

	env.ExecuteWorkflow(ExecuteCompiledPlanWorkflowName, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow itself must not error on a job-level eval failure: %v", err)
	}

	var resp ExecuteCompiledPlanOutput
	if err := env.GetWorkflowResult(&resp); err != nil {
		t.Fatalf("get workflow result: %v", err)
	}
	if got := resp.JobStatuses["use-missing"]; got != jobStatusFailed {
		t.Fatalf("job status = %q, want %q (missing input reference must fail the job, not panic the workflow)", got, jobStatusFailed)
	}
}
