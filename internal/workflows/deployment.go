package workflows

import (
	"errors"
	"fmt"
	"sort"
	"time"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/proto"
)

type deploymentWorkflows struct {
	options Options
}

func (w *deploymentWorkflows) CalculateQuotasWorkflow(_ workflow.Context, input *workflowpb.CalculateQuotasWorkflowWorkflowInput) (workflowpb.CalculateQuotasWorkflowWorkflow, error) {
	return &calculateQuotasWorkflow{req: input.Req}, nil
}

func (w *deploymentWorkflows) ExecuteDeploymentPlanWorkflow(_ workflow.Context, input *workflowpb.ExecuteDeploymentPlanWorkflowWorkflowInput) (workflowpb.ExecuteDeploymentPlanWorkflowWorkflow, error) {
	return &executeDeploymentPlanWorkflow{req: input.Req}, nil
}

func (w *deploymentWorkflows) ProcessInfrastructureWorkflow(_ workflow.Context, input *workflowpb.ProcessInfrastructureWorkflowWorkflowInput) (workflowpb.ProcessInfrastructureWorkflowWorkflow, error) {
	return &processInfrastructureWorkflow{req: input.Req}, nil
}

func (w *deploymentWorkflows) RenderDeploymentPlanWorkflow(_ workflow.Context, input *workflowpb.RenderDeploymentPlanWorkflowWorkflowInput) (workflowpb.RenderDeploymentPlanWorkflowWorkflow, error) {
	return &renderDeploymentPlanWorkflow{req: input.Req, options: w.options}, nil
}

func (w *deploymentWorkflows) RenderDockerInputWorkflow(_ workflow.Context, input *workflowpb.RenderDockerInputWorkflowWorkflowInput) (workflowpb.RenderDockerInputWorkflowWorkflow, error) {
	return &renderDockerInputWorkflow{req: input.Req}, nil
}

func (w *deploymentWorkflows) RenderTerraformVariablesWorkflow(_ workflow.Context, input *workflowpb.RenderTerraformVariablesWorkflowWorkflowInput) (workflowpb.RenderTerraformVariablesWorkflowWorkflow, error) {
	return &renderTerraformVariablesWorkflow{req: input.Req}, nil
}

type calculateQuotasWorkflow struct {
	req *workflowpb.CalculateQuotasWorkflowRequest
}

func (w *calculateQuotasWorkflow) Execute(workflow.Context) (*workflowpb.CalculateQuotasWorkflowResponse, error) {
	plan := w.req.GetPlan()
	if plan == nil {
		return nil, errors.New("infrastructure plan is required")
	}
	if err := plan.Validate(); err != nil {
		return nil, err
	}

	quotas := make(map[string]*deploymentpb.Quota_Request, len(plan.GetMachines()))
	for _, machine := range plan.GetMachines() {
		if len(machine.GetQuotaRequests()) == 0 {
			continue
		}
		quotas[machine.GetNodeId()] = machine.GetQuotaRequests()[0]
	}

	return &workflowpb.CalculateQuotasWorkflowResponse{
		Plan:          plan,
		QuotaRequests: quotas,
	}, nil
}

type processInfrastructureWorkflow struct {
	req *workflowpb.ProcessInfrastructureWorkflowRequest
}

