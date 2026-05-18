package connect

import (
	"context"

	"connectrpc.com/connect"

	testingsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/testing"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
	"github.com/stroppy-io/stroppy-cloud/internal/transport/middleware"
)

// ComparisonHandler implements ComparisonServiceHandler.
type ComparisonHandler struct {
	svc *testingsvc.ComparisonService
}

// NewComparisonHandler constructs a ComparisonHandler.
func NewComparisonHandler(svc *testingsvc.ComparisonService) *ComparisonHandler {
	return &ComparisonHandler{svc: svc}
}

func (h *ComparisonHandler) CompareRuns(ctx context.Context, req *connect.Request[testingpb.CompareRunsRequest]) (*connect.Response[testingpb.CompareRunsResponse], error) {
	tenantID := &iampb.TenantId{Value: middleware.TenantFromCtx(ctx)}
	result, err := h.svc.CompareRuns(ctx, tenantID, req.Msg.GetA(), req.Msg.GetB(), req.Msg.GetMetricNames())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *ComparisonHandler) CrossCompareBatch(ctx context.Context, req *connect.Request[testingpb.CrossCompareBatchRequest]) (*connect.Response[testingpb.CrossCompareBatchResponse], error) {
	tenantID := &iampb.TenantId{Value: middleware.TenantFromCtx(ctx)}
	out, err := h.svc.CrossCompareBatch(ctx, tenantID, req.Msg.GetTestSuiteRunId(), req.Msg.GetBaselineRunId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(out), nil
}
