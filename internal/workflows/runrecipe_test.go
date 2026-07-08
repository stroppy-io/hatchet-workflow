package workflows

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domainpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

// --- stub activity functions (real signatures, for env registration) -------
//
// CompileRecipeActivity/ProvisionActivity/TeardownActivity have no real
// implementation yet (Task 4 provides it) — env.OnActivity's string branch
// panics unless the name is already registered with a real Go func (mirrors
// dslrun_test.go's registerCompiledPlanActivityStubs for the same reason), so
// each is registered here under its production name with a tiny stub body
// every test's env.OnActivity mock always intercepts before it runs.

func stubCompileRecipeActivity(context.Context, *CompileRecipeActivityInput) (*CompileRecipeActivityOutput, error) {
	return nil, nil
}

func stubProvisionActivity(context.Context, *ProvisionActivityInput) (*ProvisionActivityOutput, error) {
	return nil, nil
}

func stubTeardownActivity(context.Context, *TeardownActivityInput) error { return nil }

func stubReserveQuotasActivity(context.Context, *ReserveQuotasActivityInput) error { return nil }
func stubCommitQuotasActivity(context.Context, *CommitQuotasActivityInput) error   { return nil }
func stubReleaseQuotasActivity(context.Context, *ReleaseQuotasActivityInput) error { return nil }

func registerRunRecipeActivityStubs(env *testsuite.TestWorkflowEnvironment) {
	env.RegisterActivityWithOptions(stubCompileRecipeActivity, activity.RegisterOptions{Name: CompileRecipeActivityName})
	env.RegisterActivityWithOptions(stubProvisionActivity, activity.RegisterOptions{Name: ProvisionActivityName})
	env.RegisterActivityWithOptions(stubTeardownActivity, activity.RegisterOptions{Name: TeardownActivityName})
	env.RegisterActivityWithOptions(stubReserveQuotasActivity, activity.RegisterOptions{Name: ReserveQuotasActivityName})
	env.RegisterActivityWithOptions(stubCommitQuotasActivity, activity.RegisterOptions{Name: CommitQuotasActivityName})
	env.RegisterActivityWithOptions(stubReleaseQuotasActivity, activity.RegisterOptions{Name: ReleaseQuotasActivityName})
}

func runRecipeTestPlan() *dslpb.CompiledPlan {
	return &dslpb.CompiledPlan{
		Provider: &dslpb.ProviderRef{Name: "docker"},
		MachineGroups: []*dslpb.MachineGroup{
			{Name: "app", Count: 1, Cpu: 2, RamMb: 2048},
		},
		Jobs: []*dslpb.CompiledJob{
			stepsJob("a", nil, "app", "", nil, deploymentbuilder.CallCmdStep("a/0", 0, "echo a")),
		},
	}
}

func runRecipeTestInput() *RunRecipeInput {
	return &RunRecipeInput{
		RunID:    "run-1",
		TenantID: "tenant-1",
		Bundle: map[string][]byte{
			"cluster.yaml": []byte("provider:\n  use: docker\n"),
		},
		Bootstrap: &workflowpb.AgentBootstrap{
			AgentTaskQueues: map[string]string{"app-1": "tq-app-1"},
		},
	}
}

// TestRunRecipeWorkflowHappyPath asserts the full compile -> infra -> execute
// -> teardown pipeline: every stage runs in order, the execute child gets the
// provisioned machines and a resolved gateway node id, teardown runs on
// success, and the exposed GetRunState query reports every stage COMPLETED.
func TestRunRecipeWorkflowHappyPath(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env)
	registerRunRecipeActivityStubs(env)
	runtime := &fakeRuntimeActivities{}
	registerFakeRuntimeActivities(env, runtime)

	plan := runRecipeTestPlan()
	machines := map[string][]*deploymentpb.MachineState{
		"app": {machineState("app-1", "10.0.0.1")},
	}

	rec := &recorder{}

	env.OnActivity(CompileRecipeActivityName, mock.Anything, mock.Anything).Return(
		func(_ context.Context, in *CompileRecipeActivityInput) (*CompileRecipeActivityOutput, error) {
			rec.add("compile")
			if len(in.Bundle) == 0 {
				t.Error("compile activity received an empty bundle")
			}
			return &CompileRecipeActivityOutput{Plan: plan}, nil
		},
	)
	env.OnActivity(ProvisionActivityName, mock.Anything, mock.Anything).Return(
		func(_ context.Context, in *ProvisionActivityInput) (*ProvisionActivityOutput, error) {
			rec.add("provision")
			if got := in.ProviderRef.GetName(); got != "docker" {
				t.Errorf("provision providerRef.name = %q, want %q", got, "docker")
			}
			if len(in.Groups) != 1 || in.Groups[0].GetName() != "app" {
				t.Errorf("provision groups = %v, want the plan's machine_groups", in.Groups)
			}
			return &ProvisionActivityOutput{Machines: machines}, nil
		},
	)
	env.OnWorkflow(ExecuteCompiledPlanWorkflowName, mock.Anything, mock.Anything).Return(
		func(_ workflow.Context, in *ExecuteCompiledPlanInput) (*ExecuteCompiledPlanOutput, error) {
			rec.add("execute")
			if in.GatewayNodeID != "app-1" {
				t.Errorf("gateway node id = %q, want %q", in.GatewayNodeID, "app-1")
			}
			if len(in.Machines["app"]) != 1 || in.Machines["app"][0].GetNodeId() != "app-1" {
				t.Errorf("child machines = %v, want the provisioned machines", in.Machines)
			}
			return &ExecuteCompiledPlanOutput{JobStatuses: map[string]string{"a": jobStatusOK}}, nil
		},
	)
	env.OnActivity(TeardownActivityName, mock.Anything, mock.Anything).Return(
		func(_ context.Context, in *TeardownActivityInput) error {
			rec.add("teardown")
			if got := in.ProviderRef.GetName(); got != "docker" {
				t.Errorf("teardown providerRef.name = %q, want %q", got, "docker")
			}
			return nil
		},
	)

	env.ExecuteWorkflow(RunRecipeWorkflowName, runRecipeTestInput())

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	var out RunRecipeOutput
	if err := env.GetWorkflowResult(&out); err != nil {
		t.Fatalf("get workflow result: %v", err)
	}
	if out.Status != "completed" {
		t.Errorf("status = %q, want %q", out.Status, "completed")
	}
	if got := out.JobStatuses["a"]; got != jobStatusOK {
		t.Errorf("job statuses = %v, want a=%q", out.JobStatuses, jobStatusOK)
	}

	if !rec.contains("compile") || !rec.contains("provision") || !rec.contains("execute") || !rec.contains("teardown") {
		t.Fatalf("expected compile/provision/execute/teardown to all run: %v", rec.events)
	}
	if rec.indexOf("compile") > rec.indexOf("provision") ||
		rec.indexOf("provision") > rec.indexOf("execute") ||
		rec.indexOf("execute") > rec.indexOf("teardown") {
		t.Fatalf("stages ran out of order: %v", rec.events)
	}

	value, err := env.QueryWorkflow(workflowpb.GetRunStateQueryName)
	if err != nil {
		t.Fatalf("query run state: %v", err)
	}
	var state workflowpb.RunState
	if err := value.Get(&state); err != nil {
		t.Fatalf("decode run state: %v", err)
	}
	if got, want := state.GetStatus(), common.Status_STATUS_COMPLETED; got != want {
		t.Fatalf("run state status = %s, want %s", got, want)
	}
	if got, want := len(state.GetStages()), 4; got != want {
		t.Fatalf("run state stage count = %d, want %d", got, want)
	}
	for _, stage := range state.GetStages() {
		if stage.GetStatus() != common.Status_STATUS_COMPLETED {
			t.Fatalf("stage %q status = %s, want COMPLETED", stage.GetName(), stage.GetStatus())
		}
	}
}

