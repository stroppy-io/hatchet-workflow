package workflows

import (
	"context"
	"fmt"
	"sync"
	"testing"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	infrastructurebuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/infrastructure"
	runbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domainpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	temporalworkflow "go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/proto"
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
		Workload:            cfg.GetWorkload(),
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

func TestTestWorkflowOrchestratesDeploymentStages(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env, DefaultOptions())
	registerFakeDeploymentActivities(env)
	registerFakeAgentActivities(env)
	runtime := &fakeRuntimeActivities{}
	registerFakeRuntimeActivities(env, runtime)

	var childStarts []string
	env.SetOnChildWorkflowStartedListener(func(info *temporalworkflow.Info, _ temporalworkflow.Context, _ converter.EncodedValues) {
		childStarts = append(childStarts, info.WorkflowType.Name)
	})

	testRun := domainTestRun(t, renderOverrides())
	env.ExecuteWorkflow(workflowpb.TestWorkflowWorkflowName, &workflowpb.TestWorkflowRequest{
		TenantId:       "tenant-1",
		TestRun:        testRun,
		AgentBootstrap: testAgentBootstrapForPlan(testRun.GetInfrastructurePlan()),
	})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	want := []string{
		workflowpb.CalculateQuotasWorkflowWorkflowName,
		workflowpb.ProcessInfrastructureWorkflowWorkflowName,
		workflowpb.RenderDockerInputWorkflowWorkflowName,
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
	registerFakeDeploymentActivities(env)
	registerFakeAgentActivities(env)
	runtime := &fakeRuntimeActivities{}
	registerFakeRuntimeActivities(env, runtime)

	testRun := domainTestRun(t, nil)
	env.ExecuteWorkflow(workflowpb.TestWorkflowWorkflowName, &workflowpb.TestWorkflowRequest{
		TenantId:       "tenant-1",
		TestRun:        testRun,
		AgentBootstrap: testAgentBootstrapForPlan(testRun.GetInfrastructurePlan()),
	})

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

	if got := runtime.lastRunStatus(); got != common.Status_STATUS_COMPLETED {
		t.Fatalf("persisted run state status = %s, want %s", got, common.Status_STATUS_COMPLETED)
	}
}

func TestExecuteDeploymentPlanWorkflowPersistsActionStatusAndExecutionContext(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env, DefaultOptions())
	registerFakeAgentActivities(env)
	runtime := &fakeRuntimeActivities{}
	registerFakeRuntimeActivities(env, runtime)

	plan := &deploymentpb.DeploymentPlan{
		Components: []*deploymentpb.ComponentDeployment{
			{
				ComponentId:    "postgres-master",
				NodeId:         "node-1",
				GlobalPriority: 1,
				Steps: []*deploymentpb.AgentStep{
					{
						Id:    "010_create_data_dir",
						Order: 10,
						Action: &deploymentpb.AgentStep_CreateDir{CreateDir: &common.Dir{
							Info:          &common.Dir_Info{Path: "/var/lib/postgresql/data", Mode: 0755},
							CreateParents: true,
						}},
					},
					{
						Id:    "020_start",
						Order: 20,
						Action: &deploymentpb.AgentStep_CallCmd{CallCmd: &common.Cmd{
							Spec: &common.Cmd_Spec{
								Command: &common.Cmd_Spec_Argv{Argv: &common.Cmd_Argv{Args: []string{"systemctl", "start", "postgresql"}}},
							},
						}},
					},
				},
			},
		},
	}

	env.ExecuteWorkflow(workflowpb.ExecuteDeploymentPlanWorkflowWorkflowName, &workflowpb.ExecuteDeploymentPlanWorkflowRequest{
		RunId:          "run-1",
		DeploymentPlan: plan,
		AgentBootstrap: &workflowpb.AgentBootstrap{
			AgentTaskQueues: map[string]string{"node-1": "secret-queue-node-1"},
		},
		InfrastructureState: &deploymentpb.InfrastructureState{
			Provider: deploymentpb.Provider_PROVIDER_DOCKER,
			Machines: []*deploymentpb.MachineState{
				{NodeId: "node-1", Status: common.Status_STATUS_DEPLOYED},
			},
		},
	})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	var resp workflowpb.ExecuteDeploymentPlanWorkflowResponse
	if err := env.GetWorkflowResult(&resp); err != nil {
		t.Fatalf("get workflow result: %v", err)
	}
	component := resp.GetDeploymentPlan().GetComponents()[0]
	if got, want := component.GetStatus(), common.Status_STATUS_DEPLOYED; got != want {
		t.Fatalf("component status = %s, want %s", got, want)
	}
	if got, want := component.GetLabels()[deploymentbuilder.LabelNodeExecutionID], deploymentbuilder.ComponentExecutionID("postgres-master"); got != want {
		t.Fatalf("component node_execution_id = %q, want %q", got, want)
	}
	for _, step := range component.GetSteps() {
		if got, want := step.GetStatus(), common.Status_STATUS_DEPLOYED; got != want {
			t.Fatalf("step %s status = %s, want %s", step.GetId(), got, want)
		}
		if got, want := step.GetLabels()[deploymentbuilder.LabelNodeExecutionID], deploymentbuilder.StepExecutionID("postgres-master", step.GetId()); got != want {
			t.Fatalf("step %s node_execution_id = %q, want %q", step.GetId(), got, want)
		}
	}
	envVars := component.GetSteps()[1].GetCallCmd().GetSpec().GetEnv()
	if got, want := envVars[deploymentbuilder.EnvNodeExecutionID], deploymentbuilder.StepExecutionID("postgres-master", "020_start"); got != want {
		t.Fatalf("command env node_execution_id = %q, want %q", got, want)
	}
	if got := len(runtime.deploymentPlans); got < 4 {
		t.Fatalf("deployment plan persist calls = %d, want live status updates", got)
	}
	if got := len(runtime.logLines); got < 4 {
		t.Fatalf("synthetic action log lines = %d, want start/completed per step", got)
	}
	if got, want := runtime.logLines[0].GetNodeExecutionId(), deploymentbuilder.StepExecutionID("postgres-master", "010_create_data_dir"); got != want {
		t.Fatalf("first log node_execution_id = %q, want %q", got, want)
	}
}

