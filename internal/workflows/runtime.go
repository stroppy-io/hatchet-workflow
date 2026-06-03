package workflows

import (
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func runtimeActivityContext(ctx workflow.Context) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumInterval: 10 * time.Second,
			MaximumAttempts: 8,
		},
	})
}

func persistRunState(
	ctx workflow.Context,
	runID string,
	state *workflowpb.RunState,
	infrastructureState *deploymentpb.InfrastructureState,
	deploymentPlan *deploymentpb.DeploymentPlan,
) error {
	actx := runtimeActivityContext(ctx)
	return workflow.ExecuteActivity(
		actx,
		PersistRunStateActivityName,
		runID,
		state,
		infrastructureState,
		deploymentPlan,
	).Get(actx, nil)
}

func persistDeploymentPlan(ctx workflow.Context, runID string, deploymentPlan *deploymentpb.DeploymentPlan) error {
	if runID == "" || deploymentPlan == nil {
		return nil
	}
	actx := runtimeActivityContext(ctx)
	return workflow.ExecuteActivity(
		actx,
		PersistDeploymentPlanActivityName,
		runID,
		deploymentPlan,
	).Get(actx, nil)
}

func appendRunLogs(ctx workflow.Context, lines ...*monitor.LogLine) error {
	if len(lines) == 0 {
		return nil
	}
	actx := runtimeActivityContext(ctx)
	return workflow.ExecuteActivity(actx, AppendRunLogsActivityName, lines).Get(actx, nil)
}

func persistSuiteRun(ctx workflow.Context, suiteRunID string, status common.Status) error {
	if suiteRunID == "" {
		return nil
	}
	actx := runtimeActivityContext(ctx)
	return workflow.ExecuteActivity(actx, PersistSuiteRunActivityName, suiteRunID, status).Get(actx, nil)
}
