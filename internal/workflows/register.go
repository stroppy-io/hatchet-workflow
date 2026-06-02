package workflows

import (
	databasecockroach "github.com/stroppy-io/stroppy-cloud/internal/domain/database/cockroach"
	databasemysql "github.com/stroppy-io/stroppy-cloud/internal/domain/database/mysql"
	databasepicodata "github.com/stroppy-io/stroppy-cloud/internal/domain/database/picodata"
	databasepostgres "github.com/stroppy-io/stroppy-cloud/internal/domain/database/postgres"
	databaseydb "github.com/stroppy-io/stroppy-cloud/internal/domain/database/ydb"
	databaseydbmanaged "github.com/stroppy-io/stroppy-cloud/internal/domain/database/ydbmanaged"
	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/packages"
	workloadbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/workload"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"go.temporal.io/sdk/worker"
)

type Options struct {
	PackageResolver     packages.Resolver
	DeploymentRenderers deploymentbuilder.Registry
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
		),
		DeploymentRenderers: deploymentbuilder.NewRegistry(
			databasepostgres.DeploymentRenderer{},
			databasepicodata.DeploymentRenderer{},
			databaseydb.DeploymentRenderer{},
			databaseydbmanaged.DeploymentRenderer{},
			databasecockroach.DeploymentRenderer{},
			databasemysql.DeploymentRenderer{},
			workloadbuilder.DeploymentRenderer{},
		),
	}
}

func RegisterWorkflows(registry worker.WorkflowRegistry, options Options) {
	workflowpb.RegisterDeploymentServiceWorkflows(registry, NewDeploymentWorkflows(options))
	workflowpb.RegisterTestServiceWorkflows(registry, NewTestWorkflows())
	workflowpb.RegisterRunWorkflowServiceWorkflows(registry, NewRunWorkflows())
	workflowpb.RegisterSuiteWorkflowServiceWorkflows(registry, NewSuiteWorkflows())
}

func RegisterActivities(registry worker.ActivityRegistry) {
	workflowpb.RegisterDeploymentServiceActivities(registry, NewDeploymentActivities())
}

func NewDeploymentWorkflows(options Options) workflowpb.DeploymentServiceWorkflows {
	return &deploymentWorkflows{options: normalizeOptions(options)}
}

func NewRunWorkflows() workflowpb.RunWorkflowServiceWorkflows {
	return &runWorkflows{}
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