func TestSuiteWorkflowFansOutRunConfigs(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	RegisterWorkflows(env, DefaultOptions())
	registerFakeDeploymentActivities(env)
	registerFakeAgentActivities(env)
	runtime := &fakeRuntimeActivities{}
	registerFakeRuntimeActivities(env, runtime)

	cfg1 := workflowRunConfig(t, nil)
	cfg2 := workflowRunConfig(t, nil)
	cfg2.Id = "run-2"

	var runChildren int
	env.SetOnChildWorkflowStartedListener(func(info *temporalworkflow.Info, _ temporalworkflow.Context, _ converter.EncodedValues) {
		if info.WorkflowType.Name == workflowpb.TestWorkflowWorkflowName {
			runChildren++
		}
	})

	env.ExecuteWorkflow(workflowpb.SuiteWorkflowWorkflowName, &workflowpb.SuiteWorkflowRequest{
		SuiteRunId:  "suite-run-1",
		Runs:        []*workflowpb.RunConfig{cfg1, cfg2},
		MaxParallel: 1,
	})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}
	if got, want := runChildren, 2; got != want {
		t.Fatalf("child run workflows = %d, want %d", got, want)
	}
	if !runtime.hasSuiteStatus(common.Status_STATUS_RUNNING) {
		t.Fatal("suite runtime persistence did not record RUNNING")
	}
	if got := runtime.lastSuiteStatus(); got != common.Status_STATUS_COMPLETED {
		t.Fatalf("last persisted suite status = %s, want %s", got, common.Status_STATUS_COMPLETED)
	}
}

func workflowRunConfig(t *testing.T, overrides *deploymentpb.RenderOverrideSet) *workflowpb.RunConfig {
	t.Helper()

	testRun := domainTestRun(t, overrides)
	return &workflowpb.RunConfig{
		TenantId:           "tenant-1",
		Id:                 testRun.GetId(),
		Database:           testRun.GetDatabase(),
		Workload:           testRun.GetWorkload(),
		TopologySpec:       testRun.GetTopologySpec(),
		InfrastructurePlan: testRun.GetInfrastructurePlan(),
		RenderOverrides:    testRun.GetRenderOverrides(),
		AgentBootstrap:     testAgentBootstrapForPlan(testRun.GetInfrastructurePlan()),
	}
}

func testAgentBootstrap() *workflowpb.AgentBootstrap {
	return &workflowpb.AgentBootstrap{
		ServerAddr:        "http://127.0.0.1:8080",
		TemporalNamespace: "default",
		ExtraEnv: map[string]string{
			"STROPPY_TEST_ENV": "test",
		},
	}
}

