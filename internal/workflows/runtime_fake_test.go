package workflows

import (
	"context"
	"sync"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"google.golang.org/protobuf/proto"

	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
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
//
// seq is a monotonic call counter incremented by every Persist* method
// below (any type) — persistedCompiledPlanAt/persistedSummaryAt capture its
// value at the moment each of those two specific activities last ran, so
// tests can assert relative ordering (e.g. the compiled plan persists no
// later than the summary — see runrecipe_test.go's
// TestRunRecipeWorkflowPersistsCompiledPlanAfterCompile) without depending
// on wall-clock timestamps.
type fakeRuntimeActivities struct {
	mu               sync.Mutex
	seq              int
	runStates        []*workflowpb.RunState
	deploymentPlans  []*deploymentpb.DeploymentPlan
	logLines         []*monitor.LogLine
	summaries        []*models.Run_Summary
	recipeTopologies []*models.RunTopology
	compiledPlans    []*dslpb.CompiledPlan

	persistedCompiledPlan   *dslpb.CompiledPlan
	persistedCompiledPlanAt int
	persistedSummary        *models.Run_Summary
	persistedSummaryAt      int
}

func registerFakeRuntimeActivities(env *testsuite.TestWorkflowEnvironment, fake *fakeRuntimeActivities) {
	env.RegisterActivityWithOptions(fake.PersistRunState, activity.RegisterOptions{Name: PersistRunStateActivityName})
	env.RegisterActivityWithOptions(fake.PersistDeploymentPlan, activity.RegisterOptions{Name: PersistDeploymentPlanActivityName})
	env.RegisterActivityWithOptions(fake.AppendRunLogs, activity.RegisterOptions{Name: AppendRunLogsActivityName})
	env.RegisterActivityWithOptions(fake.PersistRunSummary, activity.RegisterOptions{Name: PersistRunSummaryActivityName})
	env.RegisterActivityWithOptions(fake.PersistRecipeTopology, activity.RegisterOptions{Name: PersistRecipeTopologyActivityName})
	env.RegisterActivityWithOptions(fake.PersistRunCompiledPlan, activity.RegisterOptions{Name: PersistRunCompiledPlanActivityName})
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
	f.seq++

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
	f.seq++

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
	summary *models.Run_Summary,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++

	if summary == nil {
		f.summaries = append(f.summaries, nil)
		return nil
	}
	cloned := proto.Clone(summary).(*models.Run_Summary)
	f.summaries = append(f.summaries, cloned)
	f.persistedSummary = cloned
	f.persistedSummaryAt = f.seq
	return nil
}

func (f *fakeRuntimeActivities) PersistRecipeTopology(
	_ context.Context,
	_ string,
	snapshot *models.RunTopology,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++

	if snapshot == nil {
		f.recipeTopologies = append(f.recipeTopologies, nil)
		return nil
	}
	f.recipeTopologies = append(f.recipeTopologies, proto.Clone(snapshot).(*models.RunTopology))
	return nil
}

// PersistRunCompiledPlan is the fake double for
// PersistRunCompiledPlanActivityName (see runtime.go's
// persistRunCompiledPlan) — records every call plus the seq value at the
// moment of the LAST call, so tests can assert it ran (persistedCompiledPlan)
// and when, relative to other persist calls (persistedCompiledPlanAt).
func (f *fakeRuntimeActivities) PersistRunCompiledPlan(
	_ context.Context,
	_ string,
	plan *dslpb.CompiledPlan,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++

	if plan == nil {
		f.compiledPlans = append(f.compiledPlans, nil)
		return nil
	}
	cloned := proto.Clone(plan).(*dslpb.CompiledPlan)
	f.compiledPlans = append(f.compiledPlans, cloned)
	f.persistedCompiledPlan = cloned
	f.persistedCompiledPlanAt = f.seq
	return nil
}

func (f *fakeRuntimeActivities) AppendRunLogs(_ context.Context, lines []*monitor.LogLine) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++

	for _, line := range lines {
		if line == nil {
			f.logLines = append(f.logLines, nil)
			continue
		}
		f.logLines = append(f.logLines, proto.Clone(line).(*monitor.LogLine))
	}
	return nil
}
