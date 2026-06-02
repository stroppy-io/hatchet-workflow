package workflows

import (
	"errors"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	stageInfrastructure = "infrastructure"
	stageRenderPlan     = "render_deployment_plan"
	stageExecutePlan    = "execute_deployment_plan"
	stageWorkload       = "workload"
)

type testWorkflows struct{}

func (w *testWorkflows) InstallDatabaseWorkflow(_ workflow.Context, input *workflowpb.InstallDatabaseWorkflowWorkflowInput) (workflowpb.InstallDatabaseWorkflowWorkflow, error) {
	return &installDatabaseWorkflow{req: input.Req}, nil
}

func (w *testWorkflows) InstallStroppyWorkflow(_ workflow.Context, input *workflowpb.InstallStroppyWorkflowWorkflowInput) (workflowpb.InstallStroppyWorkflowWorkflow, error) {
	return &installStroppyWorkflow{req: input.Req}, nil
}

func (w *testWorkflows) RunWorkloadWorkflow(_ workflow.Context, input *workflowpb.RunWorkloadWorkflowWorkflowInput) (workflowpb.RunWorkloadWorkflowWorkflow, error) {
	return &runWorkloadWorkflow{req: input.Req}, nil
}

func (w *testWorkflows) TestWorkflow(_ workflow.Context, input *workflowpb.TestWorkflowWorkflowInput) (workflowpb.TestWorkflowWorkflow, error) {
	return newDomainTestWorkflow(input.Req), nil
}

type installDatabaseWorkflow struct {
	req *workflowpb.InstallDatabaseWorkflowRequest
}

func (w *installDatabaseWorkflow) Execute(workflow.Context) (*workflowpb.InstallDatabaseWorkflowResponse, error) {
	if w.req == nil {
		return nil, errors.New("install database request is required")
	}
	if err := w.req.Validate(); err != nil {
		return nil, err
	}
	return &workflowpb.InstallDatabaseWorkflowResponse{}, nil
}

type installStroppyWorkflow struct {
	req *workflowpb.InstallStroppyWorkflowRequest
}

func (w *installStroppyWorkflow) Execute(workflow.Context) (*workflowpb.InstallStroppyWorkflowResponse, error) {
	if w.req == nil {
		return nil, errors.New("install stroppy request is required")
	}
	if err := w.req.Validate(); err != nil {
		return nil, err
	}
	return &workflowpb.InstallStroppyWorkflowResponse{}, nil
}

type runWorkloadWorkflow struct {
	req *workflowpb.RunWorkloadWorkflowRequest
}

func (w *runWorkloadWorkflow) Execute(workflow.Context) (*workflowpb.RunWorkloadWorkflowResponse, error) {
	if w.req == nil {
		return nil, errors.New("run workload request is required")
	}
	if err := w.req.Validate(); err != nil {
		return nil, err
	}
	return &workflowpb.RunWorkloadWorkflowResponse{}, nil
}

type domainTestWorkflow struct {
	req   *workflowpb.TestWorkflowRequest
	state *workflowpb.RunState
}

func newDomainTestWorkflow(req *workflowpb.TestWorkflowRequest) *domainTestWorkflow {
	return &domainTestWorkflow{
		req: req,
		state: &workflowpb.RunState{
			Status: common.Status_STATUS_PENDING,
			Stages: []*workflowpb.Stage{
				stage(stageInfrastructure),
				stage(stageRenderPlan),
				stage(stageExecutePlan),
				stage(stageWorkload),
			},
		},
	}
}

