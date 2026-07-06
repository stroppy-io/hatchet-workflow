package workflows

import (
	"context"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	databasecockroach "github.com/stroppy-io/stroppy-cloud/internal/domain/database/cockroach"
	databasemysql "github.com/stroppy-io/stroppy-cloud/internal/domain/database/mysql"
	databasenoop "github.com/stroppy-io/stroppy-cloud/internal/domain/database/noop"
	databaseorioledb "github.com/stroppy-io/stroppy-cloud/internal/domain/database/orioledb"
	databasepgnoop "github.com/stroppy-io/stroppy-cloud/internal/domain/database/pgnoop"
	databasepicodata "github.com/stroppy-io/stroppy-cloud/internal/domain/database/picodata"
	databasepostgres "github.com/stroppy-io/stroppy-cloud/internal/domain/database/postgres"
	databaseydb "github.com/stroppy-io/stroppy-cloud/internal/domain/database/ydb"
	databaseydbmanaged "github.com/stroppy-io/stroppy-cloud/internal/domain/database/ydbmanaged"
	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/packages"
	workloadbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/workload"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

const (
	PersistRunStateActivityName       = "stroppy.runtime.PersistRunState"
	PersistDeploymentPlanActivityName = "stroppy.runtime.PersistDeploymentPlan"
	AppendRunLogsActivityName         = "stroppy.runtime.AppendRunLogs"
	PersistSuiteRunActivityName       = "stroppy.runtime.PersistSuiteRun"
)

type RuntimeActivities interface {
	PersistRunState(context.Context, string, *workflowpb.RunState, *deploymentpb.InfrastructureState, *deploymentpb.DeploymentPlan) error
	PersistDeploymentPlan(context.Context, string, *deploymentpb.DeploymentPlan) error
	AppendRunLogs(context.Context, []*monitor.LogLine) error
	PersistSuiteRun(context.Context, string, common.Status) error
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

type Options struct {
	PackageResolver     packages.Resolver
	DeploymentRenderers deploymentbuilder.Registry
}

type ActivityOptions struct {
	Quotas   QuotaManager
	Networks NetworkManager
	Logs     RunLogWriter
}

func DefaultOptions() Options {
	return Options{
		PackageResolver: packages.NewRegistry(
			databasepostgres.PackageResolver{},
			databasepicodata.PackageResolver{},
			databaseydb.PackageResolver{},
			databaseydbmanaged.PackageResolver{},
			databasecockroach.PackageResolver{},
			databasemysql.PackageResolver{},
			databaseorioledb.PackageResolver{},
			databasepgnoop.PackageResolver{},
			databasenoop.PackageResolver{},
		),
		DeploymentRenderers: deploymentbuilder.NewRegistry(
			databasepostgres.DeploymentRenderer{},
			databasepicodata.DeploymentRenderer{},
			databaseydb.DeploymentRenderer{},
			databaseydbmanaged.DeploymentRenderer{},
			databasecockroach.DeploymentRenderer{},
			databasemysql.DeploymentRenderer{},
			databaseorioledb.DeploymentRenderer{},
			databasepgnoop.DeploymentRenderer{},
			workloadbuilder.DeploymentRenderer{},
		),
	}
}

func RegisterWorkflows(registry worker.WorkflowRegistry, options Options) {
	workflowpb.RegisterDeploymentServiceWorkflows(registry, NewDeploymentWorkflows(options))
	workflowpb.RegisterTestServiceWorkflows(registry, NewTestWorkflows())
	workflowpb.RegisterSuiteWorkflowServiceWorkflows(registry, NewSuiteWorkflows())
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

func RegisterActivities(registry worker.ActivityRegistry, runtime RuntimeActivities, options ...ActivityOptions) {
	opts := ActivityOptions{}
	if len(options) > 0 {
		opts = options[0]
	}
	workflowpb.RegisterDeploymentServiceActivities(registry, NewDeploymentActivities(opts.Quotas, opts.Networks, opts.Logs))
	if runtime != nil {
		registry.RegisterActivityWithOptions(runtime.PersistRunState, activity.RegisterOptions{Name: PersistRunStateActivityName})
		registry.RegisterActivityWithOptions(runtime.PersistDeploymentPlan, activity.RegisterOptions{Name: PersistDeploymentPlanActivityName})
		registry.RegisterActivityWithOptions(runtime.AppendRunLogs, activity.RegisterOptions{Name: AppendRunLogsActivityName})
		registry.RegisterActivityWithOptions(runtime.PersistSuiteRun, activity.RegisterOptions{Name: PersistSuiteRunActivityName})
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

func NewDeploymentWorkflows(options Options) workflowpb.DeploymentServiceWorkflows {
	return &deploymentWorkflows{options: normalizeOptions(options)}
}

func NewTestWorkflows() workflowpb.TestServiceWorkflows {
	return &testWorkflows{}
}

func NewSuiteWorkflows() workflowpb.SuiteWorkflowServiceWorkflows {
	return &suiteWorkflows{}
}

func normalizeOptions(options Options) Options {
	defaults := DefaultOptions()
	if options.PackageResolver == nil {
		options.PackageResolver = defaults.PackageResolver
	}
	if options.DeploymentRenderers.IsEmpty() {
		options.DeploymentRenderers = defaults.DeploymentRenderers
	}
	return options
}
