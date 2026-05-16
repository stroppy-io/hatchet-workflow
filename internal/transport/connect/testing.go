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

// TestingHandler implements TestRunTemplateServiceHandler, TestRunServiceHandler,
// and TestSuiteServiceHandler.
type TestingHandler struct {
	templates *testingsvc.TemplateService
	runs      *testingsvc.TestRunService
	suites    *testingsvc.TestSuiteService
}

// NewTestingHandler constructs a TestingHandler.
func NewTestingHandler(
	templates *testingsvc.TemplateService,
	runs *testingsvc.TestRunService,
	suites *testingsvc.TestSuiteService,
) *TestingHandler {
	return &TestingHandler{templates: templates, runs: runs, suites: suites}
}

// ─── TestRunTemplateService ──────────────────────────────────────────────────

func (h *TestingHandler) CreateTestRunTemplate(ctx context.Context, req *connect.Request[testingpb.CreateTestRunTemplateRequest]) (*connect.Response[testingpb.TestRunTemplate], error) {
	tenantID := &iampb.TenantId{Value: middleware.TenantFromCtx(ctx)}
	callerID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	result, err := h.templates.CreateTestRunTemplate(ctx, tenantID, callerID, req.Msg.GetTemplate())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *TestingHandler) UpdateTestRunTemplate(ctx context.Context, req *connect.Request[testingpb.UpdateTestRunTemplateRequest]) (*connect.Response[testingpb.TestRunTemplate], error) {
	result, err := h.templates.UpdateTestRunTemplate(ctx, req.Msg.GetTemplate())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *TestingHandler) DeleteTestRunTemplate(ctx context.Context, req *connect.Request[testingpb.TestRunTemplateId]) (*connect.Response[testingpb.TestRunTemplate], error) {
	existing, err := h.templates.GetTestRunTemplate(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	if err := h.templates.DeleteTestRunTemplate(ctx, req.Msg); err != nil {
		return nil, err
	}
	return connect.NewResponse(existing), nil
}

func (h *TestingHandler) GetTestRunTemplate(ctx context.Context, req *connect.Request[testingpb.TestRunTemplateId]) (*connect.Response[testingpb.TestRunTemplate], error) {
	result, err := h.templates.GetTestRunTemplate(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *TestingHandler) ListTestRunTemplates(ctx context.Context, req *connect.Request[iampb.TenantId]) (*connect.Response[testingpb.TestRunTemplate_List], error) {
	items, err := h.templates.ListTestRunTemplates(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&testingpb.TestRunTemplate_List{TestRunTemplates: items}), nil
}

// ─── TestSuiteService ────────────────────────────────────────────────────────

func (h *TestingHandler) CreateTestSuite(ctx context.Context, req *connect.Request[testingpb.CreateTestSuiteRequest]) (*connect.Response[testingpb.TestSuite], error) {
	tenantID := &iampb.TenantId{Value: middleware.TenantFromCtx(ctx)}
	callerID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	result, err := h.suites.CreateTestSuite(ctx, tenantID, callerID, req.Msg.GetSuite())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *TestingHandler) UpdateTestSuite(ctx context.Context, req *connect.Request[testingpb.UpdateTestSuiteRequest]) (*connect.Response[testingpb.TestSuite], error) {
	result, err := h.suites.UpdateTestSuite(ctx, req.Msg.GetSuite())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *TestingHandler) DeleteTestSuite(ctx context.Context, req *connect.Request[testingpb.TestSuiteId]) (*connect.Response[testingpb.TestSuite], error) {
	result, err := h.suites.DeleteTestSuite(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *TestingHandler) GetTestSuite(ctx context.Context, req *connect.Request[testingpb.TestSuiteId]) (*connect.Response[testingpb.TestSuite], error) {
	result, err := h.suites.GetTestSuite(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *TestingHandler) ListTestSuites(ctx context.Context, req *connect.Request[iampb.TenantId]) (*connect.Response[testingpb.TestSuite_List], error) {
	items, err := h.suites.ListTestSuites(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&testingpb.TestSuite_List{TestSuites: items}), nil
}

func (h *TestingHandler) CloneTestSuite(ctx context.Context, req *connect.Request[testingpb.TestSuiteId]) (*connect.Response[testingpb.TestSuite], error) {
	callerID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	result, err := h.suites.CloneTestSuite(ctx, req.Msg, callerID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

// ─── TestRunService ──────────────────────────────────────────────────────────

func (h *TestingHandler) CreateTestRun(ctx context.Context, req *connect.Request[testingpb.CreateTestRunRequest]) (*connect.Response[testingpb.TestRun], error) {
	tenantID := &iampb.TenantId{Value: middleware.TenantFromCtx(ctx)}
	callerID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	result, err := h.runs.CreateTestRun(ctx, tenantID, callerID, req.Msg.GetTestRun())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *TestingHandler) UpdateTestRun(ctx context.Context, req *connect.Request[testingpb.UpdateTestRunRequest]) (*connect.Response[testingpb.TestRun], error) {
	result, err := h.runs.UpdateTestRun(ctx, req.Msg.GetTestRun())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *TestingHandler) DeleteTestRun(ctx context.Context, req *connect.Request[testingpb.TestRunId]) (*connect.Response[testingpb.TestRun], error) {
	result, err := h.runs.DeleteTestRun(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *TestingHandler) GetTestRun(ctx context.Context, req *connect.Request[testingpb.TestRunId]) (*connect.Response[testingpb.GetTestRunResponse], error) {
	tr, err := h.runs.GetTestRun(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&testingpb.GetTestRunResponse{TestRun: tr}), nil
}

func (h *TestingHandler) ListTestRuns(ctx context.Context, req *connect.Request[iampb.TenantId]) (*connect.Response[testingpb.TestRun_List], error) {
	items, err := h.runs.ListTestRuns(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&testingpb.TestRun_List{TestRuns: items}), nil
}

func (h *TestingHandler) WatchTestRun(_ context.Context, _ *connect.Request[testingpb.WatchTestRunRequest], _ *connect.ServerStream[testingpb.TestRunProgress]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *TestingHandler) StreamTestRunLogs(_ context.Context, _ *connect.Request[testingpb.StreamTestRunLogsRequest], _ *connect.ServerStream[agentpb.LogLine]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *TestingHandler) GetTestRunMetrics(_ context.Context, _ *connect.Request[testingpb.GetTestRunMetricsRequest]) (*connect.Response[testingpb.MetricSeriesList], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *TestingHandler) LaunchTestRun(ctx context.Context, req *connect.Request[testingpb.TestRunId]) (*connect.Response[testingpb.TestRun], error) {
	result, err := h.runs.LaunchTestRun(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *TestingHandler) CancelTestRun(ctx context.Context, req *connect.Request[testingpb.TestRunId]) (*connect.Response[testingpb.TestRun], error) {
	if err := h.runs.CancelTestRun(ctx, req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeUnimplemented, err)
	}
	tr, err := h.runs.GetTestRun(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(tr), nil
}

func (h *TestingHandler) InstantiateTestRun(ctx context.Context, req *connect.Request[testingpb.TestRunTemplateId]) (*connect.Response[testingpb.TestRun], error) {
	callerID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	result, err := h.runs.InstantiateTestRun(ctx, req.Msg, callerID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}
