package workflows

import (
	"errors"
	"fmt"
	"time"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	workloadbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/workload"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
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
	stageTeardown       = "teardown"

	actionCalculateQuotas          = "calculate_quotas"
	actionAcquireQuotas            = "acquire_quotas"
	actionAcquireNetwork           = "acquire_network"
	actionProcessInfrastructure    = "process_infrastructure"
	actionCommitQuotas             = "commit_quotas"
	actionCommitNetwork            = "commit_network"
	actionRenderDockerInput        = "render_docker_input"
	actionDockerPull               = "docker_pull"
	actionDockerUp                 = "docker_up"
	actionDockerDown               = "docker_down"
	actionRenderTerraformVariables = "render_terraform_variables"
	actionTerraformApply           = "terraform_apply"
	actionTerraformDestroy         = "terraform_destroy"
	actionReleaseQuotas            = "release_quotas"
	actionReleaseNetwork           = "release_network"
)

const (
	stageInfrastructureIndex = iota
	stageRenderPlanIndex
	stageExecutePlanIndex
	stageWorkloadIndex
	stageTeardownIndex
)

const runtimeProjectionPersistMinInterval = 2 * time.Second

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
	return newDomainTestWorkflow(input.Req, input.UpdateStage), nil
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

func (w *runWorkloadWorkflow) Execute(ctx workflow.Context) (*workflowpb.RunWorkloadWorkflowResponse, error) {
	if w.req == nil {
		return nil, errors.New("run workload request is required")
	}
	if err := w.req.Validate(); err != nil {
		return nil, err
	}
	runID := w.req.GetRunId()
	plan := proto.Clone(w.req.GetDeploymentPlan()).(*deploymentpb.DeploymentPlan)
	deploymentbuilder.StampDeploymentPlanExecutionContext(runID, plan)

	component, err := workloadRunnerDeployment(plan)
	if err != nil {
		return nil, err
	}
	if !infrastructureHasNode(w.req.GetInfrastructureState(), component.GetNodeId()) {
		return nil, fmt.Errorf("workload runner node %q is missing from infrastructure state", component.GetNodeId())
	}
	taskQueue, err := agentTaskQueue(w.req.GetAgentBootstrap(), component.GetNodeId())
	if err != nil {
		return nil, err
	}

	activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           taskQueue,
		StartToCloseTimeout: 30 * 24 * time.Hour,
		HeartbeatTimeout:    time.Minute,
	})
	if err := executeActivityNoResult(activityCtx, workflowpb.EnsureAgentOnlineActivityActivityName); err != nil {
		return nil, fmt.Errorf("agent %s is not online: %w", component.GetNodeId(), err)
	}

	step := workloadRunStep(component)
	stampWorkloadStepExecutionContext(runID, component, step)
	started := timestamppb.New(workflow.Now(ctx))
	step.Status = common.Status_STATUS_RUNNING
	emitStageUpdate(ctx, runID, workloadAgentStepStage(component, step, common.Status_STATUS_RUNNING, started, nil, ""))
	if err := appendDeploymentStepLog(ctx, runID, component, step, monitor.Stream_STREAM_STDOUT, "started "+agentStepDescription(step)); err != nil {
		return nil, err
	}
	if err := executeAgentStep(activityCtx, step); err != nil {
		finished := timestamppb.New(workflow.Now(ctx))
		step.Status = common.Status_STATUS_FAILED
		emitStageUpdate(ctx, runID, workloadAgentStepStage(component, step, common.Status_STATUS_FAILED, started, finished, err.Error()))
		if lerr := appendDeploymentStepLog(ctx, runID, component, step, monitor.Stream_STREAM_STDERR, "failed "+agentStepDescription(step)+": "+err.Error()); lerr != nil {
			return nil, lerr
		}
		return nil, fmt.Errorf("workload runner %s step %s: %w", component.GetComponentId(), step.GetId(), err)
	}
	finished := timestamppb.New(workflow.Now(ctx))
	step.Status = common.Status_STATUS_COMPLETED
	emitStageUpdate(ctx, runID, workloadAgentStepStage(component, step, common.Status_STATUS_COMPLETED, started, finished, ""))
	if err := appendDeploymentStepLog(ctx, runID, component, step, monitor.Stream_STREAM_STDOUT, "completed "+agentStepDescription(step)); err != nil {
		return nil, err
	}
	return &workflowpb.RunWorkloadWorkflowResponse{}, nil
}

