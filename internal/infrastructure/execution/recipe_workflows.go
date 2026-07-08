package execution

import (
	"context"
	"log/slog"

	"go.temporal.io/sdk/client"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/settings"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/recipe"
	"github.com/stroppy-io/stroppy-cloud/internal/workflows"
)

// recipeTaskQueue is the Temporal task queue RunRecipeWorkflow (and every
// other cloud-owned workflow/activity — see internal/app/run.go's
// worker.New(tc, "stroppy-cloud", ...)) is dispatched on.
const recipeTaskQueue = "stroppy-cloud"

// workflowStarter is the minimal slice of client.Client RecipeWorkflows
// depends on to start and cancel RunRecipeWorkflow. Kept narrow (mirroring
// runStateQuerier in overview.go) so LaunchRecipeRun/CancelRecipeRun are
// unit-testable without a live Temporal server.
type workflowStarter interface {
	ExecuteWorkflow(ctx context.Context, options client.StartWorkflowOptions, workflow interface{}, args ...interface{}) (client.WorkflowRun, error)
	CancelWorkflow(ctx context.Context, workflowID string, runID string) error
}

// RecipeWorkflows implements recipe.RecipeWorkflows over a Temporal client.
// RunRecipeWorkflow (internal/workflows/runrecipe.go) has no generated proto
// workflow service of its own — like ExecuteCompiledPlanWorkflow, it
// registers directly against the SDK (see workflows.RegisterWorkflows) — so,
// unlike TestWorkflows/SuiteRunLauncher above, this launches it via the raw
// client.ExecuteWorkflow rather than a generated *ServiceClient.
type RecipeWorkflows struct {
	client    workflowStarter
	bootstrap settings.AgentBootstrapSource
	log       *slog.Logger
}

var _ recipe.RecipeWorkflows = (*RecipeWorkflows)(nil)

// NewRecipeWorkflows builds the recipe.RecipeWorkflows adapter. bootstrap
// supplies the AgentBootstrap delivered to provisioned agents (pass nil to
// launch without one). log may be nil.
func NewRecipeWorkflows(c client.Client, bootstrap settings.AgentBootstrapSource, log *slog.Logger) *RecipeWorkflows {
	if log == nil {
		log = slog.Default()
	}
	return &RecipeWorkflows{client: c, bootstrap: bootstrap, log: log}
}

// LaunchRecipeRun starts RunRecipeWorkflow for the persisted run, bound to
// the deterministic workflow id "run-recipe/"+runID (see
// runRecipeWorkflowID) so a duplicate launch for the same run is
// deduplicated by Temporal exactly like TestWorkflows.LaunchTest.
//
// Unlike TestWorkflows.testWorkflowRequest, this does NOT attach per-node
// agent tokens (attachAgentTokens): that helper needs a
// deployment.InfrastructurePlan naming every machine's node id up front, but
// a recipe run has no such plan at launch time — CompileRecipeActivity only
// resolves machine groups once RunRecipeWorkflow itself starts running (see
// runrecipe.go's package doc). Per-node agent auth for recipe runs is a
// documented follow-up for the provisioning activities (Task 4+), not this
// launch path.
func (w *RecipeWorkflows) LaunchRecipeRun(ctx context.Context, run *models.Run, bundle map[string][]byte) error {
	in, err := w.runRecipeInput(ctx, run, bundle)
	if err != nil {
		return err
	}
	opts := client.StartWorkflowOptions{
		ID:        runRecipeWorkflowID(run.GetEntity().GetId()),
		TaskQueue: recipeTaskQueue,
	}
	// Async start: the run is fire-and-forget from the API's perspective; the
	// Overview service later observes its progress via the persisted
	// RunState (see overview.go's overviewFromRecord / the package doc on
	// live-query being a follow-up for recipe runs).
	_, err = w.client.ExecuteWorkflow(ctx, opts, workflows.RunRecipeWorkflowName, in)
	return err
}

// CancelRecipeRun requests cancellation of the RunRecipeWorkflow launched
// for runID, addressed by the same deterministic workflow id
// LaunchRecipeRun used (runRecipeWorkflowID). The client.CancelWorkflow
// runID parameter (Temporal's own execution-run identifier, distinct from
// our runID) is left empty to target the workflow's current/latest
// execution. Cancellation is asynchronous: the workflow's own cancel
// handling persists the resulting run status; this call only requests it.
func (w *RecipeWorkflows) CancelRecipeRun(ctx context.Context, runID string) error {
	return w.client.CancelWorkflow(ctx, runRecipeWorkflowID(runID), "")
}

func (w *RecipeWorkflows) runRecipeInput(ctx context.Context, run *models.Run, bundle map[string][]byte) (*workflows.RunRecipeInput, error) {
	runID := run.GetEntity().GetId()
	if runID == "" {
		return nil, errRecipeRunMissingID
	}
	in := &workflows.RunRecipeInput{
		RunID:    runID,
		TenantID: run.GetEntity().GetTenantId(),
		Bundle:   bundle,
	}
	if w.bootstrap != nil {
		boot, err := w.bootstrap.AgentBootstrap(ctx)
		if err != nil {
			return nil, err
		}
		in.Bootstrap = boot
	}
	return in, nil
}
