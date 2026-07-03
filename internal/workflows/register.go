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