func workloadRunnerDeployment(plan *deploymentpb.DeploymentPlan) (*deploymentpb.ComponentDeployment, error) {
	if plan == nil {
		return nil, errors.New("deployment plan is required")
	}
	for _, component := range plan.GetComponents() {
		if component == nil {
			continue
		}
		labels := component.GetLabels()
		if labels["engine"] == workloadbuilder.Engine && labels["role"] == workloadbuilder.RunnerRole {
			return component, nil
		}
		if component.GetComponentId() == workloadbuilder.RunnerNodeID {
			return component, nil
		}
	}
	return nil, errors.New("deployment plan has no workload runner component")
}

func infrastructureHasNode(state *deploymentpb.InfrastructureState, nodeID string) bool {
	if state == nil || nodeID == "" {
		return false
	}
	for _, machine := range state.GetMachines() {
		if machine != nil && machine.GetNodeId() == nodeID {
			return true
		}
	}
	return false
}

func workloadRunStep(component *deploymentpb.ComponentDeployment) *deploymentpb.AgentStep {
	componentID := component.GetComponentId()
	configDir := deploymentbuilder.ConfigDir(componentID)
	configPath := configDir + "/stroppy-config.json"
	script := "set -e\n" +
		"if ! command -v stroppy >/dev/null 2>&1; then\n" +
		"  echo 'stroppy binary is not installed on workload runner' >&2\n" +
		"  exit 127\n" +
		"fi\n" +
		"exec stroppy run -f " + deploymentbuilder.ShellQuote(configPath) + "\n"
	step := deploymentbuilder.CallCmdStep("900_run_stroppy", 900, script)
	step.Tags = deploymentbuilder.Tags("workload", workloadbuilder.Engine, "command")
	step.Labels = deploymentbuilder.MergeLabels(step.GetLabels(), map[string]string{
		"engine": workloadbuilder.Engine,
		"role":   workloadbuilder.RunnerRole,
		"phase":  stageWorkload,
	})
	if cmd := step.GetCallCmd(); cmd != nil && cmd.GetSpec() != nil {
		cmd.Spec.Cwd = configDir
	}
	return step
}

func stampWorkloadStepExecutionContext(runID string, component *deploymentpb.ComponentDeployment, step *deploymentpb.AgentStep) {
	deploymentbuilder.StampAgentStepExecutionContext(runID, component, step)
	parentNodeExecutionID := deploymentbuilder.StageExecutionID(stageWorkload)
	if step.Labels == nil {
		step.Labels = map[string]string{}
	}
	step.Labels[deploymentbuilder.LabelPhase] = stageWorkload
	step.Labels[deploymentbuilder.LabelParentNodeExecutionID] = parentNodeExecutionID
	if cmd := step.GetCallCmd(); cmd != nil && cmd.GetSpec() != nil {
		if cmd.Spec.Env == nil {
			cmd.Spec.Env = map[string]string{}
		}
		cmd.Spec.Env[deploymentbuilder.EnvPhase] = stageWorkload
		cmd.Spec.Env[deploymentbuilder.EnvParentNodeExecutionID] = parentNodeExecutionID
	}
}

func workloadAgentStepStage(component *deploymentpb.ComponentDeployment, step *deploymentpb.AgentStep, status common.Status, started, finished *timestamppb.Timestamp, errText string) *workflowpb.Stage {
	return agentStepStageForPhase(component, step, 1, status, started, finished, errText, stageWorkload, deploymentbuilder.StageExecutionID(stageWorkload))
}

type domainTestWorkflow struct {
	req                            *workflowpb.TestWorkflowRequest
	stageUpdates                   *workflowpb.UpdateStageSignal
	state                          *workflowpb.RunState
	persistedInfrastructureState   *deploymentpb.InfrastructureState
	persistedDeploymentPlan        *deploymentpb.DeploymentPlan
	lastRuntimeProjectionPersistAt time.Time
}

func newDomainTestWorkflow(req *workflowpb.TestWorkflowRequest, stageUpdates *workflowpb.UpdateStageSignal) *domainTestWorkflow {
	return &domainTestWorkflow{
		req:          req,
		stageUpdates: stageUpdates,
		state: &workflowpb.RunState{
			Status: common.Status_STATUS_PENDING,
			Stages: []*workflowpb.Stage{
				rootStage(stageInfrastructure, 1),
				rootStage(stageRenderPlan, 2),
				rootStage(stageExecutePlan, 3),
				rootStage(stageWorkload, 4),
				rootStage(stageTeardown, 5),
			},
		},
	}
}

