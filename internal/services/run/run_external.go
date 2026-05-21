package run

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"

	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/logs"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/metrics"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"github.com/stroppy-io/stroppy-cloud/internal/services/svcutil"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// Planner compiles a TestPreset into an executable Dag (C12). Implemented by
// internal/domain/planner.
type Planner interface {
	Compile(preset *domain.TestPreset) (*primitive.Dag, error)
}

// DagStore persists and loads Dag aggregates. Implemented by
// internal/infrastructure/dagstore.
type DagStore interface {
	SaveDag(ctx context.Context, dag *primitive.Dag) error
	GetDag(ctx context.Context, id string) (*primitive.Dag, error)
}

// LogsClient serves run logs from the log store (VictoriaLogs, H54).
//
// TODO(run): no implementation yet — needs a VictoriaLogs query/stream client +
// LogRef link building. Reported.
type LogsClient interface {
	QueryRunLogs(ctx context.Context, req *uipb.QueryRunLogsRequest) (*logs.LogPage, error)
	StreamTestRunLogs(req *uipb.StreamTestRunLogsRequest, stream grpc.ServerStreamingServer[logs.LogLine]) error
	BuildLogLink(ctx context.Context, req *uipb.BuildLogLinkRequest) (string, error)
}

// MetricsClient serves run metric summaries (VictoriaMetrics, E19).
//
// TODO(run): no implementation yet — needs a VictoriaMetrics summary client.
// Reported.
type MetricsClient interface {
	GetRunMetrics(ctx context.Context, runID string) (*metrics.RunMetrics, error)
	CompareRuns(ctx context.Context, runA, runB string) (*metrics.Comparison, error)
}

// ShareStore mints and resolves immutable public run-share snapshots (G5).
//
// TODO(run): no implementation yet — needs a snapshot store + opaque token.
// Reported.
type ShareStore interface {
	CreateShareLink(ctx context.Context, run *models.TestRun) (token, url string, err error)
	GetSharedRun(ctx context.Context, token string) (*uipb.GetSharedRunResponse, error)
}

// StreamTestRunLogs streams unified run logs (VIEWER).
func (s *RunService) StreamTestRunLogs(req *uipb.StreamTestRunLogsRequest, stream grpc.ServerStreamingServer[logs.LogLine]) error {
	return s.Trace(stream.Context(), "StreamTestRunLogs",
		func(ctx context.Context, _ trace.Span) error {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return err
			}
			return s.logs.StreamTestRunLogs(req, stream)
		})
}

// QueryRunLogs returns a page of run logs (VIEWER).
func (s *RunService) QueryRunLogs(ctx context.Context, req *uipb.QueryRunLogsRequest) (*logs.LogPage, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "QueryRunLogs",
		func(ctx context.Context, _ trace.Span) (*logs.LogPage, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			return s.logs.QueryRunLogs(ctx, req)
		})
}

// BuildLogLink builds a deep link into the log viewer (VIEWER).
func (s *RunService) BuildLogLink(ctx context.Context, req *uipb.BuildLogLinkRequest) (*uipb.BuildLogLinkResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "BuildLogLink",
		func(ctx context.Context, _ trace.Span) (*uipb.BuildLogLinkResponse, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			url, err := s.logs.BuildLogLink(ctx, req)
			if err != nil {
				return nil, err
			}
			return &uipb.BuildLogLinkResponse{Url: url}, nil
		})
}

// GetRunMetrics returns the run's metric summary (VIEWER).
func (s *RunService) GetRunMetrics(ctx context.Context, req *uipb.GetRunMetricsRequest) (*metrics.RunMetrics, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "GetRunMetrics",
		func(ctx context.Context, _ trace.Span) (*metrics.RunMetrics, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			return s.metrics.GetRunMetrics(ctx, req.GetRunId().GetValue())
		})
}

// CompareRuns compares two runs in the same tenant (VIEWER).
func (s *RunService) CompareRuns(ctx context.Context, req *uipb.CompareRunsRequest) (*metrics.Comparison, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CompareRuns",
		func(ctx context.Context, _ trace.Span) (*metrics.Comparison, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			return s.metrics.CompareRuns(ctx, req.GetRunA().GetValue(), req.GetRunB().GetValue())
		})
}

// CreateShareLink mints a public share token for a run (VIEWER).
func (s *RunService) CreateShareLink(ctx context.Context, req *uipb.CreateShareLinkRequest) (*uipb.CreateShareLinkResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CreateShareLink",
		func(ctx context.Context, _ trace.Span) (*uipb.CreateShareLinkResponse, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			run, err := s.loadRun(ctx, req.GetTenantId().GetValue(), req.GetRunId().GetValue())
			if err != nil {
				return nil, err
			}
			token, url, err := s.share.CreateShareLink(ctx, run)
			if err != nil {
				return nil, err
			}
			return &uipb.CreateShareLinkResponse{Token: token, Url: url}, nil
		})
}

// GetSharedRun resolves a public share token — the only unauthenticated RPC.
func (s *RunService) GetSharedRun(ctx context.Context, req *uipb.GetSharedRunRequest) (*uipb.GetSharedRunResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "GetSharedRun",
		func(ctx context.Context, _ trace.Span) (*uipb.GetSharedRunResponse, error) {
			return s.share.GetSharedRun(ctx, req.GetToken())
		})
}
