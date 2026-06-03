package workflows

import (
	"errors"
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	domainpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	enumsv1 "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type suiteWorkflows struct{}

func (w *suiteWorkflows) SuiteWorkflow(_ workflow.Context, input *workflowpb.SuiteWorkflowWorkflowInput) (workflowpb.SuiteWorkflowWorkflow, error) {
	return &suiteWorkflow{req: input.Req}, nil
}

type suiteWorkflow struct {
	req *workflowpb.SuiteWorkflowRequest
}

func (w *suiteWorkflow) Execute(ctx workflow.Context) (resp *workflowpb.SuiteWorkflowResponse, err error) {
	defer func() {
		if err == nil || !temporal.IsCanceledError(err) {
			return
		}
		disconnected, _ := workflow.NewDisconnectedContext(ctx)
		if perr := persistSuiteRun(disconnected, w.req.GetSuiteRunId(), common.Status_STATUS_CANCELLED); perr != nil {
			err = fmt.Errorf("persist cancelled suite run: %w", perr)
		}
	}()

	if w.req == nil {
		return nil, errors.New("suite workflow request is required")
	}
	if err := w.req.Validate(); err != nil {
		if perr := persistSuiteRun(ctx, w.req.GetSuiteRunId(), common.Status_STATUS_FAILED); perr != nil {
			return nil, fmt.Errorf("persist failed suite run: %w", perr)
		}
		return nil, err
	}
	if err := persistSuiteRun(ctx, w.req.GetSuiteRunId(), common.Status_STATUS_RUNNING); err != nil {
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

		child, err := workflowpb.TestWorkflowChildAsync(ctx, testWorkflowRequest(run), suiteChildOptions())
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
			if perr := persistSuiteRun(ctx, w.req.GetSuiteRunId(), common.Status_STATUS_FAILED); perr != nil {
				return nil, fmt.Errorf("persist failed suite run: %w", perr)
			}
			return nil, err
		}
	}

	for completed < len(runs) {
		selector.Select(ctx)
		if firstErr != nil {
			if err := persistSuiteRun(ctx, w.req.GetSuiteRunId(), common.Status_STATUS_FAILED); err != nil {
				return nil, err
			}
			return nil, firstErr
		}
		if err := persistSuiteRun(ctx, w.req.GetSuiteRunId(), common.Status_STATUS_UNSPECIFIED); err != nil {
			return nil, err
		}
		for started < len(runs) && inFlight < maxParallel {
			if err := startNext(); err != nil {
				if perr := persistSuiteRun(ctx, w.req.GetSuiteRunId(), common.Status_STATUS_FAILED); perr != nil {
					return nil, fmt.Errorf("persist failed suite run: %w", perr)
				}
				return nil, err
			}
		}
	}

	if err := persistSuiteRun(ctx, w.req.GetSuiteRunId(), common.Status_STATUS_COMPLETED); err != nil {
		return nil, err
	}
	return &workflowpb.SuiteWorkflowResponse{}, nil
}

func suiteChildOptions() *workflowpb.TestWorkflowChildOptions {
	return workflowpb.NewTestWorkflowChildOptions().
		WithParentClosePolicy(enumsv1.PARENT_CLOSE_POLICY_REQUEST_CANCEL).
		WithWaitForCancellation(true)
}

func testWorkflowRequest(run *workflowpb.RunConfig) *workflowpb.TestWorkflowRequest {
	return &workflowpb.TestWorkflowRequest{
		TenantId: run.GetTenantId(),
		TestRun: &domainpb.TestRun{
			Id:                 run.GetId(),
			Database:           run.GetDatabase(),
			Workload:           run.GetWorkload(),
			TopologySpec:       run.GetTopologySpec(),
			InfrastructurePlan: run.GetInfrastructurePlan(),
			RenderOverrides:    run.GetRenderOverrides(),
		},
		AgentBootstrap: run.GetAgentBootstrap(),
	}
}