func (w *domainTestWorkflow) Execute(ctx workflow.Context) (resp *workflowpb.TestWorkflowResponse, err error) {
	w.listenStageUpdates(ctx)
	var (
		infrastructureState *deploymentpb.InfrastructureState
		deploymentPlan      *deploymentpb.DeploymentPlan
		quotaReserved       bool
		quotaCommitted      bool
		networkReserved     bool
		networkCommitted    bool
		infrastructureReady bool
		quotaAllocations    []*workflowpb.QuotaAllocationRef
		// teardownPlan is the infrastructure plan that was actually provisioned. It
		// is set once ProcessInfrastructure succeeds and drives the teardown that
		// MUST run on every terminal outcome (success, failure, cancel) so a run
		// never leaks containers/VMs.
		teardownPlan *deploymentpb.InfrastructurePlan
	)
	defer func() {
		if teardownPlan != nil {
			// Run teardown on a disconnected context so it completes even when the
			// workflow context is already cancelled.
			tctx, _ := workflow.NewDisconnectedContext(ctx)
			w.startStage(tctx, stageTeardownIndex)
			if perr := w.persist(tctx, infrastructureState, deploymentPlan); perr != nil && err == nil {
				err = fmt.Errorf("persist teardown run state: %w", perr)
			}
			if derr := w.teardownInfrastructure(tctx, teardownPlan); derr != nil {
				w.failStage(tctx, stageTeardownIndex)
				if err != nil {
					err = fmt.Errorf("%w; teardown infrastructure: %v", err, derr)
				} else {
					err = fmt.Errorf("teardown infrastructure: %w", derr)
				}
				if perr := w.persist(tctx, infrastructureState, deploymentPlan); perr != nil {
					err = fmt.Errorf("%w; persist teardown failed run state: %v", err, perr)
				}
			} else if rerr := w.releaseCommittedAllocations(tctx, quotaCommitted, networkCommitted); rerr != nil {
				w.failStage(tctx, stageTeardownIndex)
				if err != nil {
					err = fmt.Errorf("%w; release committed allocations: %v", err, rerr)
				} else {
					err = fmt.Errorf("release committed allocations: %w", rerr)
				}
				if perr := w.persist(tctx, infrastructureState, deploymentPlan); perr != nil {
					err = fmt.Errorf("%w; persist teardown failed run state: %v", err, perr)
				}
			} else {
				w.completeStage(tctx, stageTeardownIndex)
				if err == nil {
					w.state.Status = common.Status_STATUS_COMPLETED
				}
				if perr := w.persist(tctx, infrastructureState, deploymentPlan); perr != nil {
					if err != nil {
						err = fmt.Errorf("%w; persist teardown completed run state: %v", err, perr)
					} else {
						err = fmt.Errorf("persist teardown completed run state: %w", perr)
					}
				}
			}
		} else if err == nil && w.state.GetStatus() == common.Status_STATUS_RUNNING {
			w.state.Status = common.Status_STATUS_COMPLETED
			if perr := w.persist(ctx, infrastructureState, deploymentPlan); perr != nil {
				err = fmt.Errorf("persist completed run state: %w", perr)
			}
		}
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
	w.state.Stages[stageInfrastructureIndex].Outputs = deploymentbuilder.InfrastructurePlanOutputs(infrastructurePlan)
	w.state.Status = common.Status_STATUS_RUNNING

	w.startStage(ctx, stageInfrastructureIndex)
	if err := w.persist(ctx, infrastructureState, deploymentPlan); err != nil {
		return nil, err
	}
	infrastructureStageID := deploymentbuilder.StageExecutionID(stageInfrastructure)
	calculateQuotasStage := w.startActionStage(ctx, stageInfrastructure, infrastructureStageID, 1, actionCalculateQuotas)
	quotaResp, err := workflowpb.CalculateQuotasWorkflowChild(ctx, &workflowpb.CalculateQuotasWorkflowRequest{
		Plan: infrastructurePlan,
	})
	if err != nil {
		w.failActionStage(ctx, calculateQuotasStage, err.Error())
		w.failStage(ctx, stageInfrastructureIndex)
		if perr := w.persist(ctx, infrastructureState, deploymentPlan); perr != nil {
			return nil, fmt.Errorf("persist failed run state: %w", perr)
		}
		return nil, err
	}
	w.completeActionStageWithOutputs(ctx, calculateQuotasStage, appendOutputs(
		deploymentbuilder.InfrastructurePlanOutputs(quotaResp.GetPlan()),
		deploymentbuilder.QuotaRequestRefOutputs(quotaResp.GetQuotaRequests())...,
	))
	acquireQuotasStage := w.startActionStage(ctx, stageInfrastructure, infrastructureStageID, 2, actionAcquireQuotas)
	acquireResp, err := workflowpb.AcquireQuotasActivity(ctx, &workflowpb.AcquireQuotasActivityRequest{
		TenantId:      w.req.GetTenantId(),
		RunId:         testRun.GetId(),
		Plan:          quotaResp.GetPlan(),
		QuotaRequests: quotaResp.GetQuotaRequests(),
	})
	if err != nil {
		w.failActionStage(ctx, acquireQuotasStage, err.Error())
		w.failStage(ctx, stageInfrastructureIndex)
		if perr := w.persist(ctx, infrastructureState, deploymentPlan); perr != nil {
			return nil, fmt.Errorf("persist failed run state: %w", perr)
		}
		return nil, err
	}
	w.completeActionStageWithOutputs(ctx, acquireQuotasStage, appendOutputs(
		deploymentbuilder.QuotaRequestRefOutputs(quotaResp.GetQuotaRequests()),
		deploymentbuilder.QuotaAllocationRefOutputs(acquireResp.GetQuotaAllocations())...,
	))
	quotaReserved = len(acquireResp.GetQuotaAllocations()) > 0
	quotaAllocations = acquireResp.GetQuotaAllocations()
	var networkCIDR string
	if quotaResp.GetPlan().GetProvider() == deploymentpb.Provider_PROVIDER_YANDEX {
		acquireNetworkStage := w.startActionStage(ctx, stageInfrastructure, infrastructureStageID, 3, actionAcquireNetwork)
		networkResp, err := workflowpb.AcquireNetworkActivity(ctx, &workflowpb.AcquireNetworkActivityRequest{
			TenantId: w.req.GetTenantId(),
			RunId:    testRun.GetId(),
			Plan:     quotaResp.GetPlan(),
		})
		if err != nil {
			w.failActionStage(ctx, acquireNetworkStage, err.Error())
			w.failStage(ctx, stageInfrastructureIndex)
			if perr := w.persist(ctx, infrastructureState, deploymentPlan); perr != nil {
				return nil, fmt.Errorf("persist failed run state: %w", perr)
			}
			return nil, err
		}
		networkCIDR = networkResp.GetNetworkCidr()
		w.completeActionStageWithOutputs(ctx, acquireNetworkStage, compactOutputs(
			deploymentbuilder.NetworkCIDROutput("network/reserved_cidr", "reserved network", networkCIDR),
		))
		if networkResp.GetNetworkCidr() != "" {
			infrastructurePlan = planWithReservedNetworkCIDR(quotaResp.GetPlan(), networkResp.GetNetworkCidr())
			w.state.Stages[stageInfrastructureIndex].Outputs = deploymentbuilder.InfrastructurePlanOutputs(infrastructurePlan)
			networkReserved = true
		}
	} else {
		infrastructurePlan = quotaResp.GetPlan()
	}
	// Arm teardown BEFORE provisioning: ProcessInfrastructure may create real
	// resources (terraform VMs / docker containers) and then fail (e.g. a
	// post-apply output decode error), so the defer must be able to tear those
	// down even when this very stage errors out. Idempotent for both providers.
	teardownPlan = infrastructurePlan
	processInfrastructureStage := w.startActionStage(ctx, stageInfrastructure, infrastructureStageID, 4, actionProcessInfrastructure)
	infrastructureResp, err := workflowpb.ProcessInfrastructureWorkflowChild(ctx, &workflowpb.ProcessInfrastructureWorkflowRequest{
		RunId:          testRun.GetId(),
		Plan:           infrastructurePlan,
		AgentBootstrap: w.req.GetAgentBootstrap(),
	})
	if err != nil {
		w.failActionStage(ctx, processInfrastructureStage, err.Error())
		w.failStage(ctx, stageInfrastructureIndex)
		if perr := w.persist(ctx, infrastructureState, deploymentPlan); perr != nil {
			return nil, fmt.Errorf("persist failed run state: %w", perr)
		}
		return nil, err
	}
	infrastructureState = infrastructureResp.GetState()
	w.completeActionStageWithOutputs(ctx, processInfrastructureStage, appendOutputs(
		deploymentbuilder.InfrastructurePlanOutputs(infrastructurePlan),
		deploymentbuilder.InfrastructureStateOutputs(infrastructureState)...,
	))
	infrastructureReady = true
	commitQuotasStage := w.startActionStage(ctx, stageInfrastructure, infrastructureStageID, 5, actionCommitQuotas)
	commitResp, err := workflowpb.CommitQuotasActivity(ctx, &workflowpb.CommitQuotasActivityRequest{
		TenantId: w.req.GetTenantId(),
		RunId:    testRun.GetId(),
	})
	if err != nil {
		w.failActionStage(ctx, commitQuotasStage, err.Error())
		w.failStage(ctx, stageInfrastructureIndex)
		if perr := w.persist(ctx, infrastructureState, deploymentPlan); perr != nil {
			return nil, fmt.Errorf("persist failed run state: %w", perr)
		}
		return nil, err
	}
	if len(commitResp.GetQuotaAllocations()) > 0 {
		quotaAllocations = commitResp.GetQuotaAllocations()
	}
	w.completeActionStageWithOutputs(ctx, commitQuotasStage, deploymentbuilder.QuotaAllocationRefOutputs(quotaAllocations))
	quotaCommitted = true
	if networkReserved {
		commitNetworkStage := w.startActionStage(ctx, stageInfrastructure, infrastructureStageID, 6, actionCommitNetwork)
		commitNetworkResp, err := workflowpb.CommitNetworkActivity(ctx, &workflowpb.CommitNetworkActivityRequest{
			TenantId: w.req.GetTenantId(),
			RunId:    testRun.GetId(),
		})
		if err != nil {
			w.failActionStage(ctx, commitNetworkStage, err.Error())
			w.failStage(ctx, stageInfrastructureIndex)
			if perr := w.persist(ctx, infrastructureState, deploymentPlan); perr != nil {
				return nil, fmt.Errorf("persist failed run state: %w", perr)
			}
			return nil, err
		}
		if commitNetworkResp.GetNetworkCidr() != "" {
			networkCIDR = commitNetworkResp.GetNetworkCidr()
		}
		w.completeActionStageWithOutputs(ctx, commitNetworkStage, compactOutputs(
			deploymentbuilder.NetworkCIDROutput("network/committed_cidr", "committed network", networkCIDR),
		))
		networkCommitted = true
	}
	attachQuotaAllocations(infrastructureState, quotaAllocations)
	w.state.Stages[stageInfrastructureIndex].Outputs = appendOutputs(
		deploymentbuilder.InfrastructurePlanOutputs(infrastructurePlan),
		deploymentbuilder.InfrastructureStateOutputs(infrastructureState)...,
	)
	w.completeStage(ctx, stageInfrastructureIndex)
	if err := w.persist(ctx, infrastructureState, deploymentPlan); err != nil {
		return nil, err
	}

	w.startStage(ctx, stageRenderPlanIndex)
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
		w.failStage(ctx, stageRenderPlanIndex)
		if perr := w.persist(ctx, infrastructureState, deploymentPlan); perr != nil {
			return nil, fmt.Errorf("persist failed run state: %w", perr)
		}
		return nil, err
	}
	deploymentPlan = renderResp.GetDeploymentPlan()
	w.state.Stages[stageRenderPlanIndex].Outputs = deploymentbuilder.DeploymentPlanOutputs(deploymentPlan)
	w.registerDeploymentPlanStages(testRun.GetId(), deploymentPlan)
	w.completeStage(ctx, stageRenderPlanIndex)
	if err := w.persist(ctx, infrastructureState, deploymentPlan); err != nil {
		return nil, err
	}

	w.startStage(ctx, stageExecutePlanIndex)
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
		w.failStage(ctx, stageExecutePlanIndex)
		if perr := w.persist(ctx, infrastructureState, deploymentPlan); perr != nil {
			return nil, fmt.Errorf("persist failed run state: %w", perr)
		}
		return nil, err
	}
	w.drainStageUpdates(ctx)
	deploymentPlan = executeResp.GetDeploymentPlan()
	w.registerDeploymentPlanStages(testRun.GetId(), deploymentPlan)
	w.completeStage(ctx, stageExecutePlanIndex)
	if err := w.persist(ctx, infrastructureState, deploymentPlan); err != nil {
		return nil, err
	}

	w.startStage(ctx, stageWorkloadIndex)
	if err := w.persist(ctx, infrastructureState, deploymentPlan); err != nil {
		return nil, err
	}
	if _, err := workflowpb.RunWorkloadWorkflowChild(ctx, &workflowpb.RunWorkloadWorkflowRequest{
		RunId:               testRun.GetId(),
		DeploymentPlan:      deploymentPlan,
		InfrastructureState: infrastructureState,
		AgentBootstrap:      w.req.GetAgentBootstrap(),
	}); err != nil {
		w.failStage(ctx, stageWorkloadIndex)
		if perr := w.persist(ctx, infrastructureState, deploymentPlan); perr != nil {
			return nil, fmt.Errorf("persist failed run state: %w", perr)
		}
		return nil, err
	}
	w.drainStageUpdates(ctx)
	w.completeStage(ctx, stageWorkloadIndex)

	if err := w.persist(ctx, infrastructureState, deploymentPlan); err != nil {
		return nil, err
	}
	return &workflowpb.TestWorkflowResponse{}, nil
}

