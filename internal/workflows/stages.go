package workflows

import (
	"sort"
	"strings"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domainpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const stageUpdateStatusPrefix = "workflow_stage_"

func rootStage(name string, order uint32) *workflowpb.Stage {
	return &workflowpb.Stage{
		NodeExecutionId: deploymentbuilder.StageExecutionID(name),
		Name:            name,
		Status:          common.Status_STATUS_PENDING,
		Attempt:         1,
		Order:           order,
		Phase:           name,
		Worker:          masterWorker(),
		StatusReason:    stageStatusReason(common.Status_STATUS_PENDING),
	}
}

func componentStage(component *deploymentpb.ComponentDeployment, order uint32, status common.Status, started, finished *timestamppb.Timestamp, errText string) *workflowpb.Stage {
	if component == nil {
		return nil
	}
	componentID := component.GetComponentId()
	nodeID := component.GetNodeId()
	nodeExecutionID := labelOr(component.GetLabels(), deploymentbuilder.LabelNodeExecutionID, deploymentbuilder.ComponentExecutionID(componentID))
	return &workflowpb.Stage{
		NodeExecutionId:       nodeExecutionID,
		Name:                  componentID,
		Status:                deploymentRuntimeStatus(status),
		StartedAt:             started,
		FinishedAt:            finished,
		Attempt:               1,
		Order:                 order,
		ParentNodeExecutionId: deploymentbuilder.StageExecutionID(stageExecutePlan),
		Phase:                 stageExecutePlan,
		ComponentId:           componentID,
		MachineId:             nodeID,
		Worker:                agentWorker(nodeID),
		StatusReason:          stageStatusReason(deploymentRuntimeStatus(status)),
		ErrorMessage:          errText,
	}
}

func agentStepStage(component *deploymentpb.ComponentDeployment, step *deploymentpb.AgentStep, order uint32, status common.Status, started, finished *timestamppb.Timestamp, errText string) *workflowpb.Stage {
	if component == nil || step == nil {
		return nil
	}
	componentID := component.GetComponentId()
	nodeID := component.GetNodeId()
	nodeExecutionID := labelOr(step.GetLabels(), deploymentbuilder.LabelNodeExecutionID, deploymentbuilder.StepExecutionID(componentID, step.GetId()))
	return &workflowpb.Stage{
		NodeExecutionId:       nodeExecutionID,
		Name:                  deploymentbuilder.AgentStepStageName(step),
		Status:                deploymentRuntimeStatus(status),
		StartedAt:             started,
		FinishedAt:            finished,
		Attempt:               1,
		Order:                 order,
		ParentNodeExecutionId: labelOr(component.GetLabels(), deploymentbuilder.LabelNodeExecutionID, deploymentbuilder.ComponentExecutionID(componentID)),
		Phase:                 stageExecutePlan,
		ComponentId:           componentID,
		MachineId:             nodeID,
		Worker:                agentWorker(nodeID),
		StatusReason:          stageStatusReason(deploymentRuntimeStatus(status)),
		ErrorMessage:          errText,
		Operation:             deploymentbuilder.AgentStepOperation(step),
		Outputs:               deploymentbuilder.AgentStepResultOutputs(component, step),
	}
}

func temporalActionStage(phase, parentNodeExecutionID string, order uint32, status common.Status, started, finished *timestamppb.Timestamp, errText string, path ...string) *workflowpb.Stage {
	name := ""
	if len(path) > 0 {
		name = path[len(path)-1]
	}
	return &workflowpb.Stage{
		NodeExecutionId:       actionStageExecutionID(append([]string{phase}, path...)...),
		Name:                  name,
		Status:                deploymentRuntimeStatus(status),
		StartedAt:             started,
		FinishedAt:            finished,
		Attempt:               1,
		Order:                 order,
		ParentNodeExecutionId: parentNodeExecutionID,
		Phase:                 phase,
		Worker:                masterWorker(),
		StatusReason:          stageStatusReason(deploymentRuntimeStatus(status)),
		ErrorMessage:          errText,
	}
}

func emitTemporalActionStage(ctx workflow.Context, phase, parentNodeExecutionID string, order uint32, status common.Status, started, finished *timestamppb.Timestamp, errText string, path ...string) {
	emitStageUpdate(ctx, "", temporalActionStage(phase, parentNodeExecutionID, order, status, started, finished, errText, path...))
}

func emitStageUpdate(ctx workflow.Context, runID string, stage *workflowpb.Stage) {
	if stage == nil {
		return
	}
	targetWorkflowID := stageUpdateTargetWorkflowID(ctx)
	if targetWorkflowID == "" {
		return
	}
	_ = workflowpb.UpdateStageExternal(ctx, targetWorkflowID, "", &workflowpb.StageUpdate{Stage: stage})
}

func stageUpdateTargetWorkflowID(ctx workflow.Context) string {
	info := workflow.GetInfo(ctx)
	if info.ParentWorkflowExecution != nil && info.ParentWorkflowExecution.ID != "" {
		return info.ParentWorkflowExecution.ID
	}
	if info.RootWorkflowExecution != nil && info.RootWorkflowExecution.ID != "" {
		return info.RootWorkflowExecution.ID
	}
	return ""
}

func deploymentRuntimeStatus(status common.Status) common.Status {
	switch status {
	case common.Status_STATUS_UNSPECIFIED:
		return common.Status_STATUS_PENDING
	case common.Status_STATUS_DEPLOYMENT,
		common.Status_STATUS_ALLOCATED,
		common.Status_STATUS_RUNNING:
		return common.Status_STATUS_RUNNING
	case common.Status_STATUS_DEPLOYED:
		return common.Status_STATUS_COMPLETED
	default:
		return status
	}
}

func startRootStage(ctx workflow.Context, stage *workflowpb.Stage) {
	stage.Status = common.Status_STATUS_RUNNING
	if stage.StartedAt == nil {
		stage.StartedAt = timestamppb.New(workflow.Now(ctx))
	}
	stage.StatusReason = stageStatusReason(stage.GetStatus())
}

func completeRootStage(ctx workflow.Context, stage *workflowpb.Stage) {
	stage.Status = common.Status_STATUS_COMPLETED
	stage.FinishedAt = timestamppb.New(workflow.Now(ctx))
	stage.StatusReason = stageStatusReason(stage.GetStatus())
}

func failRootStage(ctx workflow.Context, stage *workflowpb.Stage) {
	stage.Status = common.Status_STATUS_FAILED
	stage.FinishedAt = timestamppb.New(workflow.Now(ctx))
	stage.StatusReason = stageStatusReason(stage.GetStatus())
}

func cancelRootStage(ctx workflow.Context, stage *workflowpb.Stage) {
	stage.Status = common.Status_STATUS_CANCELLED
	stage.FinishedAt = timestamppb.New(workflow.Now(ctx))
	stage.StatusReason = stageStatusReason(stage.GetStatus())
}

func sortRunStages(stages []*workflowpb.Stage) {
	sort.SliceStable(stages, func(i, j int) bool {
		left := stages[i]
		right := stages[j]
		if left.GetParentNodeExecutionId() != right.GetParentNodeExecutionId() {
			return left.GetParentNodeExecutionId() < right.GetParentNodeExecutionId()
		}
		if left.GetOrder() != right.GetOrder() {
			return left.GetOrder() < right.GetOrder()
		}
		return left.GetNodeExecutionId() < right.GetNodeExecutionId()
	})
}

func sortAgentSteps(component *deploymentpb.ComponentDeployment) {
	if component == nil {
		return
	}
	sort.SliceStable(component.Steps, func(i, j int) bool {
		if component.Steps[i].GetOrder() != component.Steps[j].GetOrder() {
			return component.Steps[i].GetOrder() < component.Steps[j].GetOrder()
		}
		return component.Steps[i].GetId() < component.Steps[j].GetId()
	})
}

func stageStatusReason(status common.Status) string {
	if status == common.Status_STATUS_UNSPECIFIED {
		status = common.Status_STATUS_PENDING
	}
	return stageUpdateStatusPrefix + strings.ToLower(status.String())
}

func masterWorker() *domainpb.Worker {
	return &domainpb.Worker{Id: "master", Kind: domainpb.Worker_KIND_MASTER}
}

func agentWorker(nodeID string) *domainpb.Worker {
	return &domainpb.Worker{Id: "agent/" + nodeID, Kind: domainpb.Worker_KIND_AGENT}
}

func actionStageExecutionID(parts ...string) string {
	return deploymentbuilder.StageExecutionID(strings.Join(parts, "/"))
}

func labelOr(labels map[string]string, key, fallback string) string {
	if value := labels[key]; value != "" {
		return value
	}
	return fallback
}