func (w *processInfrastructureWorkflow) Execute(ctx workflow.Context) (*workflowpb.ProcessInfrastructureWorkflowResponse, error) {
	if err := w.req.Validate(); err != nil {
		return nil, err
	}
	plan := w.req.GetPlan()
	switch plan.GetProvider() {
	case deploymentpb.Provider_PROVIDER_DOCKER:
		dockerInput, err := workflowpb.RenderDockerInputWorkflowChild(ctx, &workflowpb.RenderDockerInputWorkflowRequest{
			RunId:          w.req.GetRunId(),
			Plan:           plan,
			AgentBootstrap: w.req.GetAgentBootstrap(),
		})
		if err != nil {
			return nil, err
		}
		if _, err := workflowpb.DockerPullActivity(ctx, dockerInput); err != nil {
			return nil, err
		}
		dockerOutput, err := workflowpb.DockerUpActivity(ctx, dockerInput)
		if err != nil {
			return nil, err
		}
		state, err := dockerInfrastructureState(w.req.GetRunId(), plan, dockerOutput)
		if err != nil {
			return nil, err
		}
		return &workflowpb.ProcessInfrastructureWorkflowResponse{State: state}, nil
	case deploymentpb.Provider_PROVIDER_YANDEX:
		terraformInput, err := workflowpb.RenderTerraformVariablesWorkflowChild(ctx, &workflowpb.RenderTerraformVariablesWorkflowRequest{
			RunId:          w.req.GetRunId(),
			Plan:           plan,
			Action:         deploymentpb.Terraform_ACTION_APPLY,
			AgentBootstrap: w.req.GetAgentBootstrap(),
		})
		if err != nil {
			return nil, err
		}
		terraformOutput, err := workflowpb.TerraformApplyActivity(ctx, terraformInput)
		if err != nil {
			return nil, err
		}
		yandexOutput, err := terraformYandexOutput(terraformOutput)
		if err != nil {
			return nil, err
		}
		state, err := yandexInfrastructureState(w.req.GetRunId(), plan, yandexOutput)
		if err != nil {
			return nil, err
		}
		return &workflowpb.ProcessInfrastructureWorkflowResponse{State: state}, nil
	default:
		return nil, fmt.Errorf("provider %s is not supported", plan.GetProvider())
	}
}

type renderDeploymentPlanWorkflow struct {
	req     *workflowpb.RenderDeploymentPlanWorkflowRequest
	options Options
}

func (w *renderDeploymentPlanWorkflow) Execute(workflow.Context) (*workflowpb.RenderDeploymentPlanWorkflowResponse, error) {
	if err := w.req.Validate(); err != nil {
		return nil, err
	}

	plan, err := deploymentbuilder.BuildPlan(w.req.GetTopologySpec(), w.req.GetInfrastructureState(), deploymentbuilder.BuildOptions{
		Database:        w.req.GetDatabase(),
		Workload:        w.req.GetWorkload(),
		PackageResolver: w.options.PackageResolver,
		Renderers:       w.options.DeploymentRenderers,
		RenderOverrides: w.req.GetRenderOverrides(),
	})
	if err != nil {
		return nil, err
	}

	return &workflowpb.RenderDeploymentPlanWorkflowResponse{DeploymentPlan: plan}, nil
}

type executeDeploymentPlanWorkflow struct {
	req *workflowpb.ExecuteDeploymentPlanWorkflowRequest
}

func (w *executeDeploymentPlanWorkflow) Execute(ctx workflow.Context) (*workflowpb.ExecuteDeploymentPlanWorkflowResponse, error) {
	if err := w.req.Validate(); err != nil {
		return nil, err
	}

	plan := proto.Clone(w.req.GetDeploymentPlan()).(*deploymentpb.DeploymentPlan)
	sortComponentExecution(plan.GetComponents())
	for _, component := range plan.GetComponents() {
		component.Status = common.Status_STATUS_DEPLOYMENT
		if err := executeComponentDeployment(ctx, component); err != nil {
			component.Status = common.Status_STATUS_FAILED
			return nil, err
		}
		component.Status = common.Status_STATUS_DEPLOYED
	}

	if err := plan.Validate(); err != nil {
		return nil, err
	}
	return &workflowpb.ExecuteDeploymentPlanWorkflowResponse{DeploymentPlan: plan}, nil
}

type renderDockerInputWorkflow struct {
	req *workflowpb.RenderDockerInputWorkflowRequest
}

func (w *renderDockerInputWorkflow) Execute(workflow.Context) (*deploymentpb.Docker_Input, error) {
	return renderDockerInput(w.req)
}

type renderTerraformVariablesWorkflow struct {
	req *workflowpb.RenderTerraformVariablesWorkflowRequest
}

