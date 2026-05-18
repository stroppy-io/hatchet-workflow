package connect

import (
	"context"

	"connectrpc.com/connect"

	testingsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/testing"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
	"github.com/stroppy-io/stroppy-cloud/internal/transport/middleware"
)

// BaselineHandler implements testingconnect.BaselineServiceHandler.
type BaselineHandler struct{ svc *testingsvc.BaselineService }

// NewBaselineHandler builds the handler.
func NewBaselineHandler(svc *testingsvc.BaselineService) *BaselineHandler {
	return &BaselineHandler{svc: svc}
}

// SetBaseline upserts (tenant_id, name) → test_run_id.
func (h *BaselineHandler) SetBaseline(ctx context.Context, req *connect.Request[testingpb.SetBaselineRequest]) (*connect.Response[testingpb.Baseline], error) {
	callerID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	out, err := h.svc.SetBaseline(ctx, req.Msg.GetTenantId(), req.Msg.GetName(), req.Msg.GetTestRunId(), callerID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(out), nil
}

// GetBaseline fetches by (tenant_id, name).
func (h *BaselineHandler) GetBaseline(ctx context.Context, req *connect.Request[testingpb.GetBaselineRequest]) (*connect.Response[testingpb.Baseline], error) {
	out, err := h.svc.GetBaseline(ctx, req.Msg.GetTenantId(), req.Msg.GetName())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(out), nil
}

// ListBaselines lists every baseline for a tenant.
func (h *BaselineHandler) ListBaselines(ctx context.Context, req *connect.Request[iampb.TenantId]) (*connect.Response[testingpb.Baseline_List], error) {
	rows, err := h.svc.ListBaselines(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&testingpb.Baseline_List{Baselines: rows}), nil
}

// DeleteBaseline soft-deletes a row, returning the pre-delete record.
func (h *BaselineHandler) DeleteBaseline(ctx context.Context, req *connect.Request[testingpb.BaselineId]) (*connect.Response[testingpb.Baseline], error) {
	out, err := h.svc.DeleteBaseline(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(out), nil
}
