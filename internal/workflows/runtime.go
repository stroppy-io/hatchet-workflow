package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
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
