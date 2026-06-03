package workflows

import (
	"context"
	"fmt"

	dockerexec "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/docker"
	terraformexec "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"go.temporal.io/sdk/activity"
	"google.golang.org/protobuf/proto"
)

type QuotaManager interface {
	Reserve(ctx context.Context, tenantID, runID, workflowID string, plan *deploymentpb.InfrastructurePlan, refs []*workflowpb.QuotaRequestRef) ([]*workflowpb.QuotaAllocationRef, error)
	Commit(ctx context.Context, tenantID, runID string) ([]*workflowpb.QuotaAllocationRef, error)
	Release(ctx context.Context, tenantID, runID string) (uint32, error)
}

type deploymentActivities struct {
	quotas QuotaManager
}

func NewDeploymentActivities(quotas QuotaManager) workflowpb.DeploymentServiceActivities {
	return &deploymentActivities{quotas: quotas}
}

func (a *deploymentActivities) AcquireNetworkActivity(_ context.Context, req *workflowpb.AcquireNetworkActivityRequest) (*workflowpb.AcquireNetworkActivityResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	return &workflowpb.AcquireNetworkActivityResponse{}, nil
}

func (a *deploymentActivities) AcquireQuotasActivity(ctx context.Context, req *workflowpb.AcquireQuotasActivityRequest) (*workflowpb.AcquireQuotasActivityResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if a.quotas != nil {
		info := activity.GetInfo(ctx)
		allocations, err := a.quotas.Reserve(ctx, req.GetTenantId(), req.GetRunId(), info.WorkflowExecution.ID, req.GetPlan(), req.GetQuotaRequests())
		if err != nil {
			return nil, err
		}
		return &workflowpb.AcquireQuotasActivityResponse{QuotaAllocations: allocations}, nil
	}
	allocations := echoQuotaAllocations(req.GetQuotaRequests())
	return &workflowpb.AcquireQuotasActivityResponse{QuotaAllocations: allocations}, nil
}

func (a *deploymentActivities) CommitQuotasActivity(ctx context.Context, req *workflowpb.CommitQuotasActivityRequest) (*workflowpb.CommitQuotasActivityResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if a.quotas == nil {
		return &workflowpb.CommitQuotasActivityResponse{}, nil
	}
	allocations, err := a.quotas.Commit(ctx, req.GetTenantId(), req.GetRunId())
	if err != nil {
		return nil, err
	}
	return &workflowpb.CommitQuotasActivityResponse{QuotaAllocations: allocations}, nil
}

func (a *deploymentActivities) ReleaseQuotasActivity(ctx context.Context, req *workflowpb.ReleaseQuotasActivityRequest) (*workflowpb.ReleaseQuotasActivityResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if a.quotas == nil {
		return &workflowpb.ReleaseQuotasActivityResponse{}, nil
	}
	released, err := a.quotas.Release(ctx, req.GetTenantId(), req.GetRunId())
	if err != nil {
		return nil, err
	}
	return &workflowpb.ReleaseQuotasActivityResponse{Released: released}, nil
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

func echoQuotaAllocations(refs []*workflowpb.QuotaRequestRef) []*workflowpb.QuotaAllocationRef {
	allocations := make([]*workflowpb.QuotaAllocationRef, 0, len(refs))
	for _, ref := range refs {
		request := ref.GetRequest()
		if request == nil {
			continue
		}
		allocations = append(allocations, &workflowpb.QuotaAllocationRef{
			NodeId: ref.GetNodeId(),
			Allocation: &deploymentpb.Quota_Allocation{
				Info: request.GetInfo(),
				Used: request.GetRequest(),
			},
		})
	}
	return allocations
}
