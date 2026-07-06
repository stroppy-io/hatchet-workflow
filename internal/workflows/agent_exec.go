package workflows

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

func agentTaskQueue(bootstrap *workflowpb.AgentBootstrap, nodeID string) (string, error) {
	if bootstrap == nil {
		return "", fmt.Errorf("agent task queue for node %q is missing: agent bootstrap is not configured", nodeID)
	}
	queue := bootstrap.GetAgentTaskQueues()[nodeID]
	if queue == "" {
		return "", fmt.Errorf("agent task queue for node %q is missing", nodeID)
	}
	return queue, nil
}

func executeAgentStep(ctx workflow.Context, step *deploymentpb.AgentStep) error {
	switch action := step.GetAction().(type) {
	case *deploymentpb.AgentStep_CreateDir:
		return executeActivityNoResult(ctx, workflowpb.CreateDirActivityActivityName, action.CreateDir)
	case *deploymentpb.AgentStep_WriteFile:
		return executeActivityNoResult(ctx, workflowpb.WriteFileActivityActivityName, action.WriteFile)
	case *deploymentpb.AgentStep_FetchFile:
		return executeActivityNoResult(ctx, workflowpb.FetchFileActivityActivityName, action.FetchFile)
	case *deploymentpb.AgentStep_CallCmd:
		var result common.Cmd_Result
		if err := workflow.ExecuteActivity(ctx, workflowpb.CallCmdActivityActivityName, action.CallCmd).Get(ctx, &result); err != nil {
			return err
		}
		if !expectedExitCode(action.CallCmd, result.GetExitCode()) {
			return fmt.Errorf("command exited %d: %s", result.GetExitCode(), string(result.GetStderr()))
		}
		action.CallCmd.Result = &result
		return nil
	default:
		return fmt.Errorf("unsupported agent step action %T", action)
	}
}

func executeActivityNoResult(ctx workflow.Context, name string, args ...any) error {
	return workflow.ExecuteActivity(ctx, name, args...).Get(ctx, nil)
}

func expectedExitCode(cmd *common.Cmd, exitCode int32) bool {
	expected := cmd.GetSpec().GetExpectedExitCodes()
	if len(expected) == 0 {
		expected = []int32{0}
	}
	for _, code := range expected {
		if code == exitCode {
			return true
		}
	}
	return false
}