// TestRunRecipeWorkflowPersistsSummaryFromCompiledPlan asserts that right
// after the compile stage succeeds, RunRecipeWorkflow calls
// PersistRunSummaryActivityName with a Summary derived from the compiled
// plan (see deriveRunSummary in runrecipe_summary.go) — the ROOT fix for
// rating/metrics/compare/share/dashboard, all of which read
// TestRunRecord.Summary rather than the (absent, for a recipe run) baked
// domain.TestRun spec.
func TestRunRecipeWorkflowPersistsSummaryFromCompiledPlan(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env)
	registerRunRecipeActivityStubs(env)
	runtime := &fakeRuntimeActivities{}
	registerFakeRuntimeActivities(env, runtime)

	plan := postgresHaTestPlan()
	machines := map[string][]*deploymentpb.MachineState{
		"db":     {machineState("db-1", "10.0.0.1"), machineState("db-2", "10.0.0.2"), machineState("db-3", "10.0.0.3")},
		"runner": {machineState("runner-1", "10.0.0.4")},
	}

	env.OnActivity(CompileRecipeActivityName, mock.Anything, mock.Anything).Return(
		&CompileRecipeActivityOutput{Plan: plan}, nil,
	)
	env.OnActivity(ProvisionActivityName, mock.Anything, mock.Anything).Return(
		&ProvisionActivityOutput{Machines: machines}, nil,
	)
	env.OnWorkflow(ExecuteCompiledPlanWorkflowName, mock.Anything, mock.Anything).Return(
		&ExecuteCompiledPlanOutput{JobStatuses: map[string]string{"bench[workload=insert]": jobStatusOK, "bench[workload=select]": jobStatusOK}}, nil,
	)
	env.OnActivity(TeardownActivityName, mock.Anything, mock.Anything).Return(nil)

	env.ExecuteWorkflow(RunRecipeWorkflowName, runRecipeTestInput())

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	runtime.mu.Lock()
	summaries := runtime.summaries
	runtime.mu.Unlock()

	if len(summaries) == 0 {
		t.Fatal("PersistRunSummaryActivityName was never called")
	}
	summary := summaries[len(summaries)-1]
	if summary == nil {
		t.Fatal("persisted summary is nil")
	}
	if got, want := summary.GetProvider(), deploymentpb.Provider_PROVIDER_YANDEX; got != want {
		t.Errorf("provider = %s, want %s", got, want)
	}
	if got, want := summary.GetNodeCount(), uint32(4); got != want {
		t.Errorf("node_count = %d, want %d", got, want)
	}
	if got, want := summary.GetDbKind(), domainpb.Database_KIND_POSTGRES; got != want {
		t.Errorf("db_kind = %s, want %s — monitoring.dbKindString now resolves the real metrics kind instead of defaulting", got, want)
	}
	if got, want := summary.GetWorkloadName(), "insert+select"; got != want {
		t.Errorf("workload_name = %q, want %q", got, want)
	}
	if got, want := summary.GetStroppyVersion(), "1.2.3"; got != want {
		t.Errorf("stroppy_version = %q, want %q", got, want)
	}
	if got, want := summary.GetTopologyLabel(), "yandex · 4 nodes"; got != want {
		t.Errorf("topology_label = %q, want %q", got, want)
	}
}

