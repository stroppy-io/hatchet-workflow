package ui

import (
	"context"

	"github.com/gopherex/xlog"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// ApiTokenActions is the dependency the ApiTokenService is built on: tenant-scoped
// API token lifecycle (3rd auth principal, OWNER-only). internal/services implements it.
type ApiTokenActions interface {
	CreateApiToken(ctx context.Context, req *uipb.CreateApiTokenRequest) (*uipb.CreateApiTokenResponse, error)
	ListApiTokens(ctx context.Context, req *uipb.ListApiTokensRequest) (*models.ApiToken_List, error)
	RevokeApiToken(ctx context.Context, req *uipb.RevokeApiTokenRequest) (*emptypb.Empty, error)
}

// ApiTokenService is the gRPC handler for cloud.v1.api.ui.ApiTokenService. Pure
// transport: trace the call and delegate to svc.
type ApiTokenService struct {
	uipb.UnimplementedApiTokenServiceServer
	*tracing.Entity
	svc ApiTokenActions
}

var _ uipb.ApiTokenServiceServer = (*ApiTokenService)(nil)

func NewApiTokenService(logger *xlog.Logger, svc ApiTokenActions) *ApiTokenService {
	return &ApiTokenService{
		Entity: tracing.NewEntity(logger.AppendName("ApiTokenService")),
		svc:    svc,
	}
}

func (s *ApiTokenService) CreateApiToken(ctx context.Context, req *uipb.CreateApiTokenRequest) (*uipb.CreateApiTokenResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CreateApiToken",
		func(ctx context.Context, _ trace.Span) (*uipb.CreateApiTokenResponse, error) {
			return s.svc.CreateApiToken(ctx, req)
		})
}

func (s *ApiTokenService) ListApiTokens(ctx context.Context, req *uipb.ListApiTokensRequest) (*models.ApiToken_List, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListApiTokens",
		func(ctx context.Context, _ trace.Span) (*models.ApiToken_List, error) {
			return s.svc.ListApiTokens(ctx, req)
		})
}

func (s *ApiTokenService) RevokeApiToken(ctx context.Context, req *uipb.RevokeApiTokenRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "RevokeApiToken",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			return s.svc.RevokeApiToken(ctx, req)
		})
}
