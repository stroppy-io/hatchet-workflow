package ui

import (
	"context"

	"github.com/gopherex/xlog"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"

	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/logs"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/metrics"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// RunActions is the dependency the RunService is built on: TestRun lifecycle plus
// logs, metrics, comparison and share (everything about a run). GetSharedRun is
// the only public RPC. internal/services implements it.
type RunActions interface {
	SubmitTestRun(ctx context.Context, req *uipb.SubmitTestRunRequest) (*models.TestRun, error)
	GetTestRun(ctx context.Context, req *uipb.GetTestRunRequest) (*models.TestRun, error)
	ListTestRuns(ctx context.Context, req *uipb.ListTestRunsRequest) (*models.TestRun_List, error)
	CancelTestRun(ctx context.Context, req *uipb.CancelTestRunRequest) (*models.TestRun, error)
	StreamTestRunLogs(req *uipb.StreamTestRunLogsRequest, stream grpc.ServerStreamingServer[logs.LogLine]) error
	QueryRunLogs(ctx context.Context, req *uipb.QueryRunLogsRequest) (*logs.LogPage, error)
	BuildLogLink(ctx context.Context, req *uipb.BuildLogLinkRequest) (*uipb.BuildLogLinkResponse, error)
	GetRunMetrics(ctx context.Context, req *uipb.GetRunMetricsRequest) (*metrics.RunMetrics, error)
	CompareRuns(ctx context.Context, req *uipb.CompareRunsRequest) (*metrics.Comparison, error)
	CreateShareLink(ctx context.Context, req *uipb.CreateShareLinkRequest) (*uipb.CreateShareLinkResponse, error)
	GetSharedRun(ctx context.Context, req *uipb.GetSharedRunRequest) (*uipb.GetSharedRunResponse, error)
}

// RunService is the gRPC handler for cloud.v1.api.ui.RunService. Pure transport:
// trace the call and delegate to svc.
type RunService struct {
	uipb.UnimplementedRunServiceServer
	*tracing.Entity
	svc RunActions
}

var _ uipb.RunServiceServer = (*RunService)(nil)

func NewRunService(logger *xlog.Logger, svc RunActions) *RunService {
	return &RunService{
		Entity: tracing.NewEntity(logger.AppendName("RunService")),
		svc:    svc,
	}
}

func (s *RunService) SubmitTestRun(ctx context.Context, req *uipb.SubmitTestRunRequest) (*models.TestRun, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "SubmitTestRun",
		func(ctx context.Context, _ trace.Span) (*models.TestRun, error) {
			return s.svc.SubmitTestRun(ctx, req)
		})
}

func (s *RunService) GetTestRun(ctx context.Context, req *uipb.GetTestRunRequest) (*models.TestRun, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "GetTestRun",
		func(ctx context.Context, _ trace.Span) (*models.TestRun, error) {
			return s.svc.GetTestRun(ctx, req)
		})
}

func (s *RunService) ListTestRuns(ctx context.Context, req *uipb.ListTestRunsRequest) (*models.TestRun_List, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListTestRuns",
		func(ctx context.Context, _ trace.Span) (*models.TestRun_List, error) {
			return s.svc.ListTestRuns(ctx, req)
		})
}

func (s *RunService) CancelTestRun(ctx context.Context, req *uipb.CancelTestRunRequest) (*models.TestRun, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CancelTestRun",
		func(ctx context.Context, _ trace.Span) (*models.TestRun, error) {
			return s.svc.CancelTestRun(ctx, req)
		})
}

func (s *RunService) StreamTestRunLogs(req *uipb.StreamTestRunLogsRequest, stream grpc.ServerStreamingServer[logs.LogLine]) error {
	return s.Trace(stream.Context(), "StreamTestRunLogs",
		func(_ context.Context, _ trace.Span) error {
			return s.svc.StreamTestRunLogs(req, stream)
		})
}

func (s *RunService) QueryRunLogs(ctx context.Context, req *uipb.QueryRunLogsRequest) (*logs.LogPage, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "QueryRunLogs",
		func(ctx context.Context, _ trace.Span) (*logs.LogPage, error) {
			return s.svc.QueryRunLogs(ctx, req)
		})
}

func (s *RunService) BuildLogLink(ctx context.Context, req *uipb.BuildLogLinkRequest) (*uipb.BuildLogLinkResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "BuildLogLink",
		func(ctx context.Context, _ trace.Span) (*uipb.BuildLogLinkResponse, error) {
			return s.svc.BuildLogLink(ctx, req)
		})
}

func (s *RunService) GetRunMetrics(ctx context.Context, req *uipb.GetRunMetricsRequest) (*metrics.RunMetrics, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "GetRunMetrics",
		func(ctx context.Context, _ trace.Span) (*metrics.RunMetrics, error) {
			return s.svc.GetRunMetrics(ctx, req)
		})
}

func (s *RunService) CompareRuns(ctx context.Context, req *uipb.CompareRunsRequest) (*metrics.Comparison, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CompareRuns",
		func(ctx context.Context, _ trace.Span) (*metrics.Comparison, error) {
			return s.svc.CompareRuns(ctx, req)
		})
}

func (s *RunService) CreateShareLink(ctx context.Context, req *uipb.CreateShareLinkRequest) (*uipb.CreateShareLinkResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CreateShareLink",
		func(ctx context.Context, _ trace.Span) (*uipb.CreateShareLinkResponse, error) {
			return s.svc.CreateShareLink(ctx, req)
		})
}

func (s *RunService) GetSharedRun(ctx context.Context, req *uipb.GetSharedRunRequest) (*uipb.GetSharedRunResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "GetSharedRun",
		func(ctx context.Context, _ trace.Span) (*uipb.GetSharedRunResponse, error) {
			return s.svc.GetSharedRun(ctx, req)
		})
}
