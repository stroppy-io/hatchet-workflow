package workflows

import (
	"go.temporal.io/sdk/worker"

	gen "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

// RegisterServer registers the RunWorkflow + the server-side activities on the
// SERVER worker (task queue "stroppy-cloud"). The agent activities are NOT
// registered here — they live on each agent's worker (per-machine task queue).
func RegisterServer(w worker.Worker, sa *ServerActivities) {
	gen.RegisterRunWorkflowServiceWorkflows(w, Workflows{})
	w.RegisterActivity(sa.DeployMachinesActivity)
	w.RegisterActivity(sa.BuildRecipeActivity)
	w.RegisterActivity(sa.TeardownActivity)
}