func (w *domainTestWorkflow) Execute(ctx workflow.Context) (*workflowpb.TestWorkflowResponse, error) {
	if w.req == nil {
		return nil, errors.New("test workflow request is required")
	}
	if err := w.req.Validate(); err != nil {
		w.fail(ctx)
		return nil, err
	}

	testRun := w.req.GetTestRun()
	w.state.Status = common.Status_STATUS_RUNNING

	w.startStage(ctx, 0)
	infrastructureResp, err := workflowpb.ProcessInfrastructureWorkflowChild(ctx, &workflowpb.ProcessInfrastructureWorkflowRequest{
		RunId:          testRun.GetId(),
		Plan:           testRun.GetInfrastructurePlan(),
		AgentBootstrap: w.req.GetAgentBootstrap(),
	})
	if err != nil {
		w.failStage(ctx, 0)
		return nil, err
	}
	w.completeStage(ctx, 0)

	w.startStage(ctx, 1)
	renderResp, err := workflowpb.RenderDeploymentPlanWorkflowChild(ctx, &workflowpb.RenderDeploymentPlanWorkflowRequest{
		TopologySpec:        testRun.GetTopologySpec(),
		InfrastructurePlan:  testRun.GetInfrastructurePlan(),
		InfrastructureState: infrastructureResp.GetState(),
		RenderOverrides:     testRun.GetRenderOverrides(),
		Database:            testRun.GetDatabase(),
		Workload:            testRun.GetWorkload(),
	})
	if err != nil {
		w.failStage(ctx, 1)
		return nil, err
	}
	w.completeStage(ctx, 1)

	w.startStage(ctx, 2)
	if _, err := workflowpb.ExecuteDeploymentPlanWorkflowChild(ctx, &workflowpb.ExecuteDeploymentPlanWorkflowRequest{
		DeploymentPlan:      renderResp.GetDeploymentPlan(),
		InfrastructureState: infrastructureResp.GetState(),
	}); err != nil {
		w.failStage(ctx, 2)
		return nil, err
	}
	w.completeStage(ctx, 2)

	w.startStage(ctx, 3)
	if _, err := workflowpb.RunWorkloadWorkflowChild(ctx, &workflowpb.RunWorkloadWorkflowRequest{
		TopologySpec:        testRun.GetTopologySpec(),
		Workload:            testRun.GetWorkload(),
		InfrastructureState: infrastructureResp.GetState(),
	}); err != nil {
		w.failStage(ctx, 3)
		return nil, err
	}
	w.completeStage(ctx, 3)

	w.state.Status = common.Status_STATUS_COMPLETED
	return &workflowpb.TestWorkflowResponse{}, nil
}

func (w *domainTestWorkflow) GetRunState() (*workflowpb.RunState, error) {
	return proto.Clone(w.state).(*workflowpb.RunState), nil
}

func (w *domainTestWorkflow) startStage(ctx workflow.Context, index int) {
	w.state.Stages[index].Status = common.Status_STATUS_RUNNING
	w.state.Stages[index].StartedAt = timestamppb.New(workflow.Now(ctx))
}

func (w *domainTestWorkflow) completeStage(ctx workflow.Context, index int) {
	w.state.Stages[index].Status = common.Status_STATUS_COMPLETED
	w.state.Stages[index].FinishedAt = timestamppb.New(workflow.Now(ctx))
}

func (w *domainTestWorkflow) failStage(ctx workflow.Context, index int) {
	w.state.Status = common.Status_STATUS_FAILED
	w.state.Stages[index].Status = common.Status_STATUS_FAILED
	w.state.Stages[index].FinishedAt = timestamppb.New(workflow.Now(ctx))
}

func (w *domainTestWorkflow) fail(ctx workflow.Context) {
	w.state.Status = common.Status_STATUS_FAILED
	now := timestamppb.New(workflow.Now(ctx))
	for _, stage := range w.state.GetStages() {
		if stage.GetStatus() == common.Status_STATUS_PENDING || stage.GetStatus() == common.Status_STATUS_RUNNING {
			stage.Status = common.Status_STATUS_FAILED
			stage.FinishedAt = now
			return
		}
	}
}

func stage(name string) *workflowpb.Stage {
	return &workflowpb.Stage{
		Name:    name,
		Status:  common.Status_STATUS_PENDING,
		Attempt: 1,
	}
}
