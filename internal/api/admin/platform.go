package admin

import (
	"context"

	"github.com/gopherex/xlog"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/admin"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// PlatformAdminActions is the dependency the PlatformAdminService is built on:
// the GLOBAL (singleton) control-plane settings, root-admin only. The single
// server_addr handed to every agent lives here. internal/services implements it.
type PlatformAdminActions interface {
	GetPlatformSettings(ctx context.Context, _ *emptypb.Empty) (*models.PlatformSettings, error)
	SetPlatformSettings(ctx context.Context, req *adminpb.SetPlatformSettingsRequest) (*models.PlatformSettings, error)
}

// PlatformAdminService is the gRPC handler for the admin PlatformAdminService.
// Pure transport: trace + delegate. Transport adaptation is wired at the app level.
type PlatformAdminService struct {
	adminpb.UnimplementedPlatformAdminServiceServer
	*tracing.Entity
	svc PlatformAdminActions
}

var _ adminpb.PlatformAdminServiceServer = (*PlatformAdminService)(nil)

func NewPlatformAdminService(logger *xlog.Logger, svc PlatformAdminActions) *PlatformAdminService {
	return &PlatformAdminService{
		Entity: tracing.NewEntity(logger.AppendName("PlatformAdminService")),
		svc:    svc,
	}
}

func (s *PlatformAdminService) GetPlatformSettings(ctx context.Context, req *emptypb.Empty) (*models.PlatformSettings, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "GetPlatformSettings",
		func(ctx context.Context, _ trace.Span) (*models.PlatformSettings, error) {
			return s.svc.GetPlatformSettings(ctx, req)
		})
}

func (s *PlatformAdminService) SetPlatformSettings(ctx context.Context, req *adminpb.SetPlatformSettingsRequest) (*models.PlatformSettings, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "SetPlatformSettings",
		func(ctx context.Context, _ trace.Span) (*models.PlatformSettings, error) {
			return s.svc.SetPlatformSettings(ctx, req)
		})
}