// TestRunRecipeWorkflowPersistsRecipeTopologyAfterProvision asserts that
// right after ProvisionActivity succeeds, RunRecipeWorkflow calls
// PersistRecipeTopologyActivityName with a snapshot derived from the
// compiled plan's machine_groups/services and the provisioned machines (see
// deriveRecipeTopology in runrecipe_topology.go) — the ROOT fix for
// recipe-run topology, which otherwise renders only the lone control-plane
// node (overview.go/runtime_topology.go project this snapshot for a recipe
// run in place of the classic TopologySpec/InfrastructureState/
// DeploymentPlan trio).
func TestRunRecipeWorkflowPersistsRecipeTopologyAfterProvision(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env)
	registerRunRecipeActivityStubs(env)
	runtime := &fakeRuntimeActivities{}
	registerFakeRuntimeActivities(env, runtime)

	plan := postgresHaTestPlan()
	machines := map[string][]*deploymentpb.MachineState{
		"db":     {machineState("db-1", "10.0.0.1"), machineState("db-2", "10.0.0.2"), machineState("db-3", "10.0.0.3")},
		"runner": {machineState("runner-1", "10.0.0.4")},
	}

	env.OnActivity(CompileRecipeActivityName, mock.Anything, mock.Anything).Return(
		&CompileRecipeActivityOutput{Plan: plan}, nil,
	)
	env.OnActivity(ProvisionActivityName, mock.Anything, mock.Anything).Return(
		&ProvisionActivityOutput{Machines: machines}, nil,
	)
	env.OnWorkflow(ExecuteCompiledPlanWorkflowName, mock.Anything, mock.Anything).Return(
		&ExecuteCompiledPlanOutput{JobStatuses: map[string]string{"bench[workload=insert]": jobStatusOK, "bench[workload=select]": jobStatusOK}}, nil,
	)
	env.OnActivity(TeardownActivityName, mock.Anything, mock.Anything).Return(nil)

	env.ExecuteWorkflow(RunRecipeWorkflowName, runRecipeTestInput())

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	runtime.mu.Lock()
	snapshots := runtime.recipeTopologies
	runtime.mu.Unlock()

	if len(snapshots) == 0 {
		t.Fatal("PersistRecipeTopologyActivityName was never called")
	}
	snap := snapshots[len(snapshots)-1]
	if snap == nil {
		t.Fatal("persisted recipe topology snapshot is nil")
	}
	if got, want := snap.GetProvider(), "yandex"; got != want {
		t.Errorf("provider = %q, want %q", got, want)
	}
	nodes := snap.GetNodes()
	if got, want := len(nodes), 4; got != want {
		t.Fatalf("node count = %d, want %d", got, want)
	}
	var sawDB, sawRunner bool
	for _, node := range nodes {
		switch node.GetGroup() {
		case "db":
			sawDB = true
		case "runner":
			sawRunner = true
			if len(node.GetServices()) == 0 {
				t.Errorf("runner node %q has no services, want the stroppy service", node.GetNodeId())
			}
		}
	}
	if !sawDB || !sawRunner {
		t.Fatalf("expected both db and runner groups represented in nodes: %v", nodes)
	}
}

// TestRunRecipeWorkflowPersistsCompiledPlanAfterCompile asserts that right
// after the compile stage succeeds, RunRecipeWorkflow calls
// PersistRunCompiledPlanActivityName with the real compiled plan — the ROOT
// fix for Run.compiled_plan, which before Task 3 lived only in workflow
// memory and was never durably stored (see runtime.go's
// persistRunCompiledPlan). It must persist no later than the summary (both
// happen back-to-back in the same post-compile block — see runrecipe.go's
// run — this task's ordering choice).
func TestRunRecipeWorkflowPersistsCompiledPlanAfterCompile(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env)
	registerRunRecipeActivityStubs(env)
	runtime := &fakeRuntimeActivities{}
	registerFakeRuntimeActivities(env, runtime)

	plan := runRecipeTestPlan()
	machines := map[string][]*deploymentpb.MachineState{
		"app": {machineState("app-1", "10.0.0.1")},
	}

	env.OnActivity(CompileRecipeActivityName, mock.Anything, mock.Anything).Return(
		&CompileRecipeActivityOutput{Plan: plan}, nil,
	)
	env.OnActivity(ProvisionActivityName, mock.Anything, mock.Anything).Return(
		&ProvisionActivityOutput{Machines: machines}, nil,
	)
	env.OnWorkflow(ExecuteCompiledPlanWorkflowName, mock.Anything, mock.Anything).Return(
		&ExecuteCompiledPlanOutput{JobStatuses: map[string]string{"a": jobStatusOK}}, nil,
	)
	env.OnActivity(TeardownActivityName, mock.Anything, mock.Anything).Return(nil)

	env.ExecuteWorkflow(RunRecipeWorkflowName, runRecipeTestInput())

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	runtime.mu.Lock()
	persisted := runtime.persistedCompiledPlan
	planAt := runtime.persistedCompiledPlanAt
	summaryAt := runtime.persistedSummaryAt
	callCount := len(runtime.compiledPlans)
	runtime.mu.Unlock()

	if persisted == nil {
		t.Fatal("persistRunCompiledPlan must be called after compile succeeds")
	}
	if persisted.GetProvider().GetName() == "" {
		t.Fatal("persisted plan is the real compiled plan, not empty")
	}
	if planAt == 0 {
		t.Fatal("persistedCompiledPlanAt was never recorded")
	}
	if planAt > summaryAt {
		t.Fatalf("compiled plan persisted after the summary: planAt=%d, summaryAt=%d", planAt, summaryAt)
	}
	// Regression guard for the Temporal history-bloat bug (see
	// project_temporal_history_bloat notes / task-3-report.md): the compiled
	// plan must be persisted exactly ONCE per successful run. If a future
	// change re-wires this call into persist()'s per-tick loop (which runs on
	// every RunState update), this call count would silently grow past 1 and
	// flood workflow history the same way the old plan-resend bug did.
	if callCount != 1 {
		t.Fatalf("PersistRunCompiledPlan called %d times, want exactly 1 (guards against re-wiring into persist()'s per-tick loop)", callCount)
	}
}

