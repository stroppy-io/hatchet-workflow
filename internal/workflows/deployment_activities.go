package workflows

import (
	"context"
	"fmt"

	dockerexec "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/docker"
	terraformexec "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"google.golang.org/protobuf/proto"
)

type deploymentActivities struct{}

func NewDeploymentActivities() workflowpb.DeploymentServiceActivities {
	return &deploymentActivities{}
}

func (a *deploymentActivities) AcquireNetworkActivity(_ context.Context, req *workflowpb.AcquireNetworkActivityRequest) (*workflowpb.AcquireNetworkActivityResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	return &workflowpb.AcquireNetworkActivityResponse{}, nil
}

func (a *deploymentActivities) AcquireQuotasActivity(_ context.Context, req *workflowpb.AcquireQuotasActivityRequest) (*workflowpb.AcquireQuotasActivityResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	allocations := make(map[string]*deploymentpb.Quota_Allocation, len(req.GetQuotaRequests()))
	for nodeID, request := range req.GetQuotaRequests() {
		allocations[nodeID] = &deploymentpb.Quota_Allocation{
			Info: request.GetInfo(),
			Used: request.GetRequest(),
		}
	}
	return &workflowpb.AcquireQuotasActivityResponse{QuotaAllocations: allocations}, nil
}

func (a *deploymentActivities) DockerPullActivity(ctx context.Context, req *deploymentpb.Docker_Input) (*deploymentpb.Docker_Output, error) {
	executor, err := dockerexec.NewExecutor()
	if err != nil {
		return nil, err
	}
	defer executor.Close()
	return executor.Pull(ctx, req)
}

func (a *deploymentActivities) DockerUpActivity(ctx context.Context, req *deploymentpb.Docker_Input) (*deploymentpb.Docker_Output, error) {
	executor, err := dockerexec.NewExecutor()
	if err != nil {
		return nil, err
	}
	defer executor.Close()
	return executor.Up(ctx, req)
}

func (a *deploymentActivities) DockerDownActivity(ctx context.Context, req *deploymentpb.Docker_Input) (*deploymentpb.Docker_Output, error) {
	executor, err := dockerexec.NewExecutor()
	if err != nil {
		return nil, err
	}
	defer executor.Close()
	return executor.Down(ctx, req)
}

func (a *deploymentActivities) TerraformPlanActivity(ctx context.Context, req *deploymentpb.Terraform_Input) (*deploymentpb.Terraform_Output, error) {
	return runTerraformActivity(ctx, req, deploymentpb.Terraform_ACTION_PLAN)
}

func (a *deploymentActivities) TerraformApplyActivity(ctx context.Context, req *deploymentpb.Terraform_Input) (*deploymentpb.Terraform_Output, error) {
	return runTerraformActivity(ctx, req, deploymentpb.Terraform_ACTION_APPLY)
}

func (a *deploymentActivities) TerraformDestroyActivity(ctx context.Context, req *deploymentpb.Terraform_Input) (*deploymentpb.Terraform_Output, error) {
	return runTerraformActivity(ctx, req, deploymentpb.Terraform_ACTION_DESTROY)
}

func runTerraformActivity(ctx context.Context, req *deploymentpb.Terraform_Input, action deploymentpb.Terraform_Action) (*deploymentpb.Terraform_Output, error) {
	if req == nil {
		return nil, fmt.Errorf("terraform input is required")
	}
	clone := proto.Clone(req).(*deploymentpb.Terraform_Input)
	if clone.Operation == nil {
		clone.Operation = &deploymentpb.Terraform_Operation{}
	}
	clone.Operation.Action = action
	executor := terraformexec.NewExecutor()
	return executor.Execute(ctx, clone)
}