// teardownInfrastructure destroys the provisioned infrastructure for the run's
// provider. Container/VM identities are deterministic from (runId, plan), so the
// re-rendered provider input is enough to tear everything down. It is invoked
// from the workflow's defer for every terminal outcome.
func (w *domainTestWorkflow) teardownInfrastructure(ctx workflow.Context, plan *deploymentpb.InfrastructurePlan) error {
	teardownStageID := deploymentbuilder.StageExecutionID(stageTeardown)
	switch plan.GetProvider() {
	case deploymentpb.Provider_PROVIDER_DOCKER:
		renderStage := w.startActionStage(ctx, stageTeardown, teardownStageID, 1, actionRenderDockerInput)
		input, err := workflowpb.RenderDockerInputWorkflowChild(ctx, &workflowpb.RenderDockerInputWorkflowRequest{
			RunId:          w.req.GetTestRun().GetId(),
			Plan:           plan,
			AgentBootstrap: w.req.GetAgentBootstrap(),
		})
		if err != nil {
			w.failActionStage(ctx, renderStage, err.Error())
			return err
		}
		w.completeActionStage(ctx, renderStage)
		downStage := w.startActionStage(ctx, stageTeardown, teardownStageID, 2, actionDockerDown)
		_, err = workflowpb.DockerDownActivity(ctx, input)
		if err != nil {
			w.failActionStage(ctx, downStage, err.Error())
			return err
		}
		w.completeActionStage(ctx, downStage)
		return err
	case deploymentpb.Provider_PROVIDER_YANDEX:
		renderStage := w.startActionStage(ctx, stageTeardown, teardownStageID, 1, actionRenderTerraformVariables)
		input, err := workflowpb.RenderTerraformVariablesWorkflowChild(ctx, &workflowpb.RenderTerraformVariablesWorkflowRequest{
			RunId:          w.req.GetTestRun().GetId(),
			Plan:           plan,
			Action:         deploymentpb.Terraform_ACTION_DESTROY,
			AgentBootstrap: w.req.GetAgentBootstrap(),
		})
		if err != nil {
			w.failActionStage(ctx, renderStage, err.Error())
			return err
		}
		w.completeActionStage(ctx, renderStage)
		destroyStage := w.startActionStage(ctx, stageTeardown, teardownStageID, 2, actionTerraformDestroy)
		_, err = workflowpb.TerraformDestroyActivity(ctx, input)
		if err != nil {
			w.failActionStage(ctx, destroyStage, err.Error())
			return err
		}
		w.completeActionStage(ctx, destroyStage)
		return err
	default:
		return nil
	}
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

func (w *domainTestWorkflow) releaseCommittedAllocations(ctx workflow.Context, releaseQuotas, releaseNetwork bool) error {
	teardownStageID := deploymentbuilder.StageExecutionID(stageTeardown)
	if releaseQuotas {
		releaseStage := w.startActionStage(ctx, stageTeardown, teardownStageID, 90, actionReleaseQuotas)
		if _, err := workflowpb.ReleaseQuotasActivity(ctx, &workflowpb.ReleaseQuotasActivityRequest{
			TenantId: w.req.GetTenantId(),
			RunId:    w.req.GetTestRun().GetId(),
		}); err != nil {
			w.failActionStage(ctx, releaseStage, err.Error())
			return err
		}
		w.completeActionStage(ctx, releaseStage)
	}
	if releaseNetwork {
		releaseStage := w.startActionStage(ctx, stageTeardown, teardownStageID, 91, actionReleaseNetwork)
		if _, err := workflowpb.ReleaseNetworkActivity(ctx, &workflowpb.ReleaseNetworkActivityRequest{
			TenantId: w.req.GetTenantId(),
			RunId:    w.req.GetTestRun().GetId(),
		}); err != nil {
			w.failActionStage(ctx, releaseStage, err.Error())
			return err
		}
		w.completeActionStage(ctx, releaseStage)
	}
	return nil
}

func appendOutputs(outputs []*monitor.PipelineOutput, extra ...*monitor.PipelineOutput) []*monitor.PipelineOutput {
	for _, output := range extra {
		if output == nil {
			continue
		}
		outputs = append(outputs, output)
	}
	return outputs
}

func compactOutputs(outputs ...*monitor.PipelineOutput) []*monitor.PipelineOutput {
	return appendOutputs(nil, outputs...)
}

func (w *domainTestWorkflow) GetRunState() (*workflowpb.RunState, error) {
	return proto.Clone(w.state).(*workflowpb.RunState), nil
}

func (w *domainTestWorkflow) listenStageUpdates(ctx workflow.Context) {
	if w.stageUpdates == nil {
		return
	}
	workflow.Go(ctx, func(ctx workflow.Context) {
		for {
			update, more := w.stageUpdates.Receive(ctx)
			if !more {
				return
			}
			stage := update.GetStage()
			w.applyStageUpdate(stage)
			w.persistRuntimeProjection(ctx, isProjectionForceStage(stage))
		}
	})
}

func (w *domainTestWorkflow) drainStageUpdates(ctx workflow.Context) {
	if w.stageUpdates == nil {
		return
	}
	for {
		update := w.stageUpdates.ReceiveAsync()
		if update == nil {
			return
		}
		stage := update.GetStage()
		w.applyStageUpdate(stage)
		w.persistRuntimeProjection(ctx, isProjectionForceStage(stage))
	}
}

func (w *domainTestWorkflow) registerDeploymentPlanStages(runID string, plan *deploymentpb.DeploymentPlan) {
	if plan == nil {
		return
	}
	deploymentbuilder.StampDeploymentPlanExecutionContext(runID, plan)
	sortComponentExecution(plan.GetComponents())
	parentStatus := w.state.GetStages()[stageExecutePlanIndex].GetStatus()
	for componentIndex, component := range plan.GetComponents() {
		if component == nil {
			continue
		}
		componentStatus := component.GetStatus()
		if componentStatus == common.Status_STATUS_UNSPECIFIED && parentStatus == common.Status_STATUS_COMPLETED {
			componentStatus = common.Status_STATUS_DEPLOYED
		}
		w.applyStageUpdate(componentStage(component, uint32(componentIndex+1), componentStatus, nil, nil, ""))
		sortAgentSteps(component)
		for stepIndex, step := range component.GetSteps() {
			if step == nil {
				continue
			}
			stepStatus := step.GetStatus()
			if stepStatus == common.Status_STATUS_UNSPECIFIED && componentStatus == common.Status_STATUS_DEPLOYED {
				stepStatus = common.Status_STATUS_DEPLOYED
			}
			w.applyStageUpdate(agentStepStage(component, step, uint32(stepIndex+1), stepStatus, nil, nil, ""))
		}
	}
}

func (w *domainTestWorkflow) applyStageUpdate(stage *workflowpb.Stage) {
	if stage == nil || stage.GetNodeExecutionId() == "" {
		return
	}
	incoming := proto.Clone(stage).(*workflowpb.Stage)
	if incoming.GetAttempt() == 0 {
		incoming.Attempt = 1
	}
	if incoming.GetStatus() == common.Status_STATUS_UNSPECIFIED {
		incoming.Status = common.Status_STATUS_PENDING
	}
	if incoming.GetStatusReason() == "" {
		incoming.StatusReason = stageStatusReason(incoming.GetStatus())
	}
	for _, existing := range w.state.GetStages() {
		if existing.GetNodeExecutionId() != incoming.GetNodeExecutionId() {
			continue
		}
		mergeStage(existing, incoming)
		sortRunStages(w.state.Stages)
		return
	}
	w.state.Stages = append(w.state.Stages, incoming)
	sortRunStages(w.state.Stages)
}

func (w *domainTestWorkflow) startActionStage(ctx workflow.Context, phase, parentNodeExecutionID string, order uint32, path ...string) *workflowpb.Stage {
	stage := temporalActionStage(phase, parentNodeExecutionID, order, common.Status_STATUS_RUNNING, timestamppb.New(workflow.Now(ctx)), nil, "", path...)
	w.applyStageUpdate(stage)
	w.persistRuntimeProjection(ctx, false)
	return stage
}

func (w *domainTestWorkflow) completeActionStage(ctx workflow.Context, stage *workflowpb.Stage) {
	w.finishActionStage(ctx, stage, common.Status_STATUS_COMPLETED, "", nil)
}

func (w *domainTestWorkflow) completeActionStageWithOutputs(ctx workflow.Context, stage *workflowpb.Stage, outputs []*monitor.PipelineOutput) {
	w.finishActionStage(ctx, stage, common.Status_STATUS_COMPLETED, "", outputs)
}

func (w *domainTestWorkflow) failActionStage(ctx workflow.Context, stage *workflowpb.Stage, errText string) {
	w.finishActionStage(ctx, stage, common.Status_STATUS_FAILED, errText, nil)
}

func (w *domainTestWorkflow) finishActionStage(ctx workflow.Context, stage *workflowpb.Stage, status common.Status, errText string, outputs []*monitor.PipelineOutput) {
	if stage == nil {
		return
	}
	update := proto.Clone(stage).(*workflowpb.Stage)
	update.Status = status
	update.FinishedAt = timestamppb.New(workflow.Now(ctx))
	update.StatusReason = stageStatusReason(status)
	update.ErrorMessage = errText
	if len(outputs) > 0 {
		update.Outputs = outputs
	}
	w.applyStageUpdate(update)
	w.persistRuntimeProjection(ctx, isProjectionForceStage(update))
}

func (w *domainTestWorkflow) startStage(ctx workflow.Context, index int) {
	startRootStage(ctx, w.state.Stages[index])
}

func (w *domainTestWorkflow) completeStage(ctx workflow.Context, index int) {
	completeRootStage(ctx, w.state.Stages[index])
}

func (w *domainTestWorkflow) failStage(ctx workflow.Context, index int) {
	w.state.Status = common.Status_STATUS_FAILED
	failRootStage(ctx, w.state.Stages[index])
}

func (w *domainTestWorkflow) fail(ctx workflow.Context) {
	w.state.Status = common.Status_STATUS_FAILED
	now := timestamppb.New(workflow.Now(ctx))
	for _, stage := range w.state.GetStages() {
		if stage.GetStatus() == common.Status_STATUS_PENDING || stage.GetStatus() == common.Status_STATUS_RUNNING {
			stage.Status = common.Status_STATUS_FAILED
			stage.FinishedAt = now
			stage.StatusReason = stageStatusReason(stage.GetStatus())
			return
		}
	}
}

func (w *domainTestWorkflow) cancel(ctx workflow.Context) {
	w.state.Status = common.Status_STATUS_CANCELLED
	now := timestamppb.New(workflow.Now(ctx))
	for _, stage := range w.state.GetStages() {
		if stage.GetStatus() == common.Status_STATUS_RUNNING || stage.GetStatus() == common.Status_STATUS_PENDING {
			cancelRootStage(ctx, stage)
			stage.FinishedAt = now
			return
		}
	}
}

func (w *domainTestWorkflow) persist(ctx workflow.Context, infrastructureState *deploymentpb.InfrastructureState, deploymentPlan *deploymentpb.DeploymentPlan) error {
	w.persistedInfrastructureState = infrastructureState
	w.persistedDeploymentPlan = deploymentPlan
	if err := persistRunState(ctx, w.req.GetTestRun().GetId(), w.state, infrastructureState, deploymentPlan); err != nil {
		return err
	}
	w.lastRuntimeProjectionPersistAt = workflow.Now(ctx)
	return nil
}

func (w *domainTestWorkflow) persistRuntimeProjection(ctx workflow.Context, force bool) {
	if w == nil || w.req == nil || w.req.GetTestRun().GetId() == "" {
		return
	}
	now := workflow.Now(ctx)
	if !force && !w.lastRuntimeProjectionPersistAt.IsZero() && now.Sub(w.lastRuntimeProjectionPersistAt) < runtimeProjectionPersistMinInterval {
		return
	}
	if err := w.persist(ctx, w.persistedInfrastructureState, w.persistedDeploymentPlan); err != nil {
		workflow.GetLogger(ctx).Warn("persist runtime projection", "run_id", w.req.GetTestRun().GetId(), "error", err)
	}
}

func isProjectionForceStage(stage *workflowpb.Stage) bool {
	if stage == nil {
		return false
	}
	switch stage.GetStatus() {
	case common.Status_STATUS_COMPLETED,
		common.Status_STATUS_FAILED,
		common.Status_STATUS_SKIPPED,
		common.Status_STATUS_CANCELLED:
		return true
	default:
		return false
	}
}

func mergeStage(existing, incoming *workflowpb.Stage) {
	existing.Name = firstNonEmpty(incoming.GetName(), existing.GetName())
	existing.Status = incoming.GetStatus()
	if incoming.GetStartedAt() != nil {
		existing.StartedAt = incoming.GetStartedAt()
	}
	if incoming.GetFinishedAt() != nil {
		existing.FinishedAt = incoming.GetFinishedAt()
	}
	if incoming.GetAttempt() != 0 {
		existing.Attempt = incoming.GetAttempt()
	}
	if incoming.GetOrder() != 0 {
		existing.Order = incoming.GetOrder()
	}
	existing.ParentNodeExecutionId = firstNonEmpty(incoming.GetParentNodeExecutionId(), existing.GetParentNodeExecutionId())
	existing.Phase = firstNonEmpty(incoming.GetPhase(), existing.GetPhase())
	existing.ComponentId = firstNonEmpty(incoming.GetComponentId(), existing.GetComponentId())
	existing.MachineId = firstNonEmpty(incoming.GetMachineId(), existing.GetMachineId())
	if incoming.GetWorker() != nil {
		existing.Worker = incoming.GetWorker()
	}
	existing.StatusReason = firstNonEmpty(incoming.GetStatusReason(), existing.GetStatusReason())
	existing.ErrorMessage = firstNonEmpty(incoming.GetErrorMessage(), existing.GetErrorMessage())
	if incoming.GetOperation() != nil {
		existing.Operation = incoming.GetOperation()
	}
	if len(incoming.GetOutputs()) > 0 {
		existing.Outputs = incoming.GetOutputs()
	}
}

func firstNonEmpty(left, right string) string {
	if left != "" {
		return left
	}
	return right
}
