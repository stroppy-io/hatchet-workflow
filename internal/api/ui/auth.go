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

// AuthActions is the dependency the AuthService is built on: account
// authentication (access+refresh tokens, refresh in Valkey TTL). Login/Refresh
// are public; Me is authenticated. internal/services implements it.
type AuthActions interface {
	Login(ctx context.Context, req *uipb.LoginRequest) (*uipb.LoginResponse, error)
	RefreshTokens(ctx context.Context, req *uipb.RefreshTokenRequest) (*uipb.RefreshTokenResponse, error)
	Logout(ctx context.Context, req *uipb.LogoutRequest) (*emptypb.Empty, error)
	Me(ctx context.Context, req *emptypb.Empty) (*models.Account, error)
}

// AuthService is the gRPC handler for cloud.v1.api.ui.AuthService. Pure transport:
// trace the call and delegate to svc.
type AuthService struct {
	uipb.UnimplementedAuthServiceServer
	*tracing.Entity
	svc AuthActions
}

var _ uipb.AuthServiceServer = (*AuthService)(nil)

func NewAuthService(logger *xlog.Logger, svc AuthActions) *AuthService {
	return &AuthService{
		Entity: tracing.NewEntity(logger.AppendName("AuthService")),
		svc:    svc,
	}
}

func (s *AuthService) Login(ctx context.Context, req *uipb.LoginRequest) (*uipb.LoginResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "Login",
		func(ctx context.Context, _ trace.Span) (*uipb.LoginResponse, error) {
			return s.svc.Login(ctx, req)
		})
}

func (s *AuthService) RefreshTokens(ctx context.Context, req *uipb.RefreshTokenRequest) (*uipb.RefreshTokenResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "RefreshTokens",
		func(ctx context.Context, _ trace.Span) (*uipb.RefreshTokenResponse, error) {
			return s.svc.RefreshTokens(ctx, req)
		})
}

func (s *AuthService) Logout(ctx context.Context, req *uipb.LogoutRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "Logout",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			return s.svc.Logout(ctx, req)
		})
}

func (s *AuthService) Me(ctx context.Context, req *emptypb.Empty) (*models.Account, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "Me",
		func(ctx context.Context, _ trace.Span) (*models.Account, error) {
			return s.svc.Me(ctx, req)
		})
}