// TestRunRecipeWorkflowProvisionFails asserts a ProvisionActivity failure
// marks the infra stage FAILED, skips the execute child entirely, still runs
// teardown (compile already resolved a provider), and the workflow itself
// completes without a Go error (activity failure surfaces as
// RunRecipeOutput.Status, not a workflow error — see runrecipe.go's package
// doc).
func TestRunRecipeWorkflowProvisionFails(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env)
	registerRunRecipeActivityStubs(env)
	runtime := &fakeRuntimeActivities{}
	registerFakeRuntimeActivities(env, runtime)

	plan := runRecipeTestPlan()
	var executeCalled, teardownCalled bool

	env.OnActivity(CompileRecipeActivityName, mock.Anything, mock.Anything).Return(
		&CompileRecipeActivityOutput{Plan: plan}, nil,
	)
	env.OnActivity(ProvisionActivityName, mock.Anything, mock.Anything).Return(
		nil, errors.New("provision boom"),
	)
	env.OnWorkflow(ExecuteCompiledPlanWorkflowName, mock.Anything, mock.Anything).Return(
		func(_ workflow.Context, _ *ExecuteCompiledPlanInput) (*ExecuteCompiledPlanOutput, error) {
			executeCalled = true
			return &ExecuteCompiledPlanOutput{}, nil
		},
	)
	env.OnActivity(TeardownActivityName, mock.Anything, mock.Anything).Return(
		func(_ context.Context, _ *TeardownActivityInput) error {
			teardownCalled = true
			return nil
		},
	)

	env.ExecuteWorkflow(RunRecipeWorkflowName, runRecipeTestInput())

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow returned a Go error, want the failure reported via output.Status instead: %v", err)
	}

	var out RunRecipeOutput
	if err := env.GetWorkflowResult(&out); err != nil {
		t.Fatalf("get workflow result: %v", err)
	}
	if out.Status != "failed" {
		t.Errorf("status = %q, want %q", out.Status, "failed")
	}
	if executeCalled {
		t.Fatal("execute child ran despite the provision failure")
	}
	if !teardownCalled {
		t.Fatal("teardown did not run after the provision failure")
	}

	value, err := env.QueryWorkflow(workflowpb.GetRunStateQueryName)
	if err != nil {
		t.Fatalf("query run state: %v", err)
	}
	var state workflowpb.RunState
	if err := value.Get(&state); err != nil {
		t.Fatalf("decode run state: %v", err)
	}
	if got := state.GetStages()[runRecipeStageCompileIndex].GetStatus(); got != common.Status_STATUS_COMPLETED {
		t.Fatalf("compile stage status = %s, want COMPLETED", got)
	}
	if got := state.GetStages()[runRecipeStageInfraIndex].GetStatus(); got != common.Status_STATUS_FAILED {
		t.Fatalf("infra stage status = %s, want FAILED", got)
	}
	if got := state.GetStages()[runRecipeStageExecuteIndex].GetStatus(); got != common.Status_STATUS_PENDING {
		t.Fatalf("execute stage status = %s, want PENDING (never started)", got)
	}
	if got := state.GetStages()[runRecipeStageTeardownIndex].GetStatus(); got != common.Status_STATUS_COMPLETED {
		t.Fatalf("teardown stage status = %s, want COMPLETED", got)
	}
}

// TestRunRecipeWorkflowCompileFails asserts a CompileRecipeActivity result
// carrying an error diagnostic marks the compile stage FAILED, skips both
// provision and execute entirely, and never calls TeardownActivity (no
// provider was ever resolved, so there is nothing to tear down) — yet the
// teardown STAGE still completes as a documented no-op.
func TestRunRecipeWorkflowCompileFails(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env)
	registerRunRecipeActivityStubs(env)
	runtime := &fakeRuntimeActivities{}
	registerFakeRuntimeActivities(env, runtime)

	var provisionCalled, executeCalled, teardownCalled bool

	env.OnActivity(CompileRecipeActivityName, mock.Anything, mock.Anything).Return(
		&CompileRecipeActivityOutput{
			Diagnostics: diag.List{{Severity: diag.Error, Path: "cluster.yaml", Message: "boom"}},
		}, nil,
	)
	env.OnActivity(ProvisionActivityName, mock.Anything, mock.Anything).Return(
		func(_ context.Context, _ *ProvisionActivityInput) (*ProvisionActivityOutput, error) {
			provisionCalled = true
			return &ProvisionActivityOutput{}, nil
		},
	)
	env.OnWorkflow(ExecuteCompiledPlanWorkflowName, mock.Anything, mock.Anything).Return(
		func(_ workflow.Context, _ *ExecuteCompiledPlanInput) (*ExecuteCompiledPlanOutput, error) {
			executeCalled = true
			return &ExecuteCompiledPlanOutput{}, nil
		},
	)
	env.OnActivity(TeardownActivityName, mock.Anything, mock.Anything).Return(
		func(_ context.Context, _ *TeardownActivityInput) error {
			teardownCalled = true
			return nil
		},
	)

	env.ExecuteWorkflow(RunRecipeWorkflowName, runRecipeTestInput())

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow returned a Go error, want the failure reported via output.Status instead: %v", err)
	}

	var out RunRecipeOutput
	if err := env.GetWorkflowResult(&out); err != nil {
		t.Fatalf("get workflow result: %v", err)
	}
	if out.Status != "failed" {
		t.Errorf("status = %q, want %q", out.Status, "failed")
	}
	if provisionCalled {
		t.Fatal("provision ran despite the compile failure")
	}
	if executeCalled {
		t.Fatal("execute ran despite the compile failure")
	}
	if teardownCalled {
		t.Fatal("teardown activity ran despite compile never resolving a provider (should be a no-op)")
	}

	value, err := env.QueryWorkflow(workflowpb.GetRunStateQueryName)
	if err != nil {
		t.Fatalf("query run state: %v", err)
	}
	var state workflowpb.RunState
	if err := value.Get(&state); err != nil {
		t.Fatalf("decode run state: %v", err)
	}
	if got := state.GetStages()[runRecipeStageCompileIndex].GetStatus(); got != common.Status_STATUS_FAILED {
		t.Fatalf("compile stage status = %s, want FAILED", got)
	}
	if got := state.GetStages()[runRecipeStageCompileIndex].GetErrorMessage(); got == "" {
		t.Fatal("compile stage has no error message")
	}
	if got := state.GetStages()[runRecipeStageInfraIndex].GetStatus(); got != common.Status_STATUS_PENDING {
		t.Fatalf("infra stage status = %s, want PENDING (never started)", got)
	}
	if got := state.GetStages()[runRecipeStageTeardownIndex].GetStatus(); got != common.Status_STATUS_COMPLETED {
		t.Fatalf("teardown stage status = %s, want COMPLETED (no-op)", got)
	}
}

