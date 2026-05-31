package workflows

import (
	"go.temporal.io/sdk/worker"

	gen "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

// RegisterAll registers the TestService workflows and the server activities on
// the SERVER worker (task queue "stroppy-cloud"). The server activities are
// registered by value so ExecuteActivity-by-func works (the workflow references
// them as sa.DeployActivity etc.). The agent activities are NOT registered here;
// they live on the separate agent worker (task queue "stroppy-agent").
func RegisterAll(w worker.Worker, sa *ServerActivities) {
	gen.RegisterTestServiceWorkflows(w, Workflows{})
	w.RegisterActivity(sa.DeployActivity)
	w.RegisterActivity(sa.BuildRecipeActivity)
	w.RegisterActivity(sa.TeardownActivity)
}
