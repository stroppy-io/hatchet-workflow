package workflows

import (
	"errors"
	"fmt"

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
	return &renderDockerInputWorkflow{plan: input.Req}, nil
}

func (w *deploymentWorkflows) RenderTerraformVariablesWorkflow(_ workflow.Context, input *workflowpb.RenderTerraformVariablesWorkflowWorkflowInput) (workflowpb.RenderTerraformVariablesWorkflowWorkflow, error) {
	return &renderTerraformVariablesWorkflow{plan: input.Req}, nil
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

func (w *processInfrastructureWorkflow) Execute(workflow.Context) (*workflowpb.ProcessInfrastructureWorkflowResponse, error) {
	plan := w.req.GetPlan()
	if plan == nil {
		return nil, errors.New("infrastructure plan is required")
	}
	if err := plan.Validate(); err != nil {
		return nil, err
	}

	state := &deploymentpb.InfrastructureState{
		Provider: plan.GetProvider(),
		Machines: make([]*deploymentpb.MachineState, 0, len(plan.GetMachines())),
		Labels: deploymentbuilder.MergeLabels(plan.GetLabels(), map[string]string{
			"stage": "infrastructure_state",
			"mode":  "local_materialized",
		}),
		Tags: plan.GetTags(),
	}

	for i, machine := range plan.GetMachines() {
		state.Machines = append(state.Machines, materializeMachineState(plan.GetProvider(), machine, i))
	}

	if err := state.Validate(); err != nil {
		return nil, err
	}
	return &workflowpb.ProcessInfrastructureWorkflowResponse{State: state}, nil
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

func (w *executeDeploymentPlanWorkflow) Execute(workflow.Context) (*workflowpb.ExecuteDeploymentPlanWorkflowResponse, error) {
	if err := w.req.Validate(); err != nil {
		return nil, err
	}

	plan := proto.Clone(w.req.GetDeploymentPlan()).(*deploymentpb.DeploymentPlan)
	for _, component := range plan.GetComponents() {
		component.Status = common.Status_STATUS_DEPLOYED
		for _, step := range component.GetSteps() {
			step.Status = common.Status_STATUS_DEPLOYED
		}
	}

	if err := plan.Validate(); err != nil {
		return nil, err
	}
	return &workflowpb.ExecuteDeploymentPlanWorkflowResponse{DeploymentPlan: plan}, nil
}

type renderDockerInputWorkflow struct {
	plan *deploymentpb.InfrastructurePlan
}

func (w *renderDockerInputWorkflow) Execute(workflow.Context) (*deploymentpb.Docker_Input, error) {
	if w.plan == nil {
		return nil, errors.New("infrastructure plan is required")
	}
	if err := w.plan.Validate(); err != nil {
		return nil, err
	}
	if w.plan.GetProvider() != deploymentpb.Provider_PROVIDER_DOCKER {
		return nil, fmt.Errorf("docker input requires docker provider, got %s", w.plan.GetProvider())
	}

	input := &deploymentpb.Docker_Input{
		Network:    &deploymentpb.Docker_Network{},
		Containers: make(map[string]*deploymentpb.Docker_Container, len(w.plan.GetMachines())),
	}
	for _, machine := range w.plan.GetMachines() {
		container := machine.GetDocker()
		if container == nil {
			return nil, fmt.Errorf("machine %q has no docker params", machine.GetNodeId())
		}
		input.Containers[machine.GetNodeId()] = container
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}
	return input, nil
}

type renderTerraformVariablesWorkflow struct {
	plan *deploymentpb.InfrastructurePlan
}

func (w *renderTerraformVariablesWorkflow) Execute(workflow.Context) (*deploymentpb.Terraform_Input, error) {
	if w.plan == nil {
		return nil, errors.New("infrastructure plan is required")
	}
	return nil, errors.New("terraform variable rendering is not implemented yet")
}

func materializeMachineState(provider deploymentpb.Provider, machine *deploymentpb.MachinePlan, index int) *deploymentpb.MachineState {
	privatePort := uint32(22)
	address := fmt.Sprintf("10.0.0.%d", index+1)
	state := &deploymentpb.MachineState{
		NodeId:             machine.GetNodeId(),
		ProviderResourceId: providerResourceID(provider, machine.GetNodeId()),
		Status:             common.Status_STATUS_DEPLOYED,
		Endpoints: []*deploymentpb.Endpoint{
			{
				Name:    "private",
				Address: address,
				Port:    &privatePort,
				Labels:  map[string]string{"source": "local_materialized"},
			},
		},
		AllocatedQuotas: nil,
		Labels: deploymentbuilder.MergeLabels(machine.GetLabels(), map[string]string{
			"node_id": machine.GetNodeId(),
		}),
		Tags: machine.GetTags(),
	}

	switch provider {
	case deploymentpb.Provider_PROVIDER_DOCKER:
		state.ProviderOutput = &deploymentpb.MachineState_Docker{
			Docker: &deploymentpb.Docker_ContainerOutput{
				Id:         state.GetProviderResourceId(),
				Name:       machine.GetNodeId(),
				InternalIp: address,
				Status:     "running",
			},
		}
	case deploymentpb.Provider_PROVIDER_YANDEX:
		state.ProviderOutput = &deploymentpb.MachineState_Yandex{
			Yandex: &deploymentpb.Yandex_VmOutput{
				Id:         state.GetProviderResourceId(),
				InternalIp: address,
			},
		}
	}

	return state
}

func providerResourceID(provider deploymentpb.Provider, nodeID string) string {
	switch provider {
	case deploymentpb.Provider_PROVIDER_DOCKER:
		return "container-" + nodeID
	case deploymentpb.Provider_PROVIDER_YANDEX:
		return "vm-" + nodeID
	default:
		return "resource-" + nodeID
	}
}
