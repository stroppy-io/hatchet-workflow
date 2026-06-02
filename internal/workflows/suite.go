package workflows

import (
	"errors"

	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"go.temporal.io/sdk/workflow"
)

type suiteWorkflows struct{}

func (w *suiteWorkflows) SuiteWorkflow(_ workflow.Context, input *workflowpb.SuiteWorkflowWorkflowInput) (workflowpb.SuiteWorkflowWorkflow, error) {
	return &suiteWorkflow{req: input.Req}, nil
}

type suiteWorkflow struct {
	req *workflowpb.SuiteWorkflowRequest
}

func (w *suiteWorkflow) Execute(ctx workflow.Context) (*workflowpb.SuiteWorkflowResponse, error) {
	if w.req == nil {
		return nil, errors.New("suite workflow request is required")
	}
	if err := w.req.Validate(); err != nil {
		return nil, err
	}

	runs := w.req.GetRuns()
	maxParallel := int(w.req.GetMaxParallel())
	if maxParallel == 0 || maxParallel > len(runs) {
		maxParallel = len(runs)
	}

	selector := workflow.NewSelector(ctx)
	started := 0
	completed := 0
	inFlight := 0
	var firstErr error

	startNext := func() error {
		run := runs[started]
		started++
		inFlight++

		child, err := workflowpb.TestRunWorkflowChildAsync(ctx, run)
		if err != nil {
			return err
		}
		selector.AddFuture(child.Future, func(f workflow.Future) {
			if err := f.Get(ctx, nil); err != nil && firstErr == nil {
				firstErr = err
			}
			completed++
			inFlight--
		})
		return nil
	}

	for started < len(runs) && inFlight < maxParallel {
		if err := startNext(); err != nil {
			return nil, err
		}
	}

	for completed < len(runs) {
		selector.Select(ctx)
		if firstErr != nil {
			return nil, firstErr
		}
		for started < len(runs) && inFlight < maxParallel {
			if err := startNext(); err != nil {
				return nil, err
			}
		}
	}

	return &workflowpb.SuiteWorkflowResponse{}, nil
}
