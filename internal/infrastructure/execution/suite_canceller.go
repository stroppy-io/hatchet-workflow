package execution

import (
	"context"

	"go.temporal.io/sdk/client"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"github.com/stroppy-io/stroppy-cloud/internal/services/suite_run"
)

// SuiteRunCanceller implements suite_run.SuiteRunCanceller over a Temporal
// client. Cancelling the SuiteWorkflow propagates to its child TestRunWorkflows.
// Idempotent for the runtime: signalling an already-finished / never-started
// workflow is a no-op (a not-found cancel is swallowed).
type SuiteRunCanceller struct {
	tc workflowpb.SuiteWorkflowServiceClient
}

var _ suite_run.SuiteRunCanceller = (*SuiteRunCanceller)(nil)

// NewSuiteRunCanceller builds the suite_run.SuiteRunCanceller adapter.
func NewSuiteRunCanceller(c client.Client) *SuiteRunCanceller {
	return &SuiteRunCanceller{tc: workflowpb.NewSuiteWorkflowServiceClient(c)}
}

// CancelSuiteRun requests cancellation of the suite workflow (and, transitively,
// its child test workflows). A not-running workflow is treated as a no-op.
func (c *SuiteRunCanceller) CancelSuiteRun(ctx context.Context, suiteRunID string) error {
	if err := c.tc.CancelWorkflow(ctx, suiteWorkflowID(suiteRunID), ""); err != nil {
		if isWorkflowNotFound(err) {
			return derrors.ErrNotFound
		}
		return err
	}
	return nil
}
