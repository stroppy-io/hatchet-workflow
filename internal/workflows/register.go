package workflows

import (
	"context"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

const (
	PersistRunStateActivityName       = "stroppy.runtime.PersistRunState"
	PersistDeploymentPlanActivityName = "stroppy.runtime.PersistDeploymentPlan"
	AppendRunLogsActivityName         = "stroppy.runtime.AppendRunLogs"
	// PersistRunSummaryActivityName merges a recipe-derived
	// TestRunRecord.Summary onto the run record — see runtime.go's
	// persistRunSummary and runrecipe_summary.go's deriveRunSummary.
	PersistRunSummaryActivityName = "stroppy.runtime.PersistRunSummary"
)

type RuntimeActivities interface {
	PersistRunState(context.Context, string, *workflowpb.RunState, *deploymentpb.InfrastructureState, *deploymentpb.DeploymentPlan) error
	PersistDeploymentPlan(context.Context, string, *deploymentpb.DeploymentPlan) error
	AppendRunLogs(context.Context, []*monitor.LogLine) error
	// PersistRunSummary merges the given Summary's non-zero fields onto the
	// run record's stored Summary (see execution.RunPersistenceActivities.
	// PersistRunSummary) — additive, never clobbers the timing/progress
	// facets PersistRunState's own applyRunSummary maintains.
	PersistRunSummary(context.Context, string, *models.TestRunRecord_Summary) error
}

// RecipeActivityImpl is the interface RunRecipeWorkflow's three by-name
// activities (CompileRecipeActivityName/ProvisionActivityName/
// TeardownActivityName — see runrecipe.go) must satisfy to register a real
// implementation via RegisterRecipeActivities. It exists as an interface
// here — rather than RegisterRecipeActivities simply taking a concrete
// *execution.RecipeActivities — so this package never needs to import
// internal/infrastructure/execution: execution already imports this package
// for the CompileRecipeActivityInput/Output etc. types (see recipe_
// activities.go), and workflows importing execution back would cycle.
// *execution.RecipeActivities satisfies this interface structurally, so
// app wiring (Task 6) passes it here without either package naming the
// other's concrete type. Mirrors RuntimeActivities' own precedent above.
type RecipeActivityImpl interface {
	CompileRecipeActivity(context.Context, *CompileRecipeActivityInput) (*CompileRecipeActivityOutput, error)
	ProvisionActivity(context.Context, *ProvisionActivityInput) (*ProvisionActivityOutput, error)
	TeardownActivity(context.Context, *TeardownActivityInput) error
}

func RegisterWorkflows(registry worker.WorkflowRegistry) {
	// ExecuteCompiledPlanWorkflow (dslrun.go) is the generic YAML-DSL
	// interpreter. It has no proto workflow service of its own (dslpb.
	// CompiledPlan carries no RPC definitions), so it registers directly
	// against the SDK rather than through a RegisterXxxServiceWorkflows
	// codegen entry point like the workflows above.
	registry.RegisterWorkflowWithOptions(ExecuteCompiledPlanWorkflow, workflow.RegisterOptions{
		Name: ExecuteCompiledPlanWorkflowName,
	})
	// RunRecipeWorkflow (runrecipe.go) orchestrates one recipe run
	// (compile -> provision -> execute -> teardown). Like
	// ExecuteCompiledPlanWorkflow, it has no proto workflow service of its
	// own, so it registers directly against the SDK too.
	registry.RegisterWorkflowWithOptions(RunRecipeWorkflow, workflow.RegisterOptions{
		Name: RunRecipeWorkflowName,
	})
}

func RegisterActivities(registry worker.ActivityRegistry, runtime RuntimeActivities) {
	if runtime != nil {
		registry.RegisterActivityWithOptions(runtime.PersistRunState, activity.RegisterOptions{Name: PersistRunStateActivityName})
		registry.RegisterActivityWithOptions(runtime.PersistDeploymentPlan, activity.RegisterOptions{Name: PersistDeploymentPlanActivityName})
		registry.RegisterActivityWithOptions(runtime.AppendRunLogs, activity.RegisterOptions{Name: AppendRunLogsActivityName})
		registry.RegisterActivityWithOptions(runtime.PersistRunSummary, activity.RegisterOptions{Name: PersistRunSummaryActivityName})
	}
}

// RegisterRecipeActivities registers a RecipeActivityImpl's three methods
// under the exact activity names RunRecipeWorkflow (runrecipe.go) calls by
// name — CompileRecipeActivityName/ProvisionActivityName/
// TeardownActivityName. impl is typically *execution.RecipeActivities
// (internal/infrastructure/execution/recipe_activities.go, Task 4),
// constructed by app wiring (Task 6) with the real provider.Deps. A nil impl
// is a documented no-op, mirroring RegisterActivities' own runtime==nil
// guard above, so a caller that has not wired recipe execution yet can still
// call this without special-casing it.
func RegisterRecipeActivities(registry worker.ActivityRegistry, impl RecipeActivityImpl) {
	if impl == nil {
		return
	}
	registry.RegisterActivityWithOptions(impl.CompileRecipeActivity, activity.RegisterOptions{Name: CompileRecipeActivityName})
	registry.RegisterActivityWithOptions(impl.ProvisionActivity, activity.RegisterOptions{Name: ProvisionActivityName})
	registry.RegisterActivityWithOptions(impl.TeardownActivity, activity.RegisterOptions{Name: TeardownActivityName})
}
