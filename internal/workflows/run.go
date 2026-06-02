package workflows

import (
	"errors"

	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"go.temporal.io/sdk/workflow"
)

type runWorkflows struct{}

func (w *runWorkflows) TestRunWorkflow(_ workflow.Context, input *workflowpb.TestRunWorkflowWorkflowInput) (workflowpb.TestRunWorkflowWorkflow, error) {
	return &testRunWorkflow{req: input.Req}, nil
}

type testRunWorkflow struct {
	req *workflowpb.RunConfig
}

func (w *testRunWorkflow) Execute(ctx workflow.Context) error {
	if w.req == nil {
		return errors.New("run config is required")
	}
	if err := w.req.Validate(); err != nil {
		return err
	}

	infrastructureState := w.req.GetInfrastructureState()
	if len(infrastructureState.GetMachines()) == 0 {
		resp, err := workflowpb.ProcessInfrastructureWorkflowChild(ctx, &workflowpb.ProcessInfrastructureWorkflowRequest{
			Plan: w.req.GetInfrastructurePlan(),
		})
		if err != nil {
			return err
		}
		infrastructureState = resp.GetState()
	}

	deploymentPlan := w.req.GetDeploymentPlan()
	if len(deploymentPlan.GetComponents()) == 0 {
		resp, err := workflowpb.RenderDeploymentPlanWorkflowChild(ctx, &workflowpb.RenderDeploymentPlanWorkflowRequest{
			TopologySpec:        w.req.GetTopologySpec(),
			InfrastructurePlan:  w.req.GetInfrastructurePlan(),
			InfrastructureState: infrastructureState,
			RenderOverrides:     w.req.GetRenderOverrides(),
			Database:            w.req.GetDatabase(),
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

	_, err := workflowpb.RunWorkloadWorkflowChild(ctx, &workflowpb.RunWorkloadWorkflowRequest{
		TopologySpec:        w.req.GetTopologySpec(),
		Workload:            w.req.GetWorkload(),
		InfrastructureState: infrastructureState,
	})
	return err
}