// TestRunRecipeWorkflowReservesQuotasBeforeProvision asserts ReserveQuotasActivity
// runs in the infra stage, before ProvisionActivity, carrying the compiled
// plan's provider name and machine groups plus the run's identity — see
// runrecipe.go's reserveQuotas and the package doc's quota reserve/commit/
// release hooks.
func TestRunRecipeWorkflowReservesQuotasBeforeProvision(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env)
	registerRunRecipeActivityStubs(env)
	runtime := &fakeRuntimeActivities{}
	registerFakeRuntimeActivities(env, runtime)

	plan := runRecipeTestPlan()
	machines := map[string][]*deploymentpb.MachineState{
		"app": {machineState("app-1", "10.0.0.1")},
	}
	rec := &recorder{}

	env.OnActivity(CompileRecipeActivityName, mock.Anything, mock.Anything).Return(
		&CompileRecipeActivityOutput{Plan: plan}, nil,
	)
	env.OnActivity(ReserveQuotasActivityName, mock.Anything, mock.Anything).Return(
		func(_ context.Context, in *ReserveQuotasActivityInput) error {
			rec.add("reserve")
			if in.TenantID != "tenant-1" {
				t.Errorf("reserve tenant_id = %q, want %q", in.TenantID, "tenant-1")
			}
			if in.RunID != "run-1" {
				t.Errorf("reserve run_id = %q, want %q", in.RunID, "run-1")
			}
			if in.WorkflowID == "" {
				t.Error("reserve workflow_id is empty, want the run-recipe workflow's own id")
			}
			if in.Provider != "docker" {
				t.Errorf("reserve provider = %q, want %q", in.Provider, "docker")
			}
			if len(in.Groups) != 1 || in.Groups[0].GetName() != "app" {
				t.Errorf("reserve groups = %v, want the plan's machine_groups", in.Groups)
			}
			return nil
		},
	)
	env.OnActivity(ProvisionActivityName, mock.Anything, mock.Anything).Return(
		func(_ context.Context, _ *ProvisionActivityInput) (*ProvisionActivityOutput, error) {
			rec.add("provision")
			return &ProvisionActivityOutput{Machines: machines}, nil
		},
	)
	env.OnWorkflow(ExecuteCompiledPlanWorkflowName, mock.Anything, mock.Anything).Return(
		&ExecuteCompiledPlanOutput{JobStatuses: map[string]string{"a": jobStatusOK}}, nil,
	)
	env.OnActivity(TeardownActivityName, mock.Anything, mock.Anything).Return(nil)

	env.ExecuteWorkflow(RunRecipeWorkflowName, runRecipeTestInput())

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}
	if !rec.contains("reserve") || !rec.contains("provision") {
		t.Fatalf("expected both reserve and provision to run: %v", rec.events)
	}
	if rec.indexOf("reserve") > rec.indexOf("provision") {
		t.Fatalf("reserve ran after provision: %v", rec.events)
	}
}

