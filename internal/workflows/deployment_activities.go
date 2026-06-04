package workflows

import (
	"context"
	"fmt"
	"time"

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

type NetworkManager interface {
	Reserve(ctx context.Context, tenantID, runID, workflowID string, plan *deploymentpb.InfrastructurePlan) (string, error)
	Commit(ctx context.Context, tenantID, runID string) (string, error)
	Release(ctx context.Context, tenantID, runID string) (uint32, error)
}

type deploymentActivities struct {
	quotas   QuotaManager
	networks NetworkManager
}

func NewDeploymentActivities(quotas QuotaManager, networks NetworkManager) workflowpb.DeploymentServiceActivities {
	return &deploymentActivities{quotas: quotas, networks: networks}
}

func (a *deploymentActivities) AcquireNetworkActivity(ctx context.Context, req *workflowpb.AcquireNetworkActivityRequest) (*workflowpb.AcquireNetworkActivityResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if a.networks == nil || req.GetPlan().GetProvider() != deploymentpb.Provider_PROVIDER_YANDEX {
		return &workflowpb.AcquireNetworkActivityResponse{}, nil
	}
	info := activity.GetInfo(ctx)
	cidr, err := a.networks.Reserve(ctx, req.GetTenantId(), req.GetRunId(), info.WorkflowExecution.ID, req.GetPlan())
	if err != nil {
		return nil, err
	}
	return &workflowpb.AcquireNetworkActivityResponse{NetworkCidr: cidr}, nil
}

func (a *deploymentActivities) CommitNetworkActivity(ctx context.Context, req *workflowpb.CommitNetworkActivityRequest) (*workflowpb.CommitNetworkActivityResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if a.networks == nil {
		return &workflowpb.CommitNetworkActivityResponse{}, nil
	}
	cidr, err := a.networks.Commit(ctx, req.GetTenantId(), req.GetRunId())
	if err != nil {
		return nil, err
	}
	return &workflowpb.CommitNetworkActivityResponse{NetworkCidr: cidr}, nil
}

func (a *deploymentActivities) ReleaseNetworkActivity(ctx context.Context, req *workflowpb.ReleaseNetworkActivityRequest) (*workflowpb.ReleaseNetworkActivityResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if a.networks == nil {
		return &workflowpb.ReleaseNetworkActivityResponse{}, nil
	}
	released, err := a.networks.Release(ctx, req.GetTenantId(), req.GetRunId())
	if err != nil {
		return nil, err
	}
	return &workflowpb.ReleaseNetworkActivityResponse{Released: released}, nil
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
	return withHeartbeat(ctx, func() (*deploymentpb.Docker_Output, error) {
		return executor.Pull(ctx, req)
	})
}

func (a *deploymentActivities) DockerUpActivity(ctx context.Context, req *deploymentpb.Docker_Input) (*deploymentpb.Docker_Output, error) {
	executor, err := dockerexec.NewExecutor()
	if err != nil {
		return nil, err
	}
	defer executor.Close()
	return withHeartbeat(ctx, func() (*deploymentpb.Docker_Output, error) {
		return executor.Up(ctx, req)
	})
}

func (a *deploymentActivities) DockerDownActivity(ctx context.Context, req *deploymentpb.Docker_Input) (*deploymentpb.Docker_Output, error) {
	executor, err := dockerexec.NewExecutor()
	if err != nil {
		return nil, err
	}
	defer executor.Close()
	return withHeartbeat(ctx, func() (*deploymentpb.Docker_Output, error) {
		return executor.Down(ctx, req)
	})
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
	return withHeartbeat(ctx, func() (*deploymentpb.Terraform_Output, error) {
		return executor.Execute(ctx, clone)
	})
}

// withHeartbeat records a Temporal activity heartbeat every 20s for the duration
// of fn. Long-running deployment activities (terraform apply/destroy, which can
// run for minutes) MUST heartbeat: with HeartbeatTimeout=1m and no heartbeats,
// Temporal declares the activity timed-out and retries it while the first
// terraform process is still running, colliding on the terraform state lock and
// failing the run (while leaking the half-applied infrastructure).
func withHeartbeat[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	beatCtx, stop := context.WithCancel(ctx)
	defer stop()
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-beatCtx.Done():
				return
			case <-ticker.C:
				activity.RecordHeartbeat(ctx)
			}
		}
	}()
	return fn()
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
