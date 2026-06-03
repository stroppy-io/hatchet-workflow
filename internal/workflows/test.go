package workflows

import (
	"errors"
	"fmt"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"go.temporal.io/sdk/temporal"
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

func (w *domainTestWorkflow) Execute(ctx workflow.Context) (resp *workflowpb.TestWorkflowResponse, err error) {
	var (
		infrastructureState *deploymentpb.InfrastructureState
		deploymentPlan      *deploymentpb.DeploymentPlan
		quotaReserved       bool
		quotaCommitted      bool
		networkReserved     bool
		networkCommitted    bool
		infrastructureReady bool
		quotaAllocations    []*workflowpb.QuotaAllocationRef
	)
	defer func() {
		if err != nil && !infrastructureReady && ((quotaReserved && !quotaCommitted) || (networkReserved && !networkCommitted)) {
			releaseCtx := ctx
			if temporal.IsCanceledError(err) {
				releaseCtx, _ = workflow.NewDisconnectedContext(ctx)
			}
			if quotaReserved && !quotaCommitted {
				if _, perr := workflowpb.ReleaseQuotasActivity(releaseCtx, &workflowpb.ReleaseQuotasActivityRequest{
					TenantId: w.req.GetTenantId(),
					RunId:    w.req.GetTestRun().GetId(),
				}); perr != nil {
					err = fmt.Errorf("%w; release quotas: %v", err, perr)
				}
			}
			if networkReserved && !networkCommitted {
				if _, perr := workflowpb.ReleaseNetworkActivity(releaseCtx, &workflowpb.ReleaseNetworkActivityRequest{
					TenantId: w.req.GetTenantId(),
					RunId:    w.req.GetTestRun().GetId(),
				}); perr != nil {
					err = fmt.Errorf("%w; release network: %v", err, perr)
				}
			}
		}
		if err == nil || !temporal.IsCanceledError(err) {
			return
		}
		w.cancel(ctx)
		disconnected, _ := workflow.NewDisconnectedContext(ctx)
		if perr := w.persist(disconnected, infrastructureState, deploymentPlan); perr != nil {
			err = fmt.Errorf("persist cancelled run state: %w", perr)
		}
	}()

	if w.req == nil {
		return nil, errors.New("test workflow request is required")
	}
	if err := w.req.Validate(); err != nil {
		w.fail(ctx)
		if perr := w.persist(ctx, infrastructureState, deploymentPlan); perr != nil {
			return nil, fmt.Errorf("persist failed run state: %w", perr)
		}
		return nil, err
	}

	testRun := w.req.GetTestRun()
	infrastructurePlan := proto.Clone(testRun.GetInfrastructurePlan()).(*deploymentpb.InfrastructurePlan)
	w.state.Status = common.Status_STATUS_RUNNING

	w.startStage(ctx, 0)
	if err := w.persist(ctx, infrastructureState, deploymentPlan); err != nil {
		return nil, err
	}
	quotaResp, err := workflowpb.CalculateQuotasWorkflowChild(ctx, &workflowpb.CalculateQuotasWorkflowRequest{
		Plan: infrastructurePlan,
	})
	if err != nil {
		w.failStage(ctx, 0)
		if perr := w.persist(ctx, infrastructureState, deploymentPlan); perr != nil {
			return nil, fmt.Errorf("persist failed run state: %w", perr)
		}
		return nil, err
	}
	acquireResp, err := workflowpb.AcquireQuotasActivity(ctx, &workflowpb.AcquireQuotasActivityRequest{
		TenantId:      w.req.GetTenantId(),
		RunId:         testRun.GetId(),
		Plan:          quotaResp.GetPlan(),
		QuotaRequests: quotaResp.GetQuotaRequests(),
	})
	if err != nil {
		w.failStage(ctx, 0)
		if perr := w.persist(ctx, infrastructureState, deploymentPlan); perr != nil {
			return nil, fmt.Errorf("persist failed run state: %w", perr)
		}
		return nil, err
	}
	quotaReserved = len(acquireResp.GetQuotaAllocations()) > 0
	quotaAllocations = acquireResp.GetQuotaAllocations()
	if quotaResp.GetPlan().GetProvider() == deploymentpb.Provider_PROVIDER_YANDEX {
		networkResp, err := workflowpb.AcquireNetworkActivity(ctx, &workflowpb.AcquireNetworkActivityRequest{
			TenantId: w.req.GetTenantId(),
			RunId:    testRun.GetId(),
			Plan:     quotaResp.GetPlan(),
		})
		if err != nil {
			w.failStage(ctx, 0)
			if perr := w.persist(ctx, infrastructureState, deploymentPlan); perr != nil {
				return nil, fmt.Errorf("persist failed run state: %w", perr)
			}
			return nil, err
		}
		if networkResp.GetNetworkCidr() != "" {
			infrastructurePlan = planWithReservedNetworkCIDR(quotaResp.GetPlan(), networkResp.GetNetworkCidr())
			networkReserved = true
		}
	} else {
		infrastructurePlan = quotaResp.GetPlan()
	}
	infrastructureResp, err := workflowpb.ProcessInfrastructureWorkflowChild(ctx, &workflowpb.ProcessInfrastructureWorkflowRequest{
		RunId:          testRun.GetId(),
		Plan:           infrastructurePlan,
		AgentBootstrap: w.req.GetAgentBootstrap(),
	})
	if err != nil {
		w.failStage(ctx, 0)
		if perr := w.persist(ctx, infrastructureState, deploymentPlan); perr != nil {
			return nil, fmt.Errorf("persist failed run state: %w", perr)
		}
		return nil, err
	}
	infrastructureState = infrastructureResp.GetState()
	infrastructureReady = true
	commitResp, err := workflowpb.CommitQuotasActivity(ctx, &workflowpb.CommitQuotasActivityRequest{
		TenantId: w.req.GetTenantId(),
		RunId:    testRun.GetId(),
	})
	if err != nil {
		w.failStage(ctx, 0)
		if perr := w.persist(ctx, infrastructureState, deploymentPlan); perr != nil {
			return nil, fmt.Errorf("persist failed run state: %w", perr)
		}
		return nil, err
	}
	if len(commitResp.GetQuotaAllocations()) > 0 {
		quotaAllocations = commitResp.GetQuotaAllocations()
	}
	quotaCommitted = true
	if networkReserved {
		if _, err := workflowpb.CommitNetworkActivity(ctx, &workflowpb.CommitNetworkActivityRequest{
			TenantId: w.req.GetTenantId(),
			RunId:    testRun.GetId(),
		}); err != nil {
			w.failStage(ctx, 0)
			if perr := w.persist(ctx, infrastructureState, deploymentPlan); perr != nil {
				return nil, fmt.Errorf("persist failed run state: %w", perr)
			}
			return nil, err
		}
		networkCommitted = true
	}
	attachQuotaAllocations(infrastructureState, quotaAllocations)
	w.completeStage(ctx, 0)
	if err := w.persist(ctx, infrastructureState, deploymentPlan); err != nil {
		return nil, err
	}

	w.startStage(ctx, 1)
	if err := w.persist(ctx, infrastructureState, deploymentPlan); err != nil {
		return nil, err
	}
	renderResp, err := workflowpb.RenderDeploymentPlanWorkflowChild(ctx, &workflowpb.RenderDeploymentPlanWorkflowRequest{
		TopologySpec:        testRun.GetTopologySpec(),
		InfrastructurePlan:  infrastructurePlan,
		InfrastructureState: infrastructureState,
		RenderOverrides:     testRun.GetRenderOverrides(),
		Database:            testRun.GetDatabase(),
		Workload:            testRun.GetWorkload(),
		AgentBootstrap:      w.req.GetAgentBootstrap(),
	})
	if err != nil {
		w.failStage(ctx, 1)
		if perr := w.persist(ctx, infrastructureState, deploymentPlan); perr != nil {
			return nil, fmt.Errorf("persist failed run state: %w", perr)
		}
		return nil, err
	}
	deploymentPlan = renderResp.GetDeploymentPlan()
	w.completeStage(ctx, 1)
	if err := w.persist(ctx, infrastructureState, deploymentPlan); err != nil {
		return nil, err
	}

	w.startStage(ctx, 2)
	if err := w.persist(ctx, infrastructureState, deploymentPlan); err != nil {
		return nil, err
	}
	executeResp, err := workflowpb.ExecuteDeploymentPlanWorkflowChild(ctx, &workflowpb.ExecuteDeploymentPlanWorkflowRequest{
		DeploymentPlan:      deploymentPlan,
		InfrastructureState: infrastructureState,
		RunId:               testRun.GetId(),
		AgentBootstrap:      w.req.GetAgentBootstrap(),
	})
	if err != nil {
		w.failStage(ctx, 2)
		if perr := w.persist(ctx, infrastructureState, deploymentPlan); perr != nil {
			return nil, fmt.Errorf("persist failed run state: %w", perr)
		}
		return nil, err
	}
	deploymentPlan = executeResp.GetDeploymentPlan()
	w.completeStage(ctx, 2)
	if err := w.persist(ctx, infrastructureState, deploymentPlan); err != nil {
		return nil, err
	}

	w.startStage(ctx, 3)
	if err := w.persist(ctx, infrastructureState, deploymentPlan); err != nil {
		return nil, err
	}
	if _, err := workflowpb.RunWorkloadWorkflowChild(ctx, &workflowpb.RunWorkloadWorkflowRequest{
		TopologySpec:        testRun.GetTopologySpec(),
		Workload:            testRun.GetWorkload(),
		InfrastructureState: infrastructureState,
	}); err != nil {
		w.failStage(ctx, 3)
		if perr := w.persist(ctx, infrastructureState, deploymentPlan); perr != nil {
			return nil, fmt.Errorf("persist failed run state: %w", perr)
		}
		return nil, err
	}
	w.completeStage(ctx, 3)

	w.state.Status = common.Status_STATUS_COMPLETED
	if err := w.persist(ctx, infrastructureState, deploymentPlan); err != nil {
		return nil, err
	}
	return &workflowpb.TestWorkflowResponse{}, nil
}

func attachQuotaAllocations(state *deploymentpb.InfrastructureState, refs []*workflowpb.QuotaAllocationRef) {
	if state == nil || len(refs) == 0 {
		return
	}
	byNode := make(map[string][]*deploymentpb.Quota_Allocation)
	for _, ref := range refs {
		allocation := ref.GetAllocation()
		if allocation == nil {
			continue
		}
		byNode[ref.GetNodeId()] = append(byNode[ref.GetNodeId()], proto.Clone(allocation).(*deploymentpb.Quota_Allocation))
	}
	for _, machine := range state.GetMachines() {
		allocations := byNode[machine.GetNodeId()]
		if len(allocations) == 0 {
			continue
		}
		machine.AllocatedQuotas = allocations
	}
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

func (w *domainTestWorkflow) cancel(ctx workflow.Context) {
	w.state.Status = common.Status_STATUS_CANCELLED
	now := timestamppb.New(workflow.Now(ctx))
	for _, stage := range w.state.GetStages() {
		if stage.GetStatus() == common.Status_STATUS_RUNNING || stage.GetStatus() == common.Status_STATUS_PENDING {
			stage.Status = common.Status_STATUS_CANCELLED
			stage.FinishedAt = now
			return
		}
	}
}

func (w *domainTestWorkflow) persist(ctx workflow.Context, infrastructureState *deploymentpb.InfrastructureState, deploymentPlan *deploymentpb.DeploymentPlan) error {
	return persistRunState(ctx, w.req.GetTestRun().GetId(), w.state, infrastructureState, deploymentPlan)
}

func stage(name string) *workflowpb.Stage {
	return &workflowpb.Stage{
		NodeExecutionId: deploymentbuilder.StageExecutionID(name),
		Name:            name,
		Status:          common.Status_STATUS_PENDING,
		Attempt:         1,
	}
}