// TestRunRecipeWorkflowOverLimitReserveFailsInfraAndSkipsProvision asserts
// that when ReserveQuotasActivity fails (e.g. the over-limit
// derrors.FailedPrecondition Manager.Reserve returns — see
// internal/infrastructure/quotas/reserve.go), the infra stage is marked
// FAILED, ProvisionActivity is never called, the workflow still completes
// (activity failure surfaces via RunRecipeOutput.Status, not a workflow
// error — see the package doc's "stage failure vs workflow error" note),
// and teardown (with its own release-quotas call) still runs.
func TestRunRecipeWorkflowOverLimitReserveFailsInfraAndSkipsProvision(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env)
	registerRunRecipeActivityStubs(env)
	runtime := &fakeRuntimeActivities{}
	registerFakeRuntimeActivities(env, runtime)

	plan := runRecipeTestPlan()
	var provisionCalled, releaseCalled bool

	env.OnActivity(CompileRecipeActivityName, mock.Anything, mock.Anything).Return(
		&CompileRecipeActivityOutput{Plan: plan}, nil,
	)
	env.OnActivity(ReserveQuotasActivityName, mock.Anything, mock.Anything).Return(
		errors.New("quota host.cpuCores has 1 available, needs 2"),
	)
	env.OnActivity(ProvisionActivityName, mock.Anything, mock.Anything).Return(
		func(_ context.Context, _ *ProvisionActivityInput) (*ProvisionActivityOutput, error) {
			provisionCalled = true
			return &ProvisionActivityOutput{}, nil
		},
	)
	env.OnWorkflow(ExecuteCompiledPlanWorkflowName, mock.Anything, mock.Anything).Return(
		func(_ workflow.Context, _ *ExecuteCompiledPlanInput) (*ExecuteCompiledPlanOutput, error) {
			t.Fatal("execute child ran despite the reserve-quotas failure")
			return nil, nil
		},
	)
	env.OnActivity(TeardownActivityName, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(ReleaseQuotasActivityName, mock.Anything, mock.Anything).Return(
		func(_ context.Context, in *ReleaseQuotasActivityInput) error {
			releaseCalled = true
			if in.RunID != "run-1" {
				t.Errorf("release run_id = %q, want %q", in.RunID, "run-1")
			}
			return nil
		},
	)

	env.ExecuteWorkflow(RunRecipeWorkflowName, runRecipeTestInput())

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow returned a Go error, want the failure reported via output.Status instead: %v", err)
	}

	var out RunRecipeOutput
	if err := env.GetWorkflowResult(&out); err != nil {
		t.Fatalf("get workflow result: %v", err)
	}
	if out.Status != "failed" {
		t.Errorf("status = %q, want %q", out.Status, "failed")
	}
	if provisionCalled {
		t.Fatal("provision ran despite the reserve-quotas failure")
	}
	if !releaseCalled {
		t.Fatal("release-quotas did not run in teardown after the reserve-quotas failure")
	}

	value, err := env.QueryWorkflow(workflowpb.GetRunStateQueryName)
	if err != nil {
		t.Fatalf("query run state: %v", err)
	}
	var state workflowpb.RunState
	if err := value.Get(&state); err != nil {
		t.Fatalf("decode run state: %v", err)
	}
	if got := state.GetStages()[runRecipeStageInfraIndex].GetStatus(); got != common.Status_STATUS_FAILED {
		t.Fatalf("infra stage status = %s, want FAILED", got)
	}
	if got := state.GetStages()[runRecipeStageInfraIndex].GetErrorMessage(); got == "" {
		t.Fatal("infra stage has no error message")
	}
	if got := state.GetStages()[runRecipeStageExecuteIndex].GetStatus(); got != common.Status_STATUS_PENDING {
		t.Fatalf("execute stage status = %s, want PENDING (never started)", got)
	}
}

// TestRunRecipeWorkflowCommitsQuotasAfterExecute asserts CommitQuotasActivity
// runs after ExecuteCompiledPlanWorkflow succeeds, before the execute stage
// is marked COMPLETED — see runrecipe.go's commitQuotas.
func TestRunRecipeWorkflowCommitsQuotasAfterExecute(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env)
	registerRunRecipeActivityStubs(env)
	runtime := &fakeRuntimeActivities{}
	registerFakeRuntimeActivities(env, runtime)

	plan := runRecipeTestPlan()
	machines := map[string][]*deploymentpb.MachineState{
		"app": {machineState("app-1", "10.0.0.1")},
	}
	rec := &recorder{}

	env.OnActivity(CompileRecipeActivityName, mock.Anything, mock.Anything).Return(
		&CompileRecipeActivityOutput{Plan: plan}, nil,
	)
	env.OnActivity(ReserveQuotasActivityName, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(ProvisionActivityName, mock.Anything, mock.Anything).Return(
		&ProvisionActivityOutput{Machines: machines}, nil,
	)
	env.OnWorkflow(ExecuteCompiledPlanWorkflowName, mock.Anything, mock.Anything).Return(
		func(_ workflow.Context, _ *ExecuteCompiledPlanInput) (*ExecuteCompiledPlanOutput, error) {
			rec.add("execute")
			return &ExecuteCompiledPlanOutput{JobStatuses: map[string]string{"a": jobStatusOK}}, nil
		},
	)
	env.OnActivity(CommitQuotasActivityName, mock.Anything, mock.Anything).Return(
		func(_ context.Context, in *CommitQuotasActivityInput) error {
			rec.add("commit")
			if in.RunID != "run-1" {
				t.Errorf("commit run_id = %q, want %q", in.RunID, "run-1")
			}
			return nil
		},
	)
	env.OnActivity(TeardownActivityName, mock.Anything, mock.Anything).Return(nil)

	env.ExecuteWorkflow(RunRecipeWorkflowName, runRecipeTestInput())

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}
	if !rec.contains("execute") || !rec.contains("commit") {
		t.Fatalf("expected both execute and commit to run: %v", rec.events)
	}
	if rec.indexOf("execute") > rec.indexOf("commit") {
		t.Fatalf("commit ran before execute: %v", rec.events)
	}

	value, err := env.QueryWorkflow(workflowpb.GetRunStateQueryName)
	if err != nil {
		t.Fatalf("query run state: %v", err)
	}
	var state workflowpb.RunState
	if err := value.Get(&state); err != nil {
		t.Fatalf("decode run state: %v", err)
	}
	if got := state.GetStages()[runRecipeStageExecuteIndex].GetStatus(); got != common.Status_STATUS_COMPLETED {
		t.Fatalf("execute stage status = %s, want COMPLETED", got)
	}
}

