package run

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"

	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/logs"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/metrics"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"github.com/stroppy-io/stroppy-cloud/internal/services/svcutil"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// ProviderResolver resolves a tenant's run-scoped deployment.Deployment (provider
// settings + per-machine network allocation + agent bootstrap) for a TestPreset.
// Implemented by internal/services/deploy. The resolved deployment is baked into the
// dag (dag.BuildTestDag) at submit time.
type ProviderResolver interface {
	Resolve(ctx context.Context, tenantID string, preset *domain.TestPreset) (*deployment.Deployment, error)
}

// DagStore persists and loads Dag aggregates. Implemented by
// internal/infrastructure/dagstore.
type DagStore interface {
	SaveDag(ctx context.Context, dag *primitive.Dag) error
	GetDag(ctx context.Context, id string) (*primitive.Dag, error)
}

// LogsClient serves run logs from the log store (VictoriaLogs, H54), addressed by
// dag id (RunService resolves run_id -> dag_id). Implemented by internal/services/logs.
type LogsClient interface {
	QueryRunLogs(ctx context.Context, dagID string, req *uipb.QueryRunLogsRequest) (*logs.LogPage, error)
	StreamRunLogs(ctx context.Context, dagID, nodeExecutionID, componentID string, stream grpc.ServerStreamingServer[logs.LogLine]) error
	BuildLogLink(ctx context.Context, dagID string, req *uipb.BuildLogLinkRequest) (string, error)
}

// MetricsClient serves run metric summaries (VictoriaMetrics, E19). Implemented by
// services/metrics over the VictoriaMetrics instant-query client.
type MetricsClient interface {
	GetRunMetrics(ctx context.Context, runID string) (*metrics.RunMetrics, error)
	CompareRuns(ctx context.Context, runIDs []string, threshold float64) (*metrics.Comparison, error)
}

// ShareStore mints and resolves immutable public run-share snapshots (G5).
// Implemented by services/share (opaque token + metric snapshot stored in Valkey).
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
			run, err := s.loadRun(ctx, req.GetTenantId().GetValue(), req.GetId().GetValue())
			if err != nil {
				return err
			}
			return s.logs.StreamRunLogs(ctx, run.GetDag().GetValue(), req.GetNodeExecutionId(), req.GetComponentId(), stream)
		})
}

// QueryRunLogs returns a page of run logs (VIEWER).
func (s *RunService) QueryRunLogs(ctx context.Context, req *uipb.QueryRunLogsRequest) (*logs.LogPage, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "QueryRunLogs",
		func(ctx context.Context, _ trace.Span) (*logs.LogPage, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			run, err := s.loadRun(ctx, req.GetTenantId().GetValue(), req.GetRunId().GetValue())
			if err != nil {
				return nil, err
			}
			return s.logs.QueryRunLogs(ctx, run.GetDag().GetValue(), req)
		})
}

// BuildLogLink builds a deep link into the log viewer (VIEWER).
func (s *RunService) BuildLogLink(ctx context.Context, req *uipb.BuildLogLinkRequest) (*uipb.BuildLogLinkResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "BuildLogLink",
		func(ctx context.Context, _ trace.Span) (*uipb.BuildLogLinkResponse, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			run, err := s.loadRun(ctx, req.GetTenantId().GetValue(), req.GetRunId().GetValue())
			if err != nil {
				return nil, err
			}
			url, err := s.logs.BuildLogLink(ctx, run.GetDag().GetValue(), req)
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
			// Metrics are tagged run_id=<dag id> (stroppy OTLP resource attr / vmagent
			// label), so resolve run -> dag and query by the dag id (mirrors logs).
			run, err := s.loadRun(ctx, req.GetTenantId().GetValue(), req.GetRunId().GetValue())
			if err != nil {
				return nil, err
			}
			return s.metrics.GetRunMetrics(ctx, run.GetDag().GetValue())
		})
}

// CompareRuns compares N runs (baseline = run_ids[0]) in the same tenant (VIEWER).
func (s *RunService) CompareRuns(ctx context.Context, req *uipb.CompareRunsRequest) (*metrics.Comparison, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CompareRuns",
		func(ctx context.Context, _ trace.Span) (*metrics.Comparison, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			runIDs := make([]string, 0, len(req.GetRunIds()))
			for _, id := range req.GetRunIds() {
				runIDs = append(runIDs, id.GetValue())
			}
			return s.metrics.CompareRuns(ctx, runIDs, req.GetThreshold())
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
