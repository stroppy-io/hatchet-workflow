package ui

import (
	"context"

	"github.com/gopherex/xlog"
	"go.opentelemetry.io/otel/trace"

	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// SuiteActions is the dependency the SuiteService is built on: Suite definition
// and SuiteRun orchestration (a set of TestRuns). internal/services implements it.
type SuiteActions interface {
	CreateSuite(ctx context.Context, req *uipb.CreateSuiteRequest) (*models.Suite, error)
	GetSuite(ctx context.Context, req *uipb.GetSuiteRequest) (*models.Suite, error)
	UpdateSuite(ctx context.Context, req *uipb.UpdateSuiteRequest) (*models.Suite, error)
	ListSuites(ctx context.Context, req *uipb.ListSuitesRequest) (*uipb.ListSuitesResponse, error)
	LaunchSuiteRun(ctx context.Context, req *uipb.LaunchSuiteRunRequest) (*models.SuiteRun, error)
	GetSuiteRun(ctx context.Context, req *uipb.GetSuiteRunRequest) (*models.SuiteRun, error)
	ListSuiteRuns(ctx context.Context, req *uipb.ListSuiteRunsRequest) (*uipb.ListSuiteRunsResponse, error)
	CancelSuiteRun(ctx context.Context, req *uipb.CancelSuiteRunRequest) (*models.SuiteRun, error)
}

// SuiteService is the gRPC handler for cloud.v1.api.ui.SuiteService. Pure
// transport: trace the call and delegate to svc.
type SuiteService struct {
	uipb.UnimplementedSuiteServiceServer
	*tracing.Entity
	svc SuiteActions
}

var _ uipb.SuiteServiceServer = (*SuiteService)(nil)

func NewSuiteService(logger *xlog.Logger, svc SuiteActions) *SuiteService {
	return &SuiteService{
		Entity: tracing.NewEntity(logger.AppendName("SuiteService")),
		svc:    svc,
	}
}

func (s *SuiteService) CreateSuite(ctx context.Context, req *uipb.CreateSuiteRequest) (*models.Suite, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CreateSuite",
		func(ctx context.Context, _ trace.Span) (*models.Suite, error) {
			return s.svc.CreateSuite(ctx, req)
		})
}

func (s *SuiteService) GetSuite(ctx context.Context, req *uipb.GetSuiteRequest) (*models.Suite, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "GetSuite",
		func(ctx context.Context, _ trace.Span) (*models.Suite, error) {
			return s.svc.GetSuite(ctx, req)
		})
}

func (s *SuiteService) UpdateSuite(ctx context.Context, req *uipb.UpdateSuiteRequest) (*models.Suite, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "UpdateSuite",
		func(ctx context.Context, _ trace.Span) (*models.Suite, error) {
			return s.svc.UpdateSuite(ctx, req)
		})
}

func (s *SuiteService) ListSuites(ctx context.Context, req *uipb.ListSuitesRequest) (*uipb.ListSuitesResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListSuites",
		func(ctx context.Context, _ trace.Span) (*uipb.ListSuitesResponse, error) {
			return s.svc.ListSuites(ctx, req)
		})
}

func (s *SuiteService) LaunchSuiteRun(ctx context.Context, req *uipb.LaunchSuiteRunRequest) (*models.SuiteRun, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "LaunchSuiteRun",
		func(ctx context.Context, _ trace.Span) (*models.SuiteRun, error) {
			return s.svc.LaunchSuiteRun(ctx, req)
		})
}

func (s *SuiteService) GetSuiteRun(ctx context.Context, req *uipb.GetSuiteRunRequest) (*models.SuiteRun, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "GetSuiteRun",
		func(ctx context.Context, _ trace.Span) (*models.SuiteRun, error) {
			return s.svc.GetSuiteRun(ctx, req)
		})
}

func (s *SuiteService) ListSuiteRuns(ctx context.Context, req *uipb.ListSuiteRunsRequest) (*uipb.ListSuiteRunsResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListSuiteRuns",
		func(ctx context.Context, _ trace.Span) (*uipb.ListSuiteRunsResponse, error) {
			return s.svc.ListSuiteRuns(ctx, req)
		})
}

func (s *SuiteService) CancelSuiteRun(ctx context.Context, req *uipb.CancelSuiteRunRequest) (*models.SuiteRun, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CancelSuiteRun",
		func(ctx context.Context, _ trace.Span) (*models.SuiteRun, error) {
			return s.svc.CancelSuiteRun(ctx, req)
		})
}