func (w *renderTerraformVariablesWorkflow) Execute(workflow.Context) (*deploymentpb.Terraform_Input, error) {
	return renderTerraformInput(w.req)
}

func dockerInfrastructureState(runID string, plan *deploymentpb.InfrastructurePlan, output *deploymentpb.Docker_Output) (*deploymentpb.InfrastructureState, error) {
	if output == nil {
		return nil, fmt.Errorf("docker output is required")
	}
	state := baseInfrastructureState(plan, "docker")
	for _, machine := range plan.GetMachines() {
		name := dockerResourceName(runID, machine.GetNodeId())
		container, ok := output.GetContainers()[name]
		if !ok {
			return nil, fmt.Errorf("docker output is missing container %q for node %q", name, machine.GetNodeId())
		}
		private := container.GetInternalIp()
		if private == "" {
			private = container.GetName()
		}
		state.Machines = append(state.Machines, &deploymentpb.MachineState{
			NodeId:             machine.GetNodeId(),
			ProviderResourceId: container.GetId(),
			Status:             common.Status_STATUS_DEPLOYED,
			Endpoints:          privateEndpoints(private, ""),
			ProviderOutput: &deploymentpb.MachineState_Docker{
				Docker: container,
			},
			AllocatedQuotas: nil,
			Labels: deploymentbuilder.MergeLabels(machine.GetLabels(), map[string]string{
				"node_id":       machine.GetNodeId(),
				"resource_name": name,
			}),
			Tags: machine.GetTags(),
		})
	}
	if err := state.Validate(); err != nil {
		return nil, err
	}
	return state, nil
}

func yandexInfrastructureState(runID string, plan *deploymentpb.InfrastructurePlan, output *deploymentpb.Yandex_Output) (*deploymentpb.InfrastructureState, error) {
	if output == nil {
		return nil, fmt.Errorf("yandex output is required")
	}
	state := baseInfrastructureState(plan, "yandex")
	for _, machine := range plan.GetMachines() {
		name := yandexResourceName(runID, machine.GetNodeId())
		vm, ok := output.GetVms()[name]
		if !ok {
			return nil, fmt.Errorf("yandex output is missing vm %q for node %q", name, machine.GetNodeId())
		}
		state.Machines = append(state.Machines, &deploymentpb.MachineState{
			NodeId:             machine.GetNodeId(),
			ProviderResourceId: vm.GetId(),
			Status:             common.Status_STATUS_DEPLOYED,
			Endpoints:          privateEndpoints(vm.GetInternalIp(), vm.GetPublicIp()),
			ProviderOutput: &deploymentpb.MachineState_Yandex{
				Yandex: vm,
			},
			AllocatedQuotas: nil,
			Labels: deploymentbuilder.MergeLabels(machine.GetLabels(), map[string]string{
				"node_id":       machine.GetNodeId(),
				"resource_name": name,
			}),
			Tags: machine.GetTags(),
		})
	}
	if err := state.Validate(); err != nil {
		return nil, err
	}
	return state, nil
}

func baseInfrastructureState(plan *deploymentpb.InfrastructurePlan, provider string) *deploymentpb.InfrastructureState {
	return &deploymentpb.InfrastructureState{
		Provider: plan.GetProvider(),
		Machines: make([]*deploymentpb.MachineState, 0, len(plan.GetMachines())),
		Labels: deploymentbuilder.MergeLabels(plan.GetLabels(), map[string]string{
			"stage":    "infrastructure_state",
			"provider": provider,
		}),
		Tags: plan.GetTags(),
	}
}

func privateEndpoints(privateAddress, publicAddress string) []*deploymentpb.Endpoint {
	sshPort := uint32(22)
	endpoints := []*deploymentpb.Endpoint{
		{
			Name:    "private",
			Address: privateAddress,
			Port:    &sshPort,
			Labels:  map[string]string{"scope": "private"},
		},
	}
	if publicAddress != "" {
		endpoints = append(endpoints, &deploymentpb.Endpoint{
			Name:    "public",
			Address: publicAddress,
			Port:    &sshPort,
			Labels:  map[string]string{"scope": "public"},
		})
	}
	return endpoints
}

