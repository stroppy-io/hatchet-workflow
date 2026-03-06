package api

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"

	apiv1 "github.com/stroppy-io/hatchet-workflow/internal/proto/api/v1"
	workflowTest "github.com/stroppy-io/hatchet-workflow/internal/domain/workflows/test"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/workflows"

	"github.com/hatchet-dev/hatchet/pkg/client/rest"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type testHandler struct {
	hatchet       *hatchetLib.Client
	settingsStore *SettingsStore
}

func NewTestHandler(hatchet *hatchetLib.Client, settingsStore *SettingsStore) *testHandler {
	return &testHandler{hatchet: hatchet, settingsStore: settingsStore}
}

func (h *testHandler) RunTestSuite(ctx context.Context, req *connect.Request[apiv1.RunTestSuiteRequest]) (*connect.Response[apiv1.RunTestSuiteResponse], error) {
	suite := req.Msg.GetTestSuite()
	if suite == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("test_suite is required"))
	}

	cfg, err := h.settingsStore.Load()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load settings: %w", err))
	}

	input := &workflows.Workflows_StroppyTestSuite_Input{
		Suite:    suite,
		Settings: cfg,
		Target:   cfg.GetPreferredTarget(),
	}

	result, err := h.hatchet.Run(ctx, workflowTest.SuiteWorkflowName, input)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("run workflow: %w", err))
	}

	return connect.NewResponse(&apiv1.RunTestSuiteResponse{
		RunId: result.RunId,
	}), nil
}

func (h *testHandler) GetTestStatus(ctx context.Context, req *connect.Request[apiv1.GetTestStatusRequest]) (*connect.Response[apiv1.GetTestStatusResponse], error) {
	runID := req.Msg.GetRunId()
	if runID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("run_id is required"))
	}

	details, err := h.hatchet.Runs().Get(ctx, runID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get run: %w", err))
	}

	status := mapTaskStatus(details.Run.Status)
	currentStep, progress := computeProgress(details)

	return connect.NewResponse(&apiv1.GetTestStatusResponse{
		Status:      status,
		CurrentStep: currentStep,
		Progress:    progress,
	}), nil
}

func (h *testHandler) GetTestResult(ctx context.Context, req *connect.Request[apiv1.GetTestResultRequest]) (*connect.Response[apiv1.GetTestResultResponse], error) {
	runID := req.Msg.GetRunId()
	if runID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("run_id is required"))
	}

	details, err := h.hatchet.Runs().Get(ctx, runID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get run: %w", err))
	}

	if details.Run.Status != rest.V1TaskStatusCOMPLETED {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("run is not completed, status: %s", details.Run.Status))
	}

	// Result extraction would need output parsing - return empty for now
	return connect.NewResponse(&apiv1.GetTestResultResponse{}), nil
}

func (h *testHandler) ListTestRuns(ctx context.Context, req *connect.Request[apiv1.ListTestRunsRequest]) (*connect.Response[apiv1.ListTestRunsResponse], error) {
	pageSize := int64(req.Msg.GetPageSize())
	if pageSize <= 0 {
		pageSize = 20
	}

	var offset int64
	// page_token is opaque - we use offset-based pagination
	if req.Msg.GetPageToken() != "" {
		fmt.Sscanf(req.Msg.GetPageToken(), "%d", &offset)
	}

	listResult, err := h.hatchet.Runs().List(ctx, rest.V1WorkflowRunListParams{
		Limit:  &pageSize,
		Offset: &offset,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list runs: %w", err))
	}

	runs := make([]*apiv1.TestRunSummary, 0, len(listResult.Rows))
	for _, row := range listResult.Rows {
		summary := &apiv1.TestRunSummary{
			RunId:     row.WorkflowRunExternalId.String(),
			Status:    mapTaskStatus(row.Status),
			CreatedAt: timestamppb.New(row.CreatedAt),
		}
		if row.FinishedAt != nil {
			summary.CompletedAt = timestamppb.New(*row.FinishedAt)
		}
		if row.WorkflowName != nil {
			summary.TestSuiteName = *row.WorkflowName
		}
		runs = append(runs, summary)
	}

	var nextPageToken string
	nextOffset := offset + pageSize
	if int64(len(listResult.Rows)) == pageSize {
		nextPageToken = fmt.Sprintf("%d", nextOffset)
	}

	return connect.NewResponse(&apiv1.ListTestRunsResponse{
		Runs:          runs,
		NextPageToken: nextPageToken,
	}), nil
}

func mapTaskStatus(s rest.V1TaskStatus) apiv1.TestRunStatus {
	switch s {
	case rest.V1TaskStatusQUEUED:
		return apiv1.TestRunStatus_TEST_RUN_STATUS_PENDING
	case rest.V1TaskStatusRUNNING:
		return apiv1.TestRunStatus_TEST_RUN_STATUS_RUNNING
	case rest.V1TaskStatusCOMPLETED:
		return apiv1.TestRunStatus_TEST_RUN_STATUS_COMPLETED
	case rest.V1TaskStatusFAILED:
		return apiv1.TestRunStatus_TEST_RUN_STATUS_FAILED
	case rest.V1TaskStatusCANCELLED:
		return apiv1.TestRunStatus_TEST_RUN_STATUS_CANCELLED
	default:
		return apiv1.TestRunStatus_TEST_RUN_STATUS_NONE
	}
}

func computeProgress(details *rest.V1WorkflowRunDetails) (string, float32) {
	if len(details.Tasks) == 0 {
		return "", 0
	}

	total := len(details.Tasks)
	completed := 0
	currentStep := ""

	for _, task := range details.Tasks {
		switch task.Status {
		case rest.V1TaskStatusCOMPLETED:
			completed++
		case rest.V1TaskStatusRUNNING:
			currentStep = task.DisplayName
		}
	}

	return currentStep, float32(completed) / float32(total)
}
