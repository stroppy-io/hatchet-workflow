package execution

import (
	"context"
	"errors"
	"log/slog"

	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/settings"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_run"
)

// TestWorkflows implements test_run.Workflows over a Temporal client. It starts
// the per-run TestWorkflow (deduplicated by the deterministic id derived from the
// run id) and cancels a running one.
type TestWorkflows struct {
	tc          workflowpb.TestServiceClient
	bootstrap   settings.AgentBootstrapSource
	agentTokens AgentTokenIssuer
	log         *slog.Logger
}

var _ test_run.Workflows = (*TestWorkflows)(nil)

// NewTestWorkflows builds the test_run.Workflows adapter. bootstrap supplies the
// AgentBootstrap delivered to provisioned agents; pass nil to launch without one
// (e.g. a topology that needs no remote agents). log may be nil.
func NewTestWorkflows(c client.Client, bootstrap settings.AgentBootstrapSource, agentTokens AgentTokenIssuer, log *slog.Logger) *TestWorkflows {
	if log == nil {
		log = slog.Default()
	}
	return &TestWorkflows{
		tc:          workflowpb.NewTestServiceClient(c, workflowpb.NewTestServiceClientOptions().WithLogger(log)),
		bootstrap:   bootstrap,
		agentTokens: agentTokens,
		log:         log,
	}
}

// LaunchTest starts TestWorkflow for the persisted run. The run's baked spec
// (rec.Spec) is the workflow input; an AgentBootstrap is resolved when a source
// is configured. The workflow id is derived from the run id by the generated
// options, so a duplicate launch for the same run is deduplicated by Temporal.
func (w *TestWorkflows) LaunchTest(ctx context.Context, run *models.TestRunRecord) error {
	spec := run.GetSpec()
	if spec == nil {
		return errors.New("test run record has no baked spec to launch")
	}

	req := &workflowpb.TestWorkflowRequest{TenantId: run.GetEntity().GetTenantId(), TestRun: spec}
	if w.bootstrap != nil {
		boot, err := w.bootstrap.AgentBootstrap(ctx)
		if err != nil {
			return err
		}
		boot, err = attachAgentTokens(boot, w.agentTokens, req.GetTenantId(), spec.GetId(), spec.GetInfrastructurePlan())
		if err != nil {
			return err
		}
		req.AgentBootstrap = boot
	}

	// Async start: the run is fire-and-forget from the API's perspective; the
	// Overview service later observes its progress via the GetRunState query.
	if _, err := w.tc.TestWorkflowAsync(ctx, req); err != nil {
		return err
	}
	return nil
}

// CancelTest signals cancellation of a running TestWorkflow. A workflow that is
// not running (already terminal / never started) is reported as
// derrors.ErrNotFound so the handler treats it as a no-op.
func (w *TestWorkflows) CancelTest(ctx context.Context, runID string) error {
	if err := w.tc.CancelWorkflow(ctx, testWorkflowID(runID), ""); err != nil {
		if isWorkflowNotFound(err) {
			return derrors.ErrNotFound
		}
		return err
	}
	return nil
}

// isWorkflowNotFound reports whether a Temporal error means the workflow does not
// exist (so a cancel/query against it is a no-op).
func isWorkflowNotFound(err error) bool {
	var nf *serviceerror.NotFound
	return errors.As(err, &nf)
}
