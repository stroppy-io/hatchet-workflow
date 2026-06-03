package test_run_overview

import (
	"context"

	"connectrpc.com/connect"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/apiconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
)

// ConnectHandler adapts TestRunOverviewService to the Connect generated
// handler interface. The native gRPC and browser Connect paths share the same
// service instance and therefore the same auth/tenant gates and dependency
// implementations.
type ConnectHandler struct {
	svc *TestRunOverviewService
}

var _ apiconnect.TestRunOverviewServiceHandler = (*ConnectHandler)(nil)

func NewConnectHandler(svc *TestRunOverviewService) *ConnectHandler {
	return &ConnectHandler{svc: svc}
}

func (h *ConnectHandler) GetTestRunOverview(ctx context.Context, req *api.GetTestRunOverviewRequest) (*api.GetTestRunOverviewResponse, error) {
	return h.svc.GetTestRunOverview(ctx, req)
}

func (h *ConnectHandler) StreamTestRunOverview(ctx context.Context, req *api.StreamTestRunOverviewRequest, stream *connect.ServerStream[api.TestRunOverviewSnapshot]) error {
	return h.svc.streamOverview(ctx, req.GetTenantId(), req.GetRunId(), stream.Send)
}

func (h *ConnectHandler) QueryLogs(ctx context.Context, req *api.QueryLogsRequest) (*api.QueryLogsResponse, error) {
	return h.svc.QueryLogs(ctx, req)
}

func (h *ConnectHandler) StreamLogs(ctx context.Context, req *api.StreamLogsRequest, stream *connect.ServerStream[monitor.LogLine]) error {
	return h.svc.streamLogs(ctx, req.GetTenantId(), req.GetRunId(), req.GetFilter(), req.GetFrom(), stream.Send)
}

func (h *ConnectHandler) ResolveLogRef(ctx context.Context, req *api.ResolveLogRefRequest) (*api.ResolveLogRefResponse, error) {
	return h.svc.ResolveLogRef(ctx, req)
}

func (h *ConnectHandler) GetRunMetrics(ctx context.Context, req *api.GetRunMetricsRequest) (*api.GetRunMetricsResponse, error) {
	return h.svc.GetRunMetrics(ctx, req)
}