func testAgentBootstrapForPlan(plan *deploymentpb.InfrastructurePlan) *workflowpb.AgentBootstrap {
	bootstrap := testAgentBootstrap()
	bootstrap.AgentTokens = make(map[string]string, len(plan.GetMachines()))
	bootstrap.AgentTaskQueues = make(map[string]string, len(plan.GetMachines()))
	for _, machine := range plan.GetMachines() {
		bootstrap.AgentTokens[machine.GetNodeId()] = "agent-token-" + machine.GetNodeId()
		bootstrap.AgentTaskQueues[machine.GetNodeId()] = "secret-queue-" + machine.GetNodeId()
	}
	return bootstrap
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
		address := fmt.Sprintf("10.0.0.%d", i+1)
		sshPort := uint32(22)
		machineState := &deploymentpb.MachineState{
			NodeId:             machine.GetNodeId(),
			ProviderResourceId: "resource-" + machine.GetNodeId(),
			Status:             common.Status_STATUS_DEPLOYED,
			Endpoints: []*deploymentpb.Endpoint{
				{Name: "private", Address: address, Port: &sshPort},
			},
			Labels: machine.GetLabels(),
			Tags:   machine.GetTags(),
		}
		if plan.GetProvider() == deploymentpb.Provider_PROVIDER_DOCKER {
			machineState.ProviderOutput = &deploymentpb.MachineState_Docker{
				Docker: &deploymentpb.Docker_ContainerOutput{
					Id:         machineState.GetProviderResourceId(),
					Name:       machine.GetNodeId(),
					InternalIp: address,
					Status:     "running",
				},
			}
		}
		state.Machines = append(state.Machines, machineState)
	}
	return state
}

func registerFakeDeploymentActivities(env *testsuite.TestWorkflowEnvironment) {
	workflowpb.RegisterDeploymentServiceActivities(env, fakeDeploymentActivities{})
}

type fakeDeploymentActivities struct{}

func (fakeDeploymentActivities) AcquireNetworkActivity(context.Context, *workflowpb.AcquireNetworkActivityRequest) (*workflowpb.AcquireNetworkActivityResponse, error) {
	return &workflowpb.AcquireNetworkActivityResponse{}, nil
}

func (fakeDeploymentActivities) CommitNetworkActivity(context.Context, *workflowpb.CommitNetworkActivityRequest) (*workflowpb.CommitNetworkActivityResponse, error) {
	return &workflowpb.CommitNetworkActivityResponse{}, nil
}

func (fakeDeploymentActivities) ReleaseNetworkActivity(context.Context, *workflowpb.ReleaseNetworkActivityRequest) (*workflowpb.ReleaseNetworkActivityResponse, error) {
	return &workflowpb.ReleaseNetworkActivityResponse{}, nil
}

func (fakeDeploymentActivities) AcquireQuotasActivity(_ context.Context, req *workflowpb.AcquireQuotasActivityRequest) (*workflowpb.AcquireQuotasActivityResponse, error) {
	return &workflowpb.AcquireQuotasActivityResponse{QuotaAllocations: echoQuotaAllocations(req.GetQuotaRequests())}, nil
}

func (fakeDeploymentActivities) CommitQuotasActivity(context.Context, *workflowpb.CommitQuotasActivityRequest) (*workflowpb.CommitQuotasActivityResponse, error) {
	return &workflowpb.CommitQuotasActivityResponse{}, nil
}

func (fakeDeploymentActivities) ReleaseQuotasActivity(context.Context, *workflowpb.ReleaseQuotasActivityRequest) (*workflowpb.ReleaseQuotasActivityResponse, error) {
	return &workflowpb.ReleaseQuotasActivityResponse{}, nil
}

func (fakeDeploymentActivities) DockerPullActivity(context.Context, *deploymentpb.Docker_Input) (*deploymentpb.Docker_Output, error) {
	return &deploymentpb.Docker_Output{}, nil
}

func (fakeDeploymentActivities) DockerUpActivity(_ context.Context, input *deploymentpb.Docker_Input) (*deploymentpb.Docker_Output, error) {
	output := &deploymentpb.Docker_Output{
		Containers: make(map[string]*deploymentpb.Docker_ContainerOutput, len(input.GetContainers())),
		NetworkId:  "network-run-1",
	}
	i := 1
	for name := range input.GetContainers() {
		output.Containers[name] = &deploymentpb.Docker_ContainerOutput{
			Id:         "container-" + name,
			Name:       name,
			InternalIp: fmt.Sprintf("10.0.0.%d", i),
			Status:     "running",
		}
		i++
	}
	return output, nil
}

func (fakeDeploymentActivities) DockerDownActivity(context.Context, *deploymentpb.Docker_Input) (*deploymentpb.Docker_Output, error) {
	return &deploymentpb.Docker_Output{}, nil
}

