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

// TenantActions is the dependency the TenantService is built on: tenant-scoped
// membership management (OWNER) plus ListMyTenants (any authenticated account).
// Platform-level tenant CRUD lives in api/admin TenantAdminService.
// internal/services implements it.
type TenantActions interface {
	ListMyTenants(ctx context.Context, req *emptypb.Empty) (*models.Tenant_List, error)
	ListTenantMembers(ctx context.Context, req *uipb.ListTenantMembersRequest) (*uipb.ListTenantMembersResponse, error)
	AddMemberToTenant(ctx context.Context, req *uipb.AddMemberRequest) (*models.TenantMember, error)
	RemoveMemberFromTenant(ctx context.Context, req *uipb.RemoveMemberRequest) (*models.TenantMember, error)
	UpdateMemberRole(ctx context.Context, req *uipb.UpdateMemberRoleRequest) (*models.TenantMember, error)
	LookupAccountByEmail(ctx context.Context, req *uipb.LookupAccountByEmailRequest) (*models.Account, error)
}

// TenantService is the gRPC handler for cloud.v1.api.ui.TenantService. Pure
// transport: trace the call and delegate to svc.
type TenantService struct {
	uipb.UnimplementedTenantServiceServer
	*tracing.Entity
	svc TenantActions
}

var _ uipb.TenantServiceServer = (*TenantService)(nil)

func NewTenantService(logger *xlog.Logger, svc TenantActions) *TenantService {
	return &TenantService{
		Entity: tracing.NewEntity(logger.AppendName("TenantService")),
		svc:    svc,
	}
}

func (s *TenantService) ListMyTenants(ctx context.Context, req *emptypb.Empty) (*models.Tenant_List, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListMyTenants",
		func(ctx context.Context, _ trace.Span) (*models.Tenant_List, error) {
			return s.svc.ListMyTenants(ctx, req)
		})
}

func (s *TenantService) ListTenantMembers(ctx context.Context, req *uipb.ListTenantMembersRequest) (*uipb.ListTenantMembersResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListTenantMembers",
		func(ctx context.Context, _ trace.Span) (*uipb.ListTenantMembersResponse, error) {
			return s.svc.ListTenantMembers(ctx, req)
		})
}

func (s *TenantService) AddMemberToTenant(ctx context.Context, req *uipb.AddMemberRequest) (*models.TenantMember, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "AddMemberToTenant",
		func(ctx context.Context, _ trace.Span) (*models.TenantMember, error) {
			return s.svc.AddMemberToTenant(ctx, req)
		})
}

func (s *TenantService) RemoveMemberFromTenant(ctx context.Context, req *uipb.RemoveMemberRequest) (*models.TenantMember, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "RemoveMemberFromTenant",
		func(ctx context.Context, _ trace.Span) (*models.TenantMember, error) {
			return s.svc.RemoveMemberFromTenant(ctx, req)
		})
}

func (s *TenantService) UpdateMemberRole(ctx context.Context, req *uipb.UpdateMemberRoleRequest) (*models.TenantMember, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "UpdateMemberRole",
		func(ctx context.Context, _ trace.Span) (*models.TenantMember, error) {
			return s.svc.UpdateMemberRole(ctx, req)
		})
}

func (s *TenantService) LookupAccountByEmail(ctx context.Context, req *uipb.LookupAccountByEmailRequest) (*models.Account, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "LookupAccountByEmail",
		func(ctx context.Context, _ trace.Span) (*models.Account, error) {
			return s.svc.LookupAccountByEmail(ctx, req)
		})
}
