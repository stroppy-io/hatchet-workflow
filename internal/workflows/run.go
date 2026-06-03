package workflows

import (
	"errors"
	"fmt"

	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type runWorkflows struct{}

func (w *runWorkflows) TestRunWorkflow(_ workflow.Context, input *workflowpb.TestRunWorkflowWorkflowInput) (workflowpb.TestRunWorkflowWorkflow, error) {
	return &testRunWorkflow{req: input.Req}, nil
}

type testRunWorkflow struct {
	req *workflowpb.RunConfig
}

func (w *testRunWorkflow) Execute(ctx workflow.Context) (err error) {
	if w.req == nil {
		return errors.New("run config is required")
	}
	if err := w.req.Validate(); err != nil {
		return err
	}

	infrastructureState := w.req.GetInfrastructureState()
	quotaReserved := false
	quotaCommitted := false
	defer func() {
		if err == nil || !quotaReserved || quotaCommitted {
			return
		}
		releaseCtx := ctx
		if temporal.IsCanceledError(err) {
			releaseCtx, _ = workflow.NewDisconnectedContext(ctx)
		}
		if _, perr := workflowpb.ReleaseQuotasActivity(releaseCtx, &workflowpb.ReleaseQuotasActivityRequest{
			TenantId: w.req.GetTenantId(),
			RunId:    w.req.GetId(),
		}); perr != nil {
			err = fmt.Errorf("%w; release quotas: %v", err, perr)
		}
	}()

	if len(infrastructureState.GetMachines()) == 0 {
		quotaResp, err := workflowpb.CalculateQuotasWorkflowChild(ctx, &workflowpb.CalculateQuotasWorkflowRequest{
			Plan: w.req.GetInfrastructurePlan(),
		})
		if err != nil {
			return err
		}
		acquireResp, err := workflowpb.AcquireQuotasActivity(ctx, &workflowpb.AcquireQuotasActivityRequest{
			TenantId:      w.req.GetTenantId(),
			RunId:         w.req.GetId(),
			Plan:          quotaResp.GetPlan(),
			QuotaRequests: quotaResp.GetQuotaRequests(),
		})
		if err != nil {
			return err
		}
		quotaReserved = len(acquireResp.GetQuotaAllocations()) > 0
		quotaAllocations := acquireResp.GetQuotaAllocations()
		resp, err := workflowpb.ProcessInfrastructureWorkflowChild(ctx, &workflowpb.ProcessInfrastructureWorkflowRequest{
			RunId:          w.req.GetId(),
			Plan:           quotaResp.GetPlan(),
			AgentBootstrap: w.req.GetAgentBootstrap(),
		})
		if err != nil {
			return err
		}
		infrastructureState = resp.GetState()
		commitResp, err := workflowpb.CommitQuotasActivity(ctx, &workflowpb.CommitQuotasActivityRequest{
			TenantId: w.req.GetTenantId(),
			RunId:    w.req.GetId(),
		})
		if err != nil {
			return err
		}
		if len(commitResp.GetQuotaAllocations()) > 0 {
			quotaAllocations = commitResp.GetQuotaAllocations()
		}
		attachQuotaAllocations(infrastructureState, quotaAllocations)
		quotaCommitted = true
	}

	var deploymentPlan *deploymentpb.DeploymentPlan = w.req.GetDeploymentPlan()
	if len(deploymentPlan.GetComponents()) == 0 {
		resp, err := workflowpb.RenderDeploymentPlanWorkflowChild(ctx, &workflowpb.RenderDeploymentPlanWorkflowRequest{
			TopologySpec:        w.req.GetTopologySpec(),
			InfrastructurePlan:  w.req.GetInfrastructurePlan(),
			InfrastructureState: infrastructureState,
			RenderOverrides:     w.req.GetRenderOverrides(),
			Database:            w.req.GetDatabase(),
			Workload:            w.req.GetWorkload(),
		})
		if err != nil {
			return err
		}
		deploymentPlan = resp.GetDeploymentPlan()
	}

	if _, err := workflowpb.ExecuteDeploymentPlanWorkflowChild(ctx, &workflowpb.ExecuteDeploymentPlanWorkflowRequest{
		DeploymentPlan:      deploymentPlan,
		InfrastructureState: infrastructureState,
	}); err != nil {
		return err
	}

	_, err = workflowpb.RunWorkloadWorkflowChild(ctx, &workflowpb.RunWorkloadWorkflowRequest{
		TopologySpec:        w.req.GetTopologySpec(),
		Workload:            w.req.GetWorkload(),
		InfrastructureState: infrastructureState,
	})
	return err
}