func (fakeDeploymentActivities) TerraformPlanActivity(context.Context, *deploymentpb.Terraform_Input) (*deploymentpb.Terraform_Output, error) {
	return &deploymentpb.Terraform_Output{}, nil
}

func (fakeDeploymentActivities) TerraformApplyActivity(context.Context, *deploymentpb.Terraform_Input) (*deploymentpb.Terraform_Output, error) {
	return &deploymentpb.Terraform_Output{}, nil
}

func (fakeDeploymentActivities) TerraformDestroyActivity(context.Context, *deploymentpb.Terraform_Input) (*deploymentpb.Terraform_Output, error) {
	return &deploymentpb.Terraform_Output{}, nil
}

func registerFakeAgentActivities(env *testsuite.TestWorkflowEnvironment) {
	workflowpb.RegisterEnsureAgentOnlineActivityActivity(env, func(context.Context) error { return nil })
	workflowpb.RegisterCreateDirActivityActivity(env, func(context.Context, *common.Dir) error { return nil })
	workflowpb.RegisterWriteFileActivityActivity(env, func(context.Context, *common.File) error { return nil })
	workflowpb.RegisterFetchFileActivityActivity(env, func(context.Context, *common.File) error { return nil })
	workflowpb.RegisterCallCmdActivityActivity(env, func(_ context.Context, _ *common.Cmd) (*common.Cmd_Result, error) {
		return &common.Cmd_Result{ExitCode: 0}, nil
	})
}

func registerFakeRuntimeActivities(env *testsuite.TestWorkflowEnvironment, fake *fakeRuntimeActivities) {
	env.RegisterActivityWithOptions(fake.PersistRunState, activity.RegisterOptions{Name: PersistRunStateActivityName})
	env.RegisterActivityWithOptions(fake.PersistDeploymentPlan, activity.RegisterOptions{Name: PersistDeploymentPlanActivityName})
	env.RegisterActivityWithOptions(fake.AppendRunLogs, activity.RegisterOptions{Name: AppendRunLogsActivityName})
	env.RegisterActivityWithOptions(fake.PersistSuiteRun, activity.RegisterOptions{Name: PersistSuiteRunActivityName})
}

type fakeRuntimeActivities struct {
	mu              sync.Mutex
	runStates       []*workflowpb.RunState
	deploymentPlans []*deploymentpb.DeploymentPlan
	logLines        []*monitor.LogLine
	suiteStatuses   []common.Status
}

func (f *fakeRuntimeActivities) PersistRunState(
	_ context.Context,
	_ string,
	state *workflowpb.RunState,
	_ *deploymentpb.InfrastructureState,
	_ *deploymentpb.DeploymentPlan,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if state == nil {
		f.runStates = append(f.runStates, nil)
		return nil
	}
	f.runStates = append(f.runStates, proto.Clone(state).(*workflowpb.RunState))
	return nil
}

func (f *fakeRuntimeActivities) PersistDeploymentPlan(
	_ context.Context,
	_ string,
	plan *deploymentpb.DeploymentPlan,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if plan == nil {
		f.deploymentPlans = append(f.deploymentPlans, nil)
		return nil
	}
	f.deploymentPlans = append(f.deploymentPlans, proto.Clone(plan).(*deploymentpb.DeploymentPlan))
	return nil
}

func (f *fakeRuntimeActivities) AppendRunLogs(_ context.Context, lines []*monitor.LogLine) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, line := range lines {
		if line == nil {
			f.logLines = append(f.logLines, nil)
			continue
		}
		f.logLines = append(f.logLines, proto.Clone(line).(*monitor.LogLine))
	}
	return nil
}

func (f *fakeRuntimeActivities) PersistSuiteRun(_ context.Context, _ string, status common.Status) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.suiteStatuses = append(f.suiteStatuses, status)
	return nil
}

func (f *fakeRuntimeActivities) lastRunStatus() common.Status {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.runStates) == 0 {
		return common.Status_STATUS_UNSPECIFIED
	}
	return f.runStates[len(f.runStates)-1].GetStatus()
}

func (f *fakeRuntimeActivities) hasSuiteStatus(status common.Status) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, got := range f.suiteStatuses {
		if got == status {
			return true
		}
	}
	return false
}

func (f *fakeRuntimeActivities) lastSuiteStatus() common.Status {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.suiteStatuses) == 0 {
		return common.Status_STATUS_UNSPECIFIED
	}
	return f.suiteStatuses[len(f.suiteStatuses)-1]
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
