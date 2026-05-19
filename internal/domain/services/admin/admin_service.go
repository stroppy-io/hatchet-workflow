package admin

import (
	"context"
	"errors"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/admin"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/transport/middleware"
)

// IamAdminPort is the subset of iam.Service methods consumed by AdminService.
type IamAdminPort interface {
	// Tenants
	ListAllTenants(ctx context.Context) ([]*iampb.Tenant, error)
	CreateTenant(ctx context.Context, tenant *iampb.Tenant, ownerID *iampb.UserId) (*iampb.Tenant, error)
	DeleteTenantHard(ctx context.Context, id *iampb.TenantId) (*iampb.Tenant, error)

	// Users
	ListUsers(ctx context.Context) ([]*iampb.User, error)
	CreateUser(ctx context.Context, user *iampb.User, password string) (*iampb.User, error)
	DeleteUser(ctx context.Context, id *iampb.UserId) (*iampb.User, error)
	ResetUserPassword(ctx context.Context, id *iampb.UserId, newPassword string) (*iampb.User, error)
	GetUserByID(ctx context.Context, id *iampb.UserId) (*iampb.User, error)
	AddMember(ctx context.Context, userID *iampb.UserId, tenantID *iampb.TenantId, role iampb.TenantRole) (*iampb.TenantMember, error)
}

// ErrNoCaller is returned when a Create-style RPC needs the calling user's ID
// (e.g. to assign as tenant OWNER for a placeholder owner_user_id) but the
// request context carries no authenticated user.
var ErrNoCaller = errors.New("admin: no authenticated caller in context")

// AdminService implements cross-tenant admin operations on tenants and users.
type AdminService struct {
	*tracing.Entity
	iam IamAdminPort
}

// NewAdminService constructs an AdminService.
func NewAdminService(iam IamAdminPort) *AdminService {
	return &AdminService{
		Entity: tracing.NewEntity("admin.AdminService"),
		iam:    iam,
	}
}

// ListAllTenants returns all tenants (platform-admin view).
func (s *AdminService) ListAllTenants(ctx context.Context, _ *emptypb.Empty) (*iampb.Tenant_List, error) {
	tenants, err := s.iam.ListAllTenants(ctx)
	if err != nil {
		return nil, err
	}
	return &iampb.Tenant_List{Tenants: tenants}, nil
}

// CreateTenant creates a tenant. If OwnerUserId is the 26-char zero placeholder
// (or empty), the calling admin is recorded as the OWNER. Tenant.Id is always
// regenerated server-side regardless of the client value.
func (s *AdminService) CreateTenant(ctx context.Context, req *adminpb.AdminCreateTenantRequest) (*iampb.Tenant, error) {
	owner := req.GetOwnerUserId()
	if owner == nil || owner.GetValue() == "" || ids.IsPlaceholder(owner.GetValue()) {
		callerID := middleware.UserFromCtx(ctx)
		if callerID == "" {
			return nil, ErrNoCaller
		}
		owner = &iampb.UserId{Value: callerID}
	}
	tenant, err := s.iam.CreateTenant(ctx, req.GetTenant(), owner)
	if err != nil {
		return nil, err
	}
	return tenant, nil
}

// DeleteTenantHard hard-deletes a tenant (DB cascade removes members).
func (s *AdminService) DeleteTenantHard(ctx context.Context, id *iampb.TenantId) (*iampb.Tenant, error) {
	return s.iam.DeleteTenantHard(ctx, id)
}

// ListAllUsers returns all users across all tenants.
func (s *AdminService) ListAllUsers(ctx context.Context, _ *emptypb.Empty) (*iampb.User_List, error) {
	users, err := s.iam.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	return &iampb.User_List{Users: users}, nil
}

// CreateUser creates a new user. The User.TenantMembers list in the request
// proto is not processed here; membership is handled separately. The user proto
// must include Email and Nickname.
func (s *AdminService) CreateUser(ctx context.Context, req *adminpb.AdminCreateUserRequest) (*iampb.User, error) {
	return s.iam.CreateUser(ctx, req.GetUser(), req.GetPassword())
}

// DeleteUser hard-deletes a user by ID.
func (s *AdminService) DeleteUser(ctx context.Context, id *iampb.UserId) (*iampb.User, error) {
	return s.iam.DeleteUser(ctx, id)
}

// ResetUserPassword resets the user's password and revokes all active sessions.
func (s *AdminService) ResetUserPassword(ctx context.Context, req *adminpb.AdminResetPasswordRequest) (*iampb.User, error) {
	return s.iam.ResetUserPassword(ctx, req.GetUserId(), req.GetNewPassword())
}
