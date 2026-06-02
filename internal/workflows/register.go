package workflows

import (
	databasepostgres "github.com/stroppy-io/stroppy-cloud/internal/domain/database/postgres"
	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/packages"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"go.temporal.io/sdk/worker"
)

type Options struct {
	PackageResolver     packages.Resolver
	DeploymentRenderers deploymentbuilder.Registry
}

func DefaultOptions() Options {
	return Options{
		PackageResolver:     packages.NewRegistry(databasepostgres.PackageResolver{}),
		DeploymentRenderers: deploymentbuilder.NewRegistry(databasepostgres.DeploymentRenderer{}),
	}
}

func RegisterWorkflows(registry worker.WorkflowRegistry, options Options) {
	workflowpb.RegisterDeploymentServiceWorkflows(registry, NewDeploymentWorkflows(options))
	workflowpb.RegisterTestServiceWorkflows(registry, NewTestWorkflows())
	workflowpb.RegisterRunWorkflowServiceWorkflows(registry, NewRunWorkflows())
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