// TestRunRecipeWorkflowCommitQuotasFailureFailsExecuteStage asserts a
// CommitQuotasActivity failure marks the execute stage FAILED (mirroring
// every other activity-failure-in-a-stage path in this workflow) rather than
// silently reporting the run as completed despite never allocating its
// reservation.
func TestRunRecipeWorkflowCommitQuotasFailureFailsExecuteStage(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env)
	registerRunRecipeActivityStubs(env)
	runtime := &fakeRuntimeActivities{}
	registerFakeRuntimeActivities(env, runtime)

	plan := runRecipeTestPlan()
	machines := map[string][]*deploymentpb.MachineState{
		"app": {machineState("app-1", "10.0.0.1")},
	}

	env.OnActivity(CompileRecipeActivityName, mock.Anything, mock.Anything).Return(
		&CompileRecipeActivityOutput{Plan: plan}, nil,
	)
	env.OnActivity(ReserveQuotasActivityName, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(ProvisionActivityName, mock.Anything, mock.Anything).Return(
		&ProvisionActivityOutput{Machines: machines}, nil,
	)
	env.OnWorkflow(ExecuteCompiledPlanWorkflowName, mock.Anything, mock.Anything).Return(
		&ExecuteCompiledPlanOutput{JobStatuses: map[string]string{"a": jobStatusOK}}, nil,
	)
	env.OnActivity(CommitQuotasActivityName, mock.Anything, mock.Anything).Return(
		errors.New("commit boom"),
	)
	env.OnActivity(TeardownActivityName, mock.Anything, mock.Anything).Return(nil)

	env.ExecuteWorkflow(RunRecipeWorkflowName, runRecipeTestInput())

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow returned a Go error, want the failure reported via output.Status instead: %v", err)
	}

	var out RunRecipeOutput
	if err := env.GetWorkflowResult(&out); err != nil {
		t.Fatalf("get workflow result: %v", err)
	}
	if out.Status != "failed" {
		t.Errorf("status = %q, want %q", out.Status, "failed")
	}

	value, err := env.QueryWorkflow(workflowpb.GetRunStateQueryName)
	if err != nil {
		t.Fatalf("query run state: %v", err)
	}
	var state workflowpb.RunState
	if err := value.Get(&state); err != nil {
		t.Fatalf("decode run state: %v", err)
	}
	if got := state.GetStages()[runRecipeStageExecuteIndex].GetStatus(); got != common.Status_STATUS_FAILED {
		t.Fatalf("execute stage status = %s, want FAILED", got)
	}
}

// TestRunRecipeWorkflowReleasesQuotasInTeardownAlways asserts
// ReleaseQuotasActivity runs from the teardown defer on every terminal path
// this table covers — happy path, a provision failure, and a compile
// failure (where reserveQuotas itself never even ran) — mirroring "always"
// in runrecipe.go's releaseQuotas doc comment.
func TestRunRecipeWorkflowReleasesQuotasInTeardownAlways(t *testing.T) {
	cases := []struct {
		name           string
		compileErr     error
		provisionErr   error
		wantWorkflowOK bool
	}{
		{name: "happy path", wantWorkflowOK: true},
		{name: "provision fails", provisionErr: errors.New("provision boom"), wantWorkflowOK: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			env := suite.NewTestWorkflowEnvironment()
			RegisterWorkflows(env)
			registerRunRecipeActivityStubs(env)
			runtime := &fakeRuntimeActivities{}
			registerFakeRuntimeActivities(env, runtime)

			plan := runRecipeTestPlan()
			machines := map[string][]*deploymentpb.MachineState{
				"app": {machineState("app-1", "10.0.0.1")},
			}
			var releaseCalled bool

			env.OnActivity(CompileRecipeActivityName, mock.Anything, mock.Anything).Return(
				&CompileRecipeActivityOutput{Plan: plan}, nil,
			)
			env.OnActivity(ReserveQuotasActivityName, mock.Anything, mock.Anything).Return(nil)
			if tc.provisionErr != nil {
				env.OnActivity(ProvisionActivityName, mock.Anything, mock.Anything).Return(nil, tc.provisionErr)
			} else {
				env.OnActivity(ProvisionActivityName, mock.Anything, mock.Anything).Return(
					&ProvisionActivityOutput{Machines: machines}, nil,
				)
			}
			env.OnWorkflow(ExecuteCompiledPlanWorkflowName, mock.Anything, mock.Anything).Return(
				&ExecuteCompiledPlanOutput{JobStatuses: map[string]string{"a": jobStatusOK}}, nil,
			)
			env.OnActivity(CommitQuotasActivityName, mock.Anything, mock.Anything).Return(nil)
			env.OnActivity(TeardownActivityName, mock.Anything, mock.Anything).Return(nil)
			env.OnActivity(ReleaseQuotasActivityName, mock.Anything, mock.Anything).Return(
				func(_ context.Context, _ *ReleaseQuotasActivityInput) error {
					releaseCalled = true
					return nil
				},
			)

			env.ExecuteWorkflow(RunRecipeWorkflowName, runRecipeTestInput())

			if !env.IsWorkflowCompleted() {
				t.Fatal("workflow did not complete")
			}
			if err := env.GetWorkflowError(); (err == nil) != tc.wantWorkflowOK {
				t.Fatalf("workflow error = %v, want ok=%v", err, tc.wantWorkflowOK)
			}
			if !releaseCalled {
				t.Fatal("release-quotas did not run in teardown")
			}
		})
	}
}

// TestRunRecipeWorkflowReleasesQuotasEvenWhenCompileFails asserts
// ReleaseQuotasActivity still runs from the teardown defer when the compile
// stage itself fails (reserveQuotas was never reached: compile never
// resolved a provider) — the strongest form of "always" in releaseQuotas'
// doc comment, since TeardownActivity itself is a documented no-op on this
// exact path (see TestRunRecipeWorkflowCompileFails above).
func TestRunRecipeWorkflowReleasesQuotasEvenWhenCompileFails(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env)
	registerRunRecipeActivityStubs(env)
	runtime := &fakeRuntimeActivities{}
	registerFakeRuntimeActivities(env, runtime)

	var releaseCalled, teardownCalled bool

	env.OnActivity(CompileRecipeActivityName, mock.Anything, mock.Anything).Return(
		&CompileRecipeActivityOutput{
			Diagnostics: diag.List{{Severity: diag.Error, Path: "cluster.yaml", Message: "boom"}},
		}, nil,
	)
	env.OnActivity(TeardownActivityName, mock.Anything, mock.Anything).Return(
		func(_ context.Context, _ *TeardownActivityInput) error {
			teardownCalled = true
			return nil
		},
	)
	env.OnActivity(ReleaseQuotasActivityName, mock.Anything, mock.Anything).Return(
		func(_ context.Context, in *ReleaseQuotasActivityInput) error {
			releaseCalled = true
			if in.TenantID != "tenant-1" || in.RunID != "run-1" {
				t.Errorf("release input = %+v, want tenant-1/run-1", in)
			}
			return nil
		},
	)

	env.ExecuteWorkflow(RunRecipeWorkflowName, runRecipeTestInput())

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow returned a Go error, want the failure reported via output.Status instead: %v", err)
	}
	if teardownCalled {
		t.Fatal("teardown activity ran despite compile never resolving a provider (should stay a no-op)")
	}
	if !releaseCalled {
		t.Fatal("release-quotas did not run despite the compile-stage failure")
	}
}

