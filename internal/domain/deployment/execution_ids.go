package deployment

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
)

const (
	LabelNodeExecutionID = "stroppy.io/node-execution-id"
	LabelComponentID     = "stroppy.io/component-id"
	LabelNodeID          = "stroppy.io/node-id"
	LabelStepID          = "stroppy.io/step-id"
	LabelAction          = "stroppy.io/action"

	EnvRunID           = "STROPPY_RUN_ID"
	EnvNodeExecutionID = "STROPPY_NODE_EXECUTION_ID"
	EnvComponentID     = "STROPPY_COMPONENT_ID"
	EnvNodeID          = "STROPPY_NODE_ID"
	EnvStepID          = "STROPPY_STEP_ID"
	EnvAction          = "STROPPY_ACTION"
)

const maxNodeExecutionIDLen = 128

func StageExecutionID(stage string) string {
	return boundedExecutionID("stage/"+stage, "stage", stage)
}

func ComponentExecutionID(componentID string) string {
	return boundedExecutionID("component/"+componentID, "component", componentID)
}

func StepExecutionID(componentID, stepID string) string {
	return boundedExecutionID("component/"+componentID+"/step/"+stepID, "step", componentID, stepID)
}

func AgentStepActionKind(step *deploymentpb.AgentStep) string {
	if step == nil {
		return ""
	}
	switch step.GetAction().(type) {
	case *deploymentpb.AgentStep_CreateDir:
		return "create_dir"
	case *deploymentpb.AgentStep_WriteFile:
		return "write_file"
	case *deploymentpb.AgentStep_FetchFile:
		return "fetch_file"
	case *deploymentpb.AgentStep_CallCmd:
		return "call_cmd"
	default:
		return ""
	}
}

func StampDeploymentPlanExecutionContext(runID string, plan *deploymentpb.DeploymentPlan) {
	if plan == nil {
		return
	}
	for _, component := range plan.GetComponents() {
		StampComponentExecutionContext(runID, component)
	}
}

func StampComponentExecutionContext(runID string, component *deploymentpb.ComponentDeployment) {
	if component == nil {
		return
	}
	componentID := component.GetComponentId()
	nodeExecutionID := ComponentExecutionID(componentID)
	if component.Labels == nil {
		component.Labels = map[string]string{}
	}
	component.Labels[LabelRunID] = runID
	component.Labels[LabelNodeExecutionID] = nodeExecutionID
	component.Labels[LabelComponentID] = componentID
	component.Labels[LabelNodeID] = component.GetNodeId()
	for _, step := range component.GetSteps() {
		StampAgentStepExecutionContext(runID, component, step)
	}
}

func StampAgentStepExecutionContext(runID string, component *deploymentpb.ComponentDeployment, step *deploymentpb.AgentStep) {
	if component == nil || step == nil {
		return
	}
	componentID := component.GetComponentId()
	nodeID := component.GetNodeId()
	stepID := step.GetId()
	nodeExecutionID := StepExecutionID(componentID, stepID)
	action := AgentStepActionKind(step)
	if step.Labels == nil {
		step.Labels = map[string]string{}
	}
	step.Labels[LabelRunID] = runID
	step.Labels[LabelNodeExecutionID] = nodeExecutionID
	step.Labels[LabelComponentID] = componentID
	step.Labels[LabelNodeID] = nodeID
	step.Labels[LabelStepID] = stepID
	step.Labels[LabelAction] = action
	if cmd := step.GetCallCmd(); cmd != nil {
		stampCommandEnv(cmd, map[string]string{
			EnvRunID:           runID,
			EnvNodeExecutionID: nodeExecutionID,
			EnvComponentID:     componentID,
			EnvNodeID:          nodeID,
			EnvStepID:          stepID,
			EnvAction:          action,
		})
	}
}

func stampCommandEnv(cmd *common.Cmd, env map[string]string) {
	if cmd == nil || cmd.Spec == nil {
		return
	}
	if cmd.Spec.Env == nil {
		cmd.Spec.Env = map[string]string{}
	}
	for key, value := range env {
		if value != "" {
			cmd.Spec.Env[key] = value
		}
	}
}

func boundedExecutionID(raw, prefix string, parts ...string) string {
	if len(raw) <= maxNodeExecutionIDLen {
		return raw
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	short := hex.EncodeToString(sum[:])[:24]
	out := prefix + "/" + short
	if len(out) <= maxNodeExecutionIDLen {
		return out
	}
	return out[:maxNodeExecutionIDLen]
}
