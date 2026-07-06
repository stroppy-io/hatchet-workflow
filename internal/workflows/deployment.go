package workflows

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
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

	quotas := make([]*workflowpb.QuotaRequestRef, 0)
	for _, machine := range plan.GetMachines() {
		for _, req := range machine.GetQuotaRequests() {
			if req == nil {
				continue
			}
			quotas = append(quotas, &workflowpb.QuotaRequestRef{
				NodeId:  machine.GetNodeId(),
				Request: req,
			})
		}
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
	processStageID := actionStageExecutionID(stageInfrastructure, actionProcessInfrastructure)
	switch plan.GetProvider() {
	case deploymentpb.Provider_PROVIDER_DOCKER:
		renderStarted := timestamppb.New(workflow.Now(ctx))
		emitTemporalActionStage(ctx, stageInfrastructure, processStageID, 1, common.Status_STATUS_RUNNING, renderStarted, nil, "", actionProcessInfrastructure, actionRenderDockerInput)
		appendServerStageLog(ctx, w.req.GetRunId(), stageInfrastructure, processStageID, monitor.Stream_STREAM_STDOUT, "started "+actionRenderDockerInput, actionProcessInfrastructure, actionRenderDockerInput)
		dockerInput, err := workflowpb.RenderDockerInputWorkflowChild(ctx, &workflowpb.RenderDockerInputWorkflowRequest{
			RunId:          w.req.GetRunId(),
			Plan:           plan,
			AgentBootstrap: w.req.GetAgentBootstrap(),
		})
		if err != nil {
			emitTemporalActionStage(ctx, stageInfrastructure, processStageID, 1, common.Status_STATUS_FAILED, renderStarted, timestamppb.New(workflow.Now(ctx)), err.Error(), actionProcessInfrastructure, actionRenderDockerInput)
			appendServerStageLog(ctx, w.req.GetRunId(), stageInfrastructure, processStageID, monitor.Stream_STREAM_STDERR, "failed "+actionRenderDockerInput+": "+err.Error(), actionProcessInfrastructure, actionRenderDockerInput)
			return nil, err
		}
		emitTemporalActionStage(ctx, stageInfrastructure, processStageID, 1, common.Status_STATUS_COMPLETED, renderStarted, timestamppb.New(workflow.Now(ctx)), "", actionProcessInfrastructure, actionRenderDockerInput)
		appendServerStageLog(ctx, w.req.GetRunId(), stageInfrastructure, processStageID, monitor.Stream_STREAM_STDOUT, "completed "+actionRenderDockerInput, actionProcessInfrastructure, actionRenderDockerInput)
		pullStarted := timestamppb.New(workflow.Now(ctx))
		emitTemporalActionStage(ctx, stageInfrastructure, processStageID, 2, common.Status_STATUS_RUNNING, pullStarted, nil, "", actionProcessInfrastructure, actionDockerPull)
		appendServerStageLog(ctx, w.req.GetRunId(), stageInfrastructure, processStageID, monitor.Stream_STREAM_STDOUT, "started "+actionDockerPull, actionProcessInfrastructure, actionDockerPull)
		stampDockerLogContext(dockerInput, w.req.GetRunId(), stageInfrastructure, processStageID, actionProcessInfrastructure, actionDockerPull)
		if _, err := workflowpb.DockerPullActivity(ctx, dockerInput); err != nil {
			emitTemporalActionStage(ctx, stageInfrastructure, processStageID, 2, common.Status_STATUS_FAILED, pullStarted, timestamppb.New(workflow.Now(ctx)), err.Error(), actionProcessInfrastructure, actionDockerPull)
			appendServerStageLog(ctx, w.req.GetRunId(), stageInfrastructure, processStageID, monitor.Stream_STREAM_STDERR, "failed "+actionDockerPull+": "+err.Error(), actionProcessInfrastructure, actionDockerPull)
			return nil, err
		}
		emitTemporalActionStage(ctx, stageInfrastructure, processStageID, 2, common.Status_STATUS_COMPLETED, pullStarted, timestamppb.New(workflow.Now(ctx)), "", actionProcessInfrastructure, actionDockerPull)
		appendServerStageLog(ctx, w.req.GetRunId(), stageInfrastructure, processStageID, monitor.Stream_STREAM_STDOUT, "completed "+actionDockerPull, actionProcessInfrastructure, actionDockerPull)
		upStarted := timestamppb.New(workflow.Now(ctx))
		emitTemporalActionStage(ctx, stageInfrastructure, processStageID, 3, common.Status_STATUS_RUNNING, upStarted, nil, "", actionProcessInfrastructure, actionDockerUp)
		appendServerStageLog(ctx, w.req.GetRunId(), stageInfrastructure, processStageID, monitor.Stream_STREAM_STDOUT, "started "+actionDockerUp, actionProcessInfrastructure, actionDockerUp)
		stampDockerLogContext(dockerInput, w.req.GetRunId(), stageInfrastructure, processStageID, actionProcessInfrastructure, actionDockerUp)
		dockerOutput, err := workflowpb.DockerUpActivity(ctx, dockerInput)
		if err != nil {
			emitTemporalActionStage(ctx, stageInfrastructure, processStageID, 3, common.Status_STATUS_FAILED, upStarted, timestamppb.New(workflow.Now(ctx)), err.Error(), actionProcessInfrastructure, actionDockerUp)
			appendServerStageLog(ctx, w.req.GetRunId(), stageInfrastructure, processStageID, monitor.Stream_STREAM_STDERR, "failed "+actionDockerUp+": "+err.Error(), actionProcessInfrastructure, actionDockerUp)
			return nil, err
		}
		emitTemporalActionStage(ctx, stageInfrastructure, processStageID, 3, common.Status_STATUS_COMPLETED, upStarted, timestamppb.New(workflow.Now(ctx)), "", actionProcessInfrastructure, actionDockerUp)
		appendServerStageLog(ctx, w.req.GetRunId(), stageInfrastructure, processStageID, monitor.Stream_STREAM_STDOUT, "completed "+actionDockerUp, actionProcessInfrastructure, actionDockerUp)
		state, err := dockerInfrastructureState(w.req.GetRunId(), plan, dockerOutput)
		if err != nil {
			return nil, err
		}
		return &workflowpb.ProcessInfrastructureWorkflowResponse{State: state}, nil
	case deploymentpb.Provider_PROVIDER_YANDEX:
		renderStarted := timestamppb.New(workflow.Now(ctx))
		emitTemporalActionStage(ctx, stageInfrastructure, processStageID, 1, common.Status_STATUS_RUNNING, renderStarted, nil, "", actionProcessInfrastructure, actionRenderTerraformVariables)
		appendServerStageLog(ctx, w.req.GetRunId(), stageInfrastructure, processStageID, monitor.Stream_STREAM_STDOUT, "started "+actionRenderTerraformVariables, actionProcessInfrastructure, actionRenderTerraformVariables)
		terraformInput, err := workflowpb.RenderTerraformVariablesWorkflowChild(ctx, &workflowpb.RenderTerraformVariablesWorkflowRequest{
			RunId:          w.req.GetRunId(),
			Plan:           plan,
			Action:         deploymentpb.Terraform_ACTION_APPLY,
			AgentBootstrap: w.req.GetAgentBootstrap(),
		})
		if err != nil {
			emitTemporalActionStage(ctx, stageInfrastructure, processStageID, 1, common.Status_STATUS_FAILED, renderStarted, timestamppb.New(workflow.Now(ctx)), err.Error(), actionProcessInfrastructure, actionRenderTerraformVariables)
			appendServerStageLog(ctx, w.req.GetRunId(), stageInfrastructure, processStageID, monitor.Stream_STREAM_STDERR, "failed "+actionRenderTerraformVariables+": "+err.Error(), actionProcessInfrastructure, actionRenderTerraformVariables)
			return nil, err
		}
		emitTemporalActionStage(ctx, stageInfrastructure, processStageID, 1, common.Status_STATUS_COMPLETED, renderStarted, timestamppb.New(workflow.Now(ctx)), "", actionProcessInfrastructure, actionRenderTerraformVariables)
		appendServerStageLog(ctx, w.req.GetRunId(), stageInfrastructure, processStageID, monitor.Stream_STREAM_STDOUT, "completed "+actionRenderTerraformVariables, actionProcessInfrastructure, actionRenderTerraformVariables)
		applyStarted := timestamppb.New(workflow.Now(ctx))
		emitTemporalActionStage(ctx, stageInfrastructure, processStageID, 2, common.Status_STATUS_RUNNING, applyStarted, nil, "", actionProcessInfrastructure, actionTerraformApply)
		appendServerStageLog(ctx, w.req.GetRunId(), stageInfrastructure, processStageID, monitor.Stream_STREAM_STDOUT, "started "+actionTerraformApply, actionProcessInfrastructure, actionTerraformApply)
		stampTerraformLogContext(terraformInput, w.req.GetRunId(), stageInfrastructure, processStageID, actionProcessInfrastructure, actionTerraformApply)
		terraformOutput, err := workflowpb.TerraformApplyActivity(ctx, terraformInput)
		if err != nil {
			emitTemporalActionStage(ctx, stageInfrastructure, processStageID, 2, common.Status_STATUS_FAILED, applyStarted, timestamppb.New(workflow.Now(ctx)), err.Error(), actionProcessInfrastructure, actionTerraformApply)
			appendServerStageLog(ctx, w.req.GetRunId(), stageInfrastructure, processStageID, monitor.Stream_STREAM_STDERR, "failed "+actionTerraformApply+": "+err.Error(), actionProcessInfrastructure, actionTerraformApply)
			return nil, err
		}
		emitTemporalActionStage(ctx, stageInfrastructure, processStageID, 2, common.Status_STATUS_COMPLETED, applyStarted, timestamppb.New(workflow.Now(ctx)), "", actionProcessInfrastructure, actionTerraformApply)
		appendServerStageLog(ctx, w.req.GetRunId(), stageInfrastructure, processStageID, monitor.Stream_STREAM_STDOUT, "completed "+actionTerraformApply, actionProcessInfrastructure, actionTerraformApply)
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
		AgentTokens:     w.req.GetAgentBootstrap().GetAgentTokens(),
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

	runID := w.req.GetRunId()
	plan := proto.Clone(w.req.GetDeploymentPlan()).(*deploymentpb.DeploymentPlan)
	deploymentbuilder.StampDeploymentPlanExecutionContext(runID, plan)
	persistPlan := func() error {
		return persistDeploymentPlan(ctx, runID, plan)
	}
	sortComponentExecution(plan.GetComponents())
	components := plan.GetComponents()
	// Deploy components in waves grouped by global priority. Components within a
	// priority are independent across machines (peer-join/retry handles cluster
	// formation), so they deploy CONCURRENTLY; a lower-priority wave fully
	// completes before the next starts, preserving cross-tier ordering
	// (etcd -> master -> replica -> patroni -> proxy; ydb storage -> database).
	// Sequential per-component deploy made cluster installs O(N) slow and broke
	// quorum formation (a bootstrap node's healthcheck waited for peers that had
	// not been deployed yet because the executor was blocked on the bootstrap).
	for waveStart := 0; waveStart < len(components); {
		waveEnd := waveStart
		prio := components[waveStart].GetGlobalPriority()
		for waveEnd < len(components) && components[waveEnd].GetGlobalPriority() == prio {
			waveEnd++
		}
		wave := components[waveStart:waveEnd]

		finished := 0
		var waveErr error
		for offset := range wave {
			componentIndex := waveStart + offset
			component := wave[offset]
			workflow.Go(ctx, func(gctx workflow.Context) {
				defer func() { finished++ }()
				started := timestamppb.New(workflow.Now(gctx))
				component.Status = common.Status_STATUS_DEPLOYMENT
				emitStageUpdate(gctx, runID, componentStage(component, uint32(componentIndex+1), component.GetStatus(), started, nil, ""))
				if err := executeComponentDeployment(gctx, runID, plan, component, w.req.GetAgentBootstrap()); err != nil {
					finishedAt := timestamppb.New(workflow.Now(gctx))
					component.Status = common.Status_STATUS_FAILED
					emitStageUpdate(gctx, runID, componentStage(component, uint32(componentIndex+1), component.GetStatus(), started, finishedAt, err.Error()))
					if waveErr == nil {
						waveErr = err
					}
					return
				}
				finishedAt := timestamppb.New(workflow.Now(gctx))
				component.Status = common.Status_STATUS_DEPLOYED
				emitStageUpdate(gctx, runID, componentStage(component, uint32(componentIndex+1), component.GetStatus(), started, finishedAt, ""))
			})
		}
		if err := workflow.Await(ctx, func() bool { return finished == len(wave) }); err != nil {
			return nil, err
		}
		if waveErr != nil {
			// Persist the full plan only on failure (final statuses for the UI) and
			// at the very end — NOT per wave. The plan body (every machine's
			// rendered files/configs/units) is ~700KB and immutable except for
			// component statuses, which the UI already gets live from
			// emitStageUpdate; re-sending it after every wave put megabytes of
			// duplicate payload into this workflow's Temporal history and pushed a
			// single WorkflowTask past the gRPC message limit on large clusters.
			if perr := persistPlan(); perr != nil {
				return nil, fmt.Errorf("persist failed deployment plan: %w", perr)
			}
			return nil, waveErr
		}
		waveStart = waveEnd
	}

	if err := plan.Validate(); err != nil {
		return nil, err
	}
	if err := persistPlan(); err != nil {
		return nil, fmt.Errorf("persist deployed deployment plan: %w", err)
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

	// Managed YDB has no VM machine in the plan (the provider runs the
	// database), so synthesize a machine state for the managed-YDB topology
	// node from the terraform managed_ydb output. Its private endpoint is the
	// managed database's API endpoint host:port — this is what the deployment
	// builder resolves the workload→DB connection to, so stroppy connects to
	// the managed YDB instead of a self-hosted node.
	if managedNode := plan.GetLabels()["managed_node"]; managedNode != "" {
		managedState, err := managedYDBMachineState(managedNode, output.GetManagedYdb())
		if err != nil {
			return nil, err
		}
		state.Machines = append(state.Machines, managedState)
	}

	if err := state.Validate(); err != nil {
		return nil, err
	}
	return state, nil
}

// managedYDBMachineState builds the synthetic MachineState for the managed-YDB
// topology node. The node carries no VM; its private endpoint is the managed
// database's gRPC(S) API endpoint (host:port), so the topology connection from
// the stroppy runner resolves to the managed YDB service.
func managedYDBMachineState(nodeID string, managed *deploymentpb.Yandex_ManagedYdbOutput) (*deploymentpb.MachineState, error) {
	if managed == nil {
		return nil, fmt.Errorf("yandex output is missing managed_ydb for node %q", nodeID)
	}
	host, port := parseYDBEndpoint(managed.GetYdbApiEndpoint())
	if host == "" {
		return nil, fmt.Errorf("cannot parse managed YDB endpoint %q for node %q", managed.GetYdbApiEndpoint(), nodeID)
	}
	endpoint := &deploymentpb.Endpoint{
		Name:    "private",
		Address: host,
		Port:    &port,
		Labels: map[string]string{
			"scope":         "private",
			"managed":       "true",
			"database_path": managed.GetDatabasePath(),
			"api_endpoint":  managed.GetYdbApiEndpoint(),
		},
	}
	return &deploymentpb.MachineState{
		NodeId:             nodeID,
		ProviderResourceId: managed.GetId(),
		Status:             common.Status_STATUS_DEPLOYED,
		Endpoints:          []*deploymentpb.Endpoint{endpoint},
		Labels: map[string]string{
			"node_id":       nodeID,
			"managed":       "true",
			"database_path": managed.GetDatabasePath(),
		},
	}, nil
}

// parseYDBEndpoint splits a YC ydb_api_endpoint URL into host and port.
// Examples: grpcs://ydb.serverless.yandexcloud.net:2135/?database=/...
//
//	grpcs://ydb.api.cloud.yandex.net:2135/...
func parseYDBEndpoint(raw string) (string, uint32) {
	s := strings.TrimPrefix(raw, "grpcs://")
	s = strings.TrimPrefix(s, "grpc://")
	if i := strings.IndexAny(s, "/?"); i >= 0 {
		s = s[:i]
	}
	host, portStr, ok := strings.Cut(s, ":")
	if !ok {
		if host == "" {
			return "", 0
		}
		return host, 2135
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return host, 2135
	}
	return host, uint32(port)
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

func executeComponentDeployment(ctx workflow.Context, runID string, plan *deploymentpb.DeploymentPlan, component *deploymentpb.ComponentDeployment, bootstrap *workflowpb.AgentBootstrap) error {
	// Provider-managed components (e.g. managed YDB) have no VM/agent — the
	// database is provisioned by terraform. Skip them entirely: reaching for an
	// agent task queue that does not exist would fail with "agent task queue for
	// node ... is missing". (They keep a no-op marker step only to satisfy
	// ComponentDeployment validation.)
	if component.GetLabels()["managed"] == "true" || len(component.GetSteps()) == 0 {
		return nil
	}
	sortAgentSteps(component)
	taskQueue, err := agentTaskQueue(bootstrap, component.GetNodeId())
	if err != nil {
		return err
	}
	activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           taskQueue,
		StartToCloseTimeout: 60 * time.Minute,
		HeartbeatTimeout:    time.Minute,
	})
	if err := executeActivityNoResult(activityCtx, workflowpb.EnsureAgentOnlineActivityActivityName); err != nil {
		return fmt.Errorf("agent %s is not online: %w", component.GetNodeId(), err)
	}
	for stepIndex, step := range component.GetSteps() {
		deploymentbuilder.StampAgentStepExecutionContext(runID, component, step)
		stepStarted := timestamppb.New(workflow.Now(ctx))
		step.Status = common.Status_STATUS_DEPLOYMENT
		emitStageUpdate(ctx, runID, agentStepStage(component, step, uint32(stepIndex+1), step.GetStatus(), stepStarted, nil, ""))
		if err := appendDeploymentStepLog(ctx, runID, component, step, monitor.Stream_STREAM_STDOUT, "started "+agentStepDescription(step)); err != nil {
			return err
		}
		if err := executeAgentStep(activityCtx, step); err != nil {
			stepFinished := timestamppb.New(workflow.Now(ctx))
			step.Status = common.Status_STATUS_FAILED
			emitStageUpdate(ctx, runID, agentStepStage(component, step, uint32(stepIndex+1), step.GetStatus(), stepStarted, stepFinished, err.Error()))
			if lerr := appendDeploymentStepLog(ctx, runID, component, step, monitor.Stream_STREAM_STDERR, "failed "+agentStepDescription(step)+": "+err.Error()); lerr != nil {
				return lerr
			}
			return fmt.Errorf("component %s step %s: %w", component.GetComponentId(), step.GetId(), err)
		}
		stepFinished := timestamppb.New(workflow.Now(ctx))
		step.Status = common.Status_STATUS_DEPLOYED
		emitStageUpdate(ctx, runID, agentStepStage(component, step, uint32(stepIndex+1), step.GetStatus(), stepStarted, stepFinished, ""))
		if err := appendDeploymentStepLog(ctx, runID, component, step, monitor.Stream_STREAM_STDOUT, "completed "+agentStepDescription(step)); err != nil {
			return err
		}
	}
	return nil
}

func appendDeploymentStepLog(ctx workflow.Context, runID string, component *deploymentpb.ComponentDeployment, step *deploymentpb.AgentStep, stream monitor.Stream, line string) error {
	if runID == "" || component == nil || step == nil || line == "" {
		return nil
	}
	return appendRunLogs(ctx, &monitor.LogLine{
		ObservedAt:            timestamppb.New(workflow.Now(ctx)),
		RunId:                 runID,
		NodeExecutionId:       step.GetLabels()[deploymentbuilder.LabelNodeExecutionID],
		ParentNodeExecutionId: step.GetLabels()[deploymentbuilder.LabelParentNodeExecutionID],
		Phase:                 step.GetLabels()[deploymentbuilder.LabelPhase],
		StageName:             step.GetLabels()[deploymentbuilder.LabelStageName],
		ComponentId:           component.GetComponentId(),
		MachineId:             component.GetNodeId(),
		StepId:                step.GetLabels()[deploymentbuilder.LabelStepID],
		Action:                step.GetLabels()[deploymentbuilder.LabelAction],
		Mentions:              splitLabelList(step.GetLabels()[deploymentbuilder.LabelOperationMentions]),
		Source:                monitor.Source_SOURCE_COMMAND,
		Unit:                  deploymentbuilder.AgentStepActionKind(step),
		Stream:                stream,
		Line:                  line,
	})
}

func splitLabelList(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func agentStepDescription(step *deploymentpb.AgentStep) string {
	action := deploymentbuilder.AgentStepActionKind(step)
	switch typed := step.GetAction().(type) {
	case *deploymentpb.AgentStep_CreateDir:
		return action + " " + typed.CreateDir.GetInfo().GetPath()
	case *deploymentpb.AgentStep_WriteFile:
		return action + " " + typed.WriteFile.GetInfo().GetPath()
	case *deploymentpb.AgentStep_FetchFile:
		return action + " " + typed.FetchFile.GetInfo().GetPath()
	case *deploymentpb.AgentStep_CallCmd:
		if argv := typed.CallCmd.GetSpec().GetArgv(); argv != nil {
			return action + " " + strings.Join(argv.GetArgs(), " ")
		}
		if script := typed.CallCmd.GetSpec().GetScript(); script != nil {
			text := strings.TrimSpace(script.GetText())
			if i := strings.IndexByte(text, '\n'); i >= 0 {
				text = text[:i]
			}
			return action + " " + text
		}
		return action
	default:
		return step.GetId()
	}
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