// TestRunRecipeWorkflowCanceledDuringProvision asserts that a cancellation
// landing while the workflow is blocked in ProvisionActivity is NOT swallowed
// into an infra-stage failure: the workflow must return a Temporal
// cancellation error (so the server records the execution as Canceled, not
// Completed/Failed), teardown must still run on the disconnected context, and
// the run/stages must be reported CANCELED rather than "failed" — regression
// test for the bug where run()'s compile/provision/execute branches swallowed
// ierr/cerr/eerr into a stage failure and `return nil, nil` even when the
// underlying error was a temporal.CanceledError, which made the deferred
// block's `canceled := temporal.IsCanceledError(err)` check always false
// (err was nil) and let Temporal record the workflow as Completed.
func TestRunRecipeWorkflowCanceledDuringProvision(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env)
	registerRunRecipeActivityStubs(env)
	runtime := &fakeRuntimeActivities{}
	registerFakeRuntimeActivities(env, runtime)

	plan := runRecipeTestPlan()
	var teardownCalled bool

	env.OnActivity(CompileRecipeActivityName, mock.Anything, mock.Anything).Return(
		&CompileRecipeActivityOutput{Plan: plan}, nil,
	)
	// ProvisionActivity blocks (heartbeating, checking ctx.Done()) exactly like
	// the SDK's own testActivityHeartbeat cancellation fixture — mirrors how
	// terraform apply/docker up would block for real until the workflow is
	// canceled mid-provision.
	env.OnActivity(ProvisionActivityName, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, _ *ProvisionActivityInput) (*ProvisionActivityOutput, error) {
			for i := 0; i < 100; i++ {
				activity.RecordHeartbeat(ctx)
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				default:
				}
				time.Sleep(50 * time.Millisecond)
			}
			return &ProvisionActivityOutput{}, nil
		},
	)
	env.OnWorkflow(ExecuteCompiledPlanWorkflowName, mock.Anything, mock.Anything).Return(
		func(_ workflow.Context, _ *ExecuteCompiledPlanInput) (*ExecuteCompiledPlanOutput, error) {
			t.Fatal("execute child ran despite cancellation during provision")
			return nil, nil
		},
	)
	env.OnActivity(TeardownActivityName, mock.Anything, mock.Anything).Return(
		func(_ context.Context, in *TeardownActivityInput) error {
			teardownCalled = true
			if got := in.ProviderRef.GetName(); got != "docker" {
				t.Errorf("teardown providerRef.name = %q, want %q", got, "docker")
			}
			return nil
		},
	)

	// Mirrors go.temporal.io/sdk/internal's own Test_WorkflowCancellation
	// fixture: the workflow is blocked in the (heartbeating) activity, so the
	// test suite's clock advances at wall-clock pace and this delayed callback
	// fires almost immediately, requesting cancellation mid-provision.
	env.RegisterDelayedCallback(func() {
		env.CancelWorkflow()
	}, time.Millisecond)

	env.ExecuteWorkflow(RunRecipeWorkflowName, runRecipeTestInput())

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	err := env.GetWorkflowError()
	if err == nil {
		t.Fatal("workflow returned no error; want a cancellation error (Temporal must record this execution as Canceled, not Completed)")
	}
	if !temporal.IsCanceledError(err) {
		t.Fatalf("workflow error = %v, want a cancellation error", err)
	}

	if !teardownCalled {
		t.Fatal("teardown did not run after cancellation during provision")
	}

	value, err := env.QueryWorkflow(workflowpb.GetRunStateQueryName)
	if err != nil {
		t.Fatalf("query run state: %v", err)
	}
	var state workflowpb.RunState
	if err := value.Get(&state); err != nil {
		t.Fatalf("decode run state: %v", err)
	}
	if got, want := state.GetStatus(), common.Status_STATUS_CANCELLED; got != want { //nolint:misspell // generated proto enum identifier.
		t.Fatalf("run state status = %s, want %s", got, want)
	}
	if got := state.GetStages()[runRecipeStageCompileIndex].GetStatus(); got != common.Status_STATUS_COMPLETED {
		t.Fatalf("compile stage status = %s, want COMPLETED", got)
	}
	if got := state.GetStages()[runRecipeStageInfraIndex].GetStatus(); got != common.Status_STATUS_CANCELLED { //nolint:misspell // generated proto enum identifier.
		t.Fatalf("infra stage status = %s, want CANCELED", got)
	}
	if got := state.GetStages()[runRecipeStageExecuteIndex].GetStatus(); got != common.Status_STATUS_CANCELLED { //nolint:misspell // generated proto enum identifier.
		t.Fatalf("execute stage status = %s, want CANCELED (never started, but marked canceled)", got)
	}
	if got := state.GetStages()[runRecipeStageTeardownIndex].GetStatus(); got != common.Status_STATUS_COMPLETED {
		t.Fatalf("teardown stage status = %s, want COMPLETED", got)
	}
}
