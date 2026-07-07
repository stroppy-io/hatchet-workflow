package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
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

// persistRunSummary calls PersistRunSummaryActivityName to merge a
// recipe-derived TestRunRecord.Summary (see runrecipe_summary.go's
// deriveRunSummary) onto the run's stored record. Distinct from
// persistRunState above: that one carries the RunState-derived timing/
// progress facets applyRunSummary computes on every stage transition, while
// this one carries the CompiledPlan-derived static facets (Provider/
// NodeCount/TopologyLabel/DbKind/WorkloadName/StroppyVersion), computed
// once right after compile succeeds — see RunRecipeWorkflow.run's call
// right after the compile stage completes.
func persistRunSummary(ctx workflow.Context, runID string, summary *models.TestRunRecord_Summary) error {
	actx := runtimeActivityContext(ctx)
	return workflow.ExecuteActivity(actx, PersistRunSummaryActivityName, runID, summary).Get(actx, nil)
}

// persistRecipeTopology calls PersistRecipeTopologyActivityName to store the
// recipe run's topology snapshot (see runrecipe_topology.go's
// deriveRecipeTopology) onto the run record's recipe_topology field — called
// once, right after ProvisionActivity succeeds (see RunRecipeWorkflow.run's
// infra-stage block), mirroring persistRunSummary's identical
// call-right-after-the-activity-that-produced-the-data shape.
func persistRecipeTopology(ctx workflow.Context, runID string, snapshot *models.RecipeTopologySnapshot) error {
	actx := runtimeActivityContext(ctx)
	return workflow.ExecuteActivity(actx, PersistRecipeTopologyActivityName, runID, snapshot).Get(actx, nil)
}
