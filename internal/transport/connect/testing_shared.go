package connect

import (
	"context"
	"time"

	"connectrpc.com/connect"

	testingsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/testing"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
	"github.com/stroppy-io/stroppy-cloud/internal/transport/middleware"
)

// SharedTestRunHandler implements SharedTestRunServiceHandler.
type SharedTestRunHandler struct {
	svc *testingsvc.SharedTestRunService
}

// NewSharedTestRunHandler constructs a SharedTestRunHandler.
func NewSharedTestRunHandler(svc *testingsvc.SharedTestRunService) *SharedTestRunHandler {
	return &SharedTestRunHandler{svc: svc}
}

func (h *SharedTestRunHandler) CreateSharedTestRun(ctx context.Context, req *connect.Request[testingpb.CreateSharedTestRunRequest]) (*connect.Response[testingpb.SharedTestRun], error) {
	tenantID := &iampb.TenantId{Value: middleware.TenantFromCtx(ctx)}
	callerID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	var ttl time.Duration
	if days := req.Msg.GetExpiresInDays(); days > 0 {
		ttl = time.Duration(days) * 24 * time.Hour
	}
	result, err := h.svc.Create(ctx, tenantID, callerID, req.Msg.GetTestRunId(), ttl)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *SharedTestRunHandler) RevokeSharedTestRun(ctx context.Context, req *connect.Request[testingpb.SharedTestRunId]) (*connect.Response[testingpb.SharedTestRun], error) {
	result, err := h.svc.Revoke(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *SharedTestRunHandler) ListSharedTestRuns(ctx context.Context, req *connect.Request[iampb.TenantId]) (*connect.Response[testingpb.SharedTestRun_List], error) {
	items, err := h.svc.ListByTenant(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&testingpb.SharedTestRun_List{SharedTestRuns: items}), nil
}

func (h *SharedTestRunHandler) GetByToken(ctx context.Context, req *connect.Request[testingpb.GetSharedTestRunByTokenRequest]) (*connect.Response[testingpb.SharedTestRun], error) {
	result, err := h.svc.GetByToken(ctx, req.Msg.GetToken())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

// ─── SharedSuiteRunHandler ───────────────────────────────────────────────────

// SharedSuiteRunHandler implements SharedSuiteRunServiceHandler.
type SharedSuiteRunHandler struct {
	svc *testingsvc.SharedSuiteRunService
}

// NewSharedSuiteRunHandler constructs a SharedSuiteRunHandler.
func NewSharedSuiteRunHandler(svc *testingsvc.SharedSuiteRunService) *SharedSuiteRunHandler {
	return &SharedSuiteRunHandler{svc: svc}
}

func (h *SharedSuiteRunHandler) CreateSharedSuiteRun(ctx context.Context, req *connect.Request[testingpb.CreateSharedSuiteRunRequest]) (*connect.Response[testingpb.SharedSuiteRun], error) {
	tenantID := &iampb.TenantId{Value: middleware.TenantFromCtx(ctx)}
	callerID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	var ttl time.Duration
	if days := req.Msg.GetExpiresInDays(); days > 0 {
		ttl = time.Duration(days) * 24 * time.Hour
	}
	result, err := h.svc.Create(ctx, tenantID, callerID, req.Msg.GetTestSuiteRunId(), ttl)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *SharedSuiteRunHandler) RevokeSharedSuiteRun(ctx context.Context, req *connect.Request[testingpb.SharedSuiteRunId]) (*connect.Response[testingpb.SharedSuiteRun], error) {
	result, err := h.svc.Revoke(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *SharedSuiteRunHandler) ListSharedSuiteRuns(ctx context.Context, req *connect.Request[iampb.TenantId]) (*connect.Response[testingpb.SharedSuiteRun_List], error) {
	items, err := h.svc.ListByTenant(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&testingpb.SharedSuiteRun_List{SharedSuiteRuns: items}), nil
}

func (h *SharedSuiteRunHandler) GetByToken(ctx context.Context, req *connect.Request[testingpb.GetSharedSuiteRunByTokenRequest]) (*connect.Response[testingpb.SharedSuiteRun], error) {
	result, err := h.svc.GetByToken(ctx, req.Msg.GetToken())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}
