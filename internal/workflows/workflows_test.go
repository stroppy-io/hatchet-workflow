package workflows

import (
	"testing"

	infrastructurebuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/infrastructure"
	runbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domainpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	temporalworkflow "go.temporal.io/sdk/workflow"
)

func TestRenderDeploymentPlanWorkflowAppliesRenderOverrides(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env, DefaultOptions())

	cfg := workflowRunConfig(t, renderOverrides())
	state := infrastructureStateForPlan(cfg.GetInfrastructurePlan())
	env.ExecuteWorkflow(workflowpb.RenderDeploymentPlanWorkflowWorkflowName, &workflowpb.RenderDeploymentPlanWorkflowRequest{
		TopologySpec:        cfg.GetTopologySpec(),
		InfrastructurePlan:  cfg.GetInfrastructurePlan(),
		InfrastructureState: state,
		RenderOverrides:     cfg.GetRenderOverrides(),
		Database:            cfg.GetDatabase(),
	})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	var resp workflowpb.RenderDeploymentPlanWorkflowResponse
	if err := env.GetWorkflowResult(&resp); err != nil {
		t.Fatalf("get workflow result: %v", err)
	}
	master := componentDeploymentByID(resp.GetDeploymentPlan(), "postgres-master")
	if master == nil {
		t.Fatal("postgres-master deployment is missing")
	}
	if got := writeFileText(master, "030_write_config"); got != "shared_buffers = 1GB\n" {
		t.Fatalf("rendered config = %q, want override", got)
	}
}

func TestTestRunWorkflowOrchestratesDeploymentStages(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env, DefaultOptions())

	var childStarts []string
	env.SetOnChildWorkflowStartedListener(func(info *temporalworkflow.Info, _ temporalworkflow.Context, _ converter.EncodedValues) {
		childStarts = append(childStarts, info.WorkflowType.Name)
	})

	env.ExecuteWorkflow(workflowpb.TestRunWorkflowWorkflowName, workflowRunConfig(t, renderOverrides()))

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	want := []string{
		workflowpb.ProcessInfrastructureWorkflowWorkflowName,
		workflowpb.RenderDeploymentPlanWorkflowWorkflowName,
		workflowpb.ExecuteDeploymentPlanWorkflowWorkflowName,
		workflowpb.RunWorkloadWorkflowWorkflowName,
	}
	if !sameStrings(childStarts, want) {
		t.Fatalf("child workflow order = %v, want %v", childStarts, want)
	}
}

func TestTestWorkflowExposesRunStateQuery(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env, DefaultOptions())

	testRun := domainTestRun(t, nil)
	env.ExecuteWorkflow(workflowpb.TestWorkflowWorkflowName, &workflowpb.TestWorkflowRequest{TestRun: testRun})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
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
	for _, stage := range state.GetStages() {
		if got, want := stage.GetStatus(), common.Status_STATUS_COMPLETED; got != want {
			t.Fatalf("stage %q status = %s, want %s", stage.GetName(), got, want)
		}
	}
}

func workflowRunConfig(t *testing.T, overrides *deploymentpb.RenderOverrideSet) *workflowpb.RunConfig {
	t.Helper()

	testRun := domainTestRun(t, overrides)
	return &workflowpb.RunConfig{
		Id:                 testRun.GetId(),
		Database:           testRun.GetDatabase(),
		Workload:           testRun.GetWorkload(),
		TopologySpec:       testRun.GetTopologySpec(),
		InfrastructurePlan: testRun.GetInfrastructurePlan(),
		RenderOverrides:    testRun.GetRenderOverrides(),
	}
}

func domainTestRun(t *testing.T, overrides *deploymentpb.RenderOverrideSet) *domainpb.TestRun {
	t.Helper()

	testRun, err := runbuilder.BuildTestRun(runbuilder.BuildOptions{
		ID:       "run-1",
		Database: postgresDatabase(),
		Workload: workload(),
		Provider: deploymentpb.Provider_PROVIDER_DOCKER,
		Infrastructure: infrastructurebuilder.BuildOptions{
			DefaultSizing: infrastructurebuilder.MachineSizing{CPUCores: 1, MemoryMB: 1024, DiskGB: 20},
		},
		RenderOverrides: overrides,
	})
	if err != nil {
		t.Fatalf("build test run: %v", err)
	}
	return testRun
}

func postgresDatabase() *domainpb.Database {
	return &domainpb.Database{
		Kind: domainpb.Database_KIND_POSTGRES,
		Source: &domainpb.Database_Params{
			Params: &domainpb.DatabaseParams{
				Version: "16",
				Engine: &domainpb.DatabaseParams_Postgres{
					Postgres: &domainpb.PostgresParams{
						Replicas: 1,
						MasterOptions: map[string]string{
							"max_connections": "200",
						},
					},
				},
			},
		},
	}
}

func workload() *domainpb.Workload {
	return &domainpb.Workload{
		Script:   "tpcc/tx",
		Protocol: domainpb.Workload_PROTOCOL_PG,
		Execution: &domainpb.Workload_Execution{
			Vus: 1,
			Limit: &domainpb.Workload_Execution_Duration{
				Duration: "1m",
			},
		},
	}
}

func renderOverrides() *deploymentpb.RenderOverrideSet {
	return &deploymentpb.RenderOverrideSet{
		Files: []*deploymentpb.FileOverride{
			{
				ArtifactId:  "postgres-master/postgresql.conf",
				ComponentId: "postgres-master",
				File: &common.File{
					Info: &common.File_Info{
						Path:          "/etc/stroppy-cloud/postgres-master/postgresql.conf",
						Mode:          0644,
						CreateParents: true,
					},
					Content: &common.File_Text{Text: "shared_buffers = 1GB\n"},
				},
			},
		},
	}
}

func infrastructureStateForPlan(plan *deploymentpb.InfrastructurePlan) *deploymentpb.InfrastructureState {
	state := &deploymentpb.InfrastructureState{
		Provider: plan.GetProvider(),
		Machines: make([]*deploymentpb.MachineState, 0, len(plan.GetMachines())),
		Tags:     plan.GetTags(),
	}
	for i, machine := range plan.GetMachines() {
		state.Machines = append(state.Machines, materializeMachineState(plan.GetProvider(), machine, i))
	}
	return state
}

func componentDeploymentByID(plan *deploymentpb.DeploymentPlan, componentID string) *deploymentpb.ComponentDeployment {
	for _, component := range plan.GetComponents() {
		if component.GetComponentId() == componentID {
			return component
		}
	}
	return nil
}

func writeFileText(component *deploymentpb.ComponentDeployment, stepID string) string {
	for _, step := range component.GetSteps() {
		if step.GetId() == stepID {
			return step.GetWriteFile().GetText()
		}
	}
	return ""
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
