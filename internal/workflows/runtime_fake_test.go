package workflows

import (
	"context"
	"sync"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"google.golang.org/protobuf/proto"

	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

// fakeRuntimeActivities is a minimal in-memory RuntimeActivities double used
// by runrecipe_test.go to exercise RunRecipeWorkflow's persistence calls
// without a real postgres-backed execution.RunPersistenceActivities.
//
// Extracted (trimmed of the old stage-machine-only query helpers) from the
// deleted workflows_test.go, which used the same fake for both the old
// TestWorkflow/DeploymentWorkflow tests and RunRecipeWorkflow.
type fakeRuntimeActivities struct {
	mu               sync.Mutex
	runStates        []*workflowpb.RunState
	deploymentPlans  []*deploymentpb.DeploymentPlan
	logLines         []*monitor.LogLine
	summaries        []*models.TestRunRecord_Summary
	recipeTopologies []*models.RecipeTopologySnapshot
}

func registerFakeRuntimeActivities(env *testsuite.TestWorkflowEnvironment, fake *fakeRuntimeActivities) {
	env.RegisterActivityWithOptions(fake.PersistRunState, activity.RegisterOptions{Name: PersistRunStateActivityName})
	env.RegisterActivityWithOptions(fake.PersistDeploymentPlan, activity.RegisterOptions{Name: PersistDeploymentPlanActivityName})
	env.RegisterActivityWithOptions(fake.AppendRunLogs, activity.RegisterOptions{Name: AppendRunLogsActivityName})
	env.RegisterActivityWithOptions(fake.PersistRunSummary, activity.RegisterOptions{Name: PersistRunSummaryActivityName})
	env.RegisterActivityWithOptions(fake.PersistRecipeTopology, activity.RegisterOptions{Name: PersistRecipeTopologyActivityName})
}

func (f *fakeRuntimeActivities) PersistRunState(
	_ context.Context,
	_ string,
	state *workflowpb.RunState,
	_ *deploymentpb.InfrastructureState,
	_ *deploymentpb.DeploymentPlan,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if state == nil {
		f.runStates = append(f.runStates, nil)
		return nil
	}
	f.runStates = append(f.runStates, proto.Clone(state).(*workflowpb.RunState))
	return nil
}

func (f *fakeRuntimeActivities) PersistDeploymentPlan(
	_ context.Context,
	_ string,
	plan *deploymentpb.DeploymentPlan,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if plan == nil {
		f.deploymentPlans = append(f.deploymentPlans, nil)
		return nil
	}
	f.deploymentPlans = append(f.deploymentPlans, proto.Clone(plan).(*deploymentpb.DeploymentPlan))
	return nil
}

func (f *fakeRuntimeActivities) PersistRunSummary(
	_ context.Context,
	_ string,
	summary *models.TestRunRecord_Summary,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if summary == nil {
		f.summaries = append(f.summaries, nil)
		return nil
	}
	f.summaries = append(f.summaries, proto.Clone(summary).(*models.TestRunRecord_Summary))
	return nil
}

func (f *fakeRuntimeActivities) PersistRecipeTopology(
	_ context.Context,
	_ string,
	snapshot *models.RecipeTopologySnapshot,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if snapshot == nil {
		f.recipeTopologies = append(f.recipeTopologies, nil)
		return nil
	}
	f.recipeTopologies = append(f.recipeTopologies, proto.Clone(snapshot).(*models.RecipeTopologySnapshot))
	return nil
}

func (f *fakeRuntimeActivities) AppendRunLogs(_ context.Context, lines []*monitor.LogLine) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, line := range lines {
		if line == nil {
			f.logLines = append(f.logLines, nil)
			continue
		}
		f.logLines = append(f.logLines, proto.Clone(line).(*monitor.LogLine))
	}
	return nil
}
