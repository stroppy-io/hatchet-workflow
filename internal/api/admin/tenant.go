package admin

import (
	"context"

	"github.com/gopherex/xlog"
	"go.opentelemetry.io/otel/trace"

	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/admin"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// TenantAdminActions is the dependency the TenantAdminService is built on:
// platform-level (is_admin) tenant CRUD across the platform. Per-tenant
// membership lives in the tenant-scoped ui TenantService (OWNER), not here.
// internal/services implements it.
type TenantAdminActions interface {
	CreateTenant(ctx context.Context, req *adminpb.CreateTenantRequest) (*models.Tenant, error)
	UpdateTenant(ctx context.Context, req *adminpb.UpdateTenantRequest) (*models.Tenant, error)
	DeleteTenant(ctx context.Context, id *models.TenantId) (*models.Tenant, error)
}

// TenantAdminService is the gRPC handler for cloud.v1.api.admin.TenantAdminService.
// It is pure transport: trace the call and delegate to svc. Transport adaptation
// (gRPC/connect) is wired at the application level.
type TenantAdminService struct {
	adminpb.UnimplementedTenantAdminServiceServer
	*tracing.Entity
	svc TenantAdminActions
}

var _ adminpb.TenantAdminServiceServer = (*TenantAdminService)(nil)

func NewTenantAdminService(logger *xlog.Logger, svc TenantAdminActions) *TenantAdminService {
	return &TenantAdminService{
		Entity: tracing.NewEntity(logger.AppendName("TenantAdminService")),
		svc:    svc,
	}
}

func (s *TenantAdminService) CreateTenant(ctx context.Context, req *adminpb.CreateTenantRequest) (*models.Tenant, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CreateTenant",
		func(ctx context.Context, _ trace.Span) (*models.Tenant, error) {
			return s.svc.CreateTenant(ctx, req)
		})
}

func (s *TenantAdminService) UpdateTenant(ctx context.Context, req *adminpb.UpdateTenantRequest) (*models.Tenant, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "UpdateTenant",
		func(ctx context.Context, _ trace.Span) (*models.Tenant, error) {
			return s.svc.UpdateTenant(ctx, req)
		})
}

func (s *TenantAdminService) DeleteTenant(ctx context.Context, id *models.TenantId) (*models.Tenant, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "DeleteTenant",
		func(ctx context.Context, _ trace.Span) (*models.Tenant, error) {
			return s.svc.DeleteTenant(ctx, id)
		})
}