func AgentQueue(nodeID string) string {
	return "stroppy-agent-" + nodeID
}

func executeComponentDeployment(ctx workflow.Context, component *deploymentpb.ComponentDeployment) error {
	sort.SliceStable(component.Steps, func(i, j int) bool {
		if component.Steps[i].GetOrder() != component.Steps[j].GetOrder() {
			return component.Steps[i].GetOrder() < component.Steps[j].GetOrder()
		}
		return component.Steps[i].GetId() < component.Steps[j].GetId()
	})
	activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           AgentQueue(component.GetNodeId()),
		StartToCloseTimeout: 60 * time.Minute,
		HeartbeatTimeout:    time.Minute,
	})
	if err := executeActivityNoResult(activityCtx, workflowpb.EnsureAgentOnlineActivityActivityName); err != nil {
		return fmt.Errorf("agent %s is not online: %w", component.GetNodeId(), err)
	}
	for _, step := range component.GetSteps() {
		step.Status = common.Status_STATUS_DEPLOYMENT
		if err := executeAgentStep(activityCtx, step); err != nil {
			step.Status = common.Status_STATUS_FAILED
			return fmt.Errorf("component %s step %s: %w", component.GetComponentId(), step.GetId(), err)
		}
		step.Status = common.Status_STATUS_DEPLOYED
	}
	return nil
}

func executeAgentStep(ctx workflow.Context, step *deploymentpb.AgentStep) error {
	switch action := step.GetAction().(type) {
	case *deploymentpb.AgentStep_CreateDir:
		return executeActivityNoResult(ctx, workflowpb.CreateDirActivityActivityName, action.CreateDir)
	case *deploymentpb.AgentStep_WriteFile:
		return executeActivityNoResult(ctx, workflowpb.WriteFileActivityActivityName, action.WriteFile)
	case *deploymentpb.AgentStep_FetchFile:
		return executeActivityNoResult(ctx, workflowpb.FetchFileActivityActivityName, action.FetchFile)
	case *deploymentpb.AgentStep_CallCmd:
		var result common.Cmd_Result
		if err := workflow.ExecuteActivity(ctx, workflowpb.CallCmdActivityActivityName, action.CallCmd).Get(ctx, &result); err != nil {
			return err
		}
		if !expectedExitCode(action.CallCmd, result.GetExitCode()) {
			return fmt.Errorf("command exited %d: %s", result.GetExitCode(), string(result.GetStderr()))
		}
		action.CallCmd.Result = &result
		return nil
	default:
		return fmt.Errorf("unsupported agent step action %T", action)
	}
}

func executeActivityNoResult(ctx workflow.Context, name string, args ...any) error {
	return workflow.ExecuteActivity(ctx, name, args...).Get(ctx, nil)
}

func expectedExitCode(cmd *common.Cmd, exitCode int32) bool {
	expected := cmd.GetSpec().GetExpectedExitCodes()
	if len(expected) == 0 {
		expected = []int32{0}
	}
	for _, code := range expected {
		if code == exitCode {
			return true
		}
	}
	return false
}

func sortComponentExecution(components []*deploymentpb.ComponentDeployment) {
	sort.SliceStable(components, func(i, j int) bool {
		left := components[i]
		right := components[j]
		if left.GetGlobalPriority() != right.GetGlobalPriority() {
			return left.GetGlobalPriority() < right.GetGlobalPriority()
		}
		if left.GetNodeId() != right.GetNodeId() {
			return left.GetNodeId() < right.GetNodeId()
		}
		if left.GetNodePriority() != right.GetNodePriority() {
			return left.GetNodePriority() < right.GetNodePriority()
		}
		return left.GetComponentId() < right.GetComponentId()
	})
}
