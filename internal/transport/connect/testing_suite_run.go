package connect

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	testingsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/testing"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
	"github.com/stroppy-io/stroppy-cloud/internal/transport/middleware"
)

// TestSuiteRunHandler implements TestSuiteRunServiceHandler.
type TestSuiteRunHandler struct {
	svc *testingsvc.TestSuiteRunService
}

// NewTestSuiteRunHandler constructs a TestSuiteRunHandler.
func NewTestSuiteRunHandler(svc *testingsvc.TestSuiteRunService) *TestSuiteRunHandler {
	return &TestSuiteRunHandler{svc: svc}
}

func (h *TestSuiteRunHandler) GetTestSuiteRun(ctx context.Context, req *connect.Request[testingpb.TestSuiteRunId]) (*connect.Response[testingpb.GetTestSuiteRunResponse], error) {
	sr, err := h.svc.GetTestSuiteRun(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&testingpb.GetTestSuiteRunResponse{SuiteRun: sr}), nil
}

func (h *TestSuiteRunHandler) ListBySuite(ctx context.Context, req *connect.Request[testingpb.TestSuiteId]) (*connect.Response[testingpb.TestSuiteRun_List], error) {
	items, err := h.svc.ListBySuite(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&testingpb.TestSuiteRun_List{TestSuiteRuns: items}), nil
}

func (h *TestSuiteRunHandler) ListByTenant(ctx context.Context, req *connect.Request[iampb.TenantId]) (*connect.Response[testingpb.TestSuiteRun_List], error) {
	items, err := h.svc.ListByTenant(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&testingpb.TestSuiteRun_List{TestSuiteRuns: items}), nil
}

func (h *TestSuiteRunHandler) LaunchTestSuite(ctx context.Context, req *connect.Request[testingpb.TestSuiteId]) (*connect.Response[testingpb.TestSuiteRun], error) {
	callerID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	result, err := h.svc.LaunchTestSuite(ctx, req.Msg, callerID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *TestSuiteRunHandler) CancelTestSuiteRun(ctx context.Context, req *connect.Request[testingpb.TestSuiteRunId]) (*connect.Response[testingpb.TestSuiteRun], error) {
	if err := h.svc.CancelTestSuiteRun(ctx, req.Msg); err != nil {
		return nil, err
	}
	sr, err := h.svc.GetTestSuiteRun(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(sr), nil
}

func (h *TestSuiteRunHandler) WatchTestSuiteRun(_ context.Context, _ *connect.Request[testingpb.WatchTestSuiteRunRequest], _ *connect.ServerStream[testingpb.TestSuiteRunProgress]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *TestSuiteRunHandler) StreamTestSuiteRunLogs(_ context.Context, _ *connect.Request[testingpb.StreamTestSuiteRunLogsRequest], _ *connect.ServerStream[agentpb.LogLine]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *TestSuiteRunHandler) GetTestSuiteRunMetrics(_ context.Context, _ *connect.Request[testingpb.GetTestSuiteRunMetricsRequest]) (*connect.Response[testingpb.MetricSeriesList], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}
