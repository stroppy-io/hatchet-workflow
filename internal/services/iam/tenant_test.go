package iam

// tenant_test.go: tests for tenant operations in service.go:
// CreateTenant, GetTenant, ListMyTenants, UpdateTenant, DeleteTenant,
// TransferTenantOwnership, LeaveTenant.

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// -------- CreateTenant --------

func TestCreateTenant(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	gates := NewMockPlatformGates(ctrl)
	tenants := NewMockTenantRepo(ctrl)
	roles := NewMockRoleRepo(ctrl)
	memberships := NewMockMembershipRepo(ctrl)

	svc := NewIamService(IamDeps{
		Tx:    &utils.MockTrm{},
		Authn: authn, Gates: gates, Tenants: tenants, Roles: roles, Memberships: memberships,
	})
	ctx := context.Background()
	req := &api.CreateTenantRequest{Name: "New Tenant", Slug: "new-tenant"}

	t.Run("SuccessAdmin", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1", IsAdmin: true}, nil)
		tenants.EXPECT().GetBySlug(ctx, req.Slug).Return(nil, derrors.ErrNotFound)
		tenants.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		roles.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		memberships.EXPECT().Create(ctx, gomock.Any()).Return(nil)

		resp, err := svc.CreateTenant(ctx, req)
		if err != nil {
			t.Fatalf("CreateTenant failed: %v", err)
		}
		if resp.Tenant.Slug != req.Slug {
			t.Errorf("expected slug %s, got %s", req.Slug, resp.Tenant.Slug)
		}
	})

	t.Run("SuccessMember", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1", IsAdmin: false}, nil)
		gates.EXPECT().MemberTenantCreationAllowed(ctx).Return(true, nil)
		tenants.EXPECT().GetBySlug(ctx, req.Slug).Return(nil, derrors.ErrNotFound)
		tenants.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		roles.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		memberships.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		_, err := svc.CreateTenant(ctx, req)
		if err != nil {
			t.Fatalf("CreateTenant member failed: %v", err)
		}
	})

	t.Run("MemberDisabled", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1", IsAdmin: false}, nil)
		gates.EXPECT().MemberTenantCreationAllowed(ctx).Return(false, nil)
		_, err := svc.CreateTenant(ctx, req)
		if status.Code(err) != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied, got %v", err)
		}
	})

	t.Run("GatesError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1", IsAdmin: false}, nil)
		gates.EXPECT().MemberTenantCreationAllowed(ctx).Return(false, errors.New("db err"))
		_, err := svc.CreateTenant(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("SlugAlreadyExists", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1", IsAdmin: true}, nil)
		tenants.EXPECT().GetBySlug(ctx, req.Slug).Return(&iampb.Tenant{}, nil)
		_, err := svc.CreateTenant(ctx, req)
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists, got %v", err)
		}
	})

	t.Run("SlugCheckError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1", IsAdmin: true}, nil)
		tenants.EXPECT().GetBySlug(ctx, req.Slug).Return(nil, errors.New("db err"))
		_, err := svc.CreateTenant(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no claims"))
		_, err := svc.CreateTenant(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- GetTenant --------

func TestGetTenant(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tenants := NewMockTenantRepo(ctrl)
	svc := NewIamService(IamDeps{Tx: &utils.MockTrm{}, Tenants: tenants})
	ctx := context.Background()

	t.Run("SuccessById", func(t *testing.T) {
		req := &api.GetTenantRequest{Ref: &api.GetTenantRequest_Id{Id: "t1"}}
		tenants.EXPECT().Get(ctx, "t1").Return(&iampb.Tenant{Id: "t1", Name: "Tenant 1"}, nil)
		resp, err := svc.GetTenant(ctx, req)
		if err != nil {
			t.Fatalf("GetTenant: %v", err)
		}
		if resp.Tenant.Id != "t1" {
			t.Errorf("expected t1, got %s", resp.Tenant.Id)
		}
	})

	t.Run("SuccessBySlug", func(t *testing.T) {
		req := &api.GetTenantRequest{Ref: &api.GetTenantRequest_Slug{Slug: "my-tenant"}}
		tenants.EXPECT().GetBySlug(ctx, "my-tenant").Return(&iampb.Tenant{Id: "t2", Slug: "my-tenant"}, nil)
		resp, err := svc.GetTenant(ctx, req)
		if err != nil {
			t.Fatalf("GetTenant by slug: %v", err)
		}
		if resp.Tenant.Id != "t2" {
			t.Errorf("expected t2, got %s", resp.Tenant.Id)
		}
	})

	t.Run("NoIdOrSlug", func(t *testing.T) {
		req := &api.GetTenantRequest{}
		_, err := svc.GetTenant(ctx, req)
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		req := &api.GetTenantRequest{Ref: &api.GetTenantRequest_Id{Id: "t1"}}
		tenants.EXPECT().Get(ctx, "t1").Return(nil, derrors.ErrNotFound)
		_, err := svc.GetTenant(ctx, req)
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})
}

// -------- ListMyTenants --------

func TestListMyTenants(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	tenants := NewMockTenantRepo(ctrl)
	svc := NewIamService(IamDeps{Tx: &utils.MockTrm{}, Authn: authn, Tenants: tenants})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		tenants.EXPECT().ListByMember(ctx, "a1").Return([]*iampb.Tenant{
			{Id: "t1"}, {Id: "t2"},
		}, nil)
		resp, err := svc.ListMyTenants(ctx, &api.ListMyTenantsRequest{})
		if err != nil {
			t.Fatalf("ListMyTenants: %v", err)
		}
		if len(resp.Tenants) != 2 {
			t.Errorf("expected 2 tenants, got %d", len(resp.Tenants))
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no claims"))
		_, err := svc.ListMyTenants(ctx, &api.ListMyTenantsRequest{})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ListError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		tenants.EXPECT().ListByMember(ctx, "a1").Return(nil, errors.New("db err"))
		_, err := svc.ListMyTenants(ctx, &api.ListMyTenantsRequest{})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- UpdateTenant --------

func TestUpdateTenant(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tenants := NewMockTenantRepo(ctrl)
	svc := NewIamService(IamDeps{Tx: &utils.MockTrm{}, Tenants: tenants})
	ctx := context.Background()

	t.Run("Success_Name", func(t *testing.T) {
		name := "Updated Tenant"
		req := &api.UpdateTenantRequest{Id: "t1", Name: &name}
		tenants.EXPECT().Get(ctx, "t1").Return(&iampb.Tenant{Id: "t1", Name: "Old"}, nil)
		tenants.EXPECT().Update(ctx, gomock.Any()).Return(nil)
		resp, err := svc.UpdateTenant(ctx, req)
		if err != nil {
			t.Fatalf("UpdateTenant: %v", err)
		}
		if resp.Tenant.Name != "Updated Tenant" {
			t.Errorf("expected Updated Tenant, got %s", resp.Tenant.Name)
		}
	})

	t.Run("Success_Slug", func(t *testing.T) {
		slug := "new-slug"
		req := &api.UpdateTenantRequest{Id: "t1", Slug: &slug}
		tenants.EXPECT().Get(ctx, "t1").Return(&iampb.Tenant{Id: "t1", Slug: "old-slug"}, nil)
		tenants.EXPECT().GetBySlug(ctx, "new-slug").Return(nil, derrors.ErrNotFound)
		tenants.EXPECT().Update(ctx, gomock.Any()).Return(nil)
		resp, err := svc.UpdateTenant(ctx, req)
		if err != nil {
			t.Fatalf("UpdateTenant slug: %v", err)
		}
		if resp.Tenant.Slug != "new-slug" {
			t.Errorf("expected new-slug, got %s", resp.Tenant.Slug)
		}
	})

	t.Run("SlugInUse", func(t *testing.T) {
		slug := "taken"
		req := &api.UpdateTenantRequest{Id: "t1", Slug: &slug}
		tenants.EXPECT().Get(ctx, "t1").Return(&iampb.Tenant{Id: "t1", Slug: "old"}, nil)
		tenants.EXPECT().GetBySlug(ctx, "taken").Return(&iampb.Tenant{Id: "t2"}, nil)
		_, err := svc.UpdateTenant(ctx, req)
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists, got %v", err)
		}
	})

	t.Run("SlugCheckError", func(t *testing.T) {
		slug := "new"
		req := &api.UpdateTenantRequest{Id: "t1", Slug: &slug}
		tenants.EXPECT().Get(ctx, "t1").Return(&iampb.Tenant{Id: "t1", Slug: "old"}, nil)
		tenants.EXPECT().GetBySlug(ctx, "new").Return(nil, errors.New("db err"))
		_, err := svc.UpdateTenant(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("GetError", func(t *testing.T) {
		tenants.EXPECT().Get(ctx, "t1").Return(nil, derrors.ErrNotFound)
		_, err := svc.UpdateTenant(ctx, &api.UpdateTenantRequest{Id: "t1"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("UpdateError", func(t *testing.T) {
		tenants.EXPECT().Get(ctx, "t1").Return(&iampb.Tenant{Id: "t1"}, nil)
		tenants.EXPECT().Update(ctx, gomock.Any()).Return(errors.New("db err"))
		_, err := svc.UpdateTenant(ctx, &api.UpdateTenantRequest{Id: "t1"})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- DeleteTenant --------

func TestDeleteTenant(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tenants := NewMockTenantRepo(ctrl)
	memberships := NewMockMembershipRepo(ctrl)
	roles := NewMockRoleRepo(ctrl)
	svc := NewIamService(IamDeps{
		Tx:      &utils.MockTrm{},
		Tenants: tenants, Memberships: memberships, Roles: roles,
	})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		memberships.EXPECT().DeleteByTenant(ctx, "t1").Return(nil)
		roles.EXPECT().DeleteByTenant(ctx, "t1").Return(nil)
		tenants.EXPECT().Delete(ctx, "t1").Return(nil)
		_, err := svc.DeleteTenant(ctx, &api.DeleteTenantRequest{Id: "t1"})
		if err != nil {
			t.Fatalf("DeleteTenant: %v", err)
		}
	})

	t.Run("DeleteMembershipsError", func(t *testing.T) {
		memberships.EXPECT().DeleteByTenant(ctx, "t1").Return(errors.New("db err"))
		_, err := svc.DeleteTenant(ctx, &api.DeleteTenantRequest{Id: "t1"})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("DeleteRolesError", func(t *testing.T) {
		memberships.EXPECT().DeleteByTenant(ctx, "t1").Return(nil)
		roles.EXPECT().DeleteByTenant(ctx, "t1").Return(errors.New("db err"))
		_, err := svc.DeleteTenant(ctx, &api.DeleteTenantRequest{Id: "t1"})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- TransferTenantOwnership --------

func TestTransferTenantOwnership(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tenants := NewMockTenantRepo(ctrl)
	memberships := NewMockMembershipRepo(ctrl)
	svc := NewIamService(IamDeps{
		Tx:      &utils.MockTrm{},
		Tenants: tenants, Memberships: memberships,
	})
	ctx := context.Background()
	req := &api.TransferTenantOwnershipRequest{TenantId: "t1", NewOwnerAccountId: "a2"}

	t.Run("Success", func(t *testing.T) {
		tenants.EXPECT().Get(ctx, "t1").Return(&iampb.Tenant{Id: "t1", OwnerAccountId: "a1"}, nil)
		memberships.EXPECT().GetByAccountTenant(ctx, "a2", "t1").Return(&iampb.Membership{}, nil)
		tenants.EXPECT().Update(ctx, gomock.Any()).Return(nil)
		resp, err := svc.TransferTenantOwnership(ctx, req)
		if err != nil {
			t.Fatalf("TransferTenantOwnership: %v", err)
		}
		if resp.Tenant.OwnerAccountId != "a2" {
			t.Errorf("expected a2, got %s", resp.Tenant.OwnerAccountId)
		}
	})

	t.Run("SameOwner_NoOp", func(t *testing.T) {
		tenants.EXPECT().Get(ctx, "t1").Return(&iampb.Tenant{Id: "t1", OwnerAccountId: "a2"}, nil)
		resp, err := svc.TransferTenantOwnership(ctx, req)
		if err != nil {
			t.Fatalf("TransferTenantOwnership same owner: %v", err)
		}
		if resp.Tenant.OwnerAccountId != "a2" {
			t.Errorf("expected a2, got %s", resp.Tenant.OwnerAccountId)
		}
	})

	t.Run("NewOwnerNotMember", func(t *testing.T) {
		tenants.EXPECT().Get(ctx, "t1").Return(&iampb.Tenant{Id: "t1", OwnerAccountId: "a1"}, nil)
		memberships.EXPECT().GetByAccountTenant(ctx, "a2", "t1").Return(nil, derrors.ErrNotFound)
		_, err := svc.TransferTenantOwnership(ctx, req)
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition, got %v", err)
		}
	})

	t.Run("MembershipCheckError", func(t *testing.T) {
		tenants.EXPECT().Get(ctx, "t1").Return(&iampb.Tenant{Id: "t1", OwnerAccountId: "a1"}, nil)
		memberships.EXPECT().GetByAccountTenant(ctx, "a2", "t1").Return(nil, errors.New("db err"))
		_, err := svc.TransferTenantOwnership(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("TenantGetError", func(t *testing.T) {
		tenants.EXPECT().Get(ctx, "t1").Return(nil, errors.New("db err"))
		_, err := svc.TransferTenantOwnership(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- LeaveTenant --------

func TestLeaveTenant(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	tenants := NewMockTenantRepo(ctrl)
	memberships := NewMockMembershipRepo(ctrl)
	svc := NewIamService(IamDeps{
		Tx:    &utils.MockTrm{},
		Authn: authn, Tenants: tenants, Memberships: memberships,
	})
	ctx := context.Background()
	req := &api.LeaveTenantRequest{TenantId: "t1"}

	t.Run("Success", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		tenants.EXPECT().Get(ctx, "t1").Return(&iampb.Tenant{Id: "t1", OwnerAccountId: "owner"}, nil)
		memberships.EXPECT().GetByAccountTenant(ctx, "a1", "t1").Return(&iampb.Membership{Id: "m1"}, nil)
		memberships.EXPECT().Delete(ctx, "m1").Return(nil)
		_, err := svc.LeaveTenant(ctx, req)
		if err != nil {
			t.Fatalf("LeaveTenant: %v", err)
		}
	})

	t.Run("OwnerCannotLeave", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		tenants.EXPECT().Get(ctx, "t1").Return(&iampb.Tenant{Id: "t1", OwnerAccountId: "a1"}, nil)
		_, err := svc.LeaveTenant(ctx, req)
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition, got %v", err)
		}
	})

	t.Run("TenantNotFound_Success", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		tenants.EXPECT().Get(ctx, "t1").Return(nil, derrors.ErrNotFound)
		_, err := svc.LeaveTenant(ctx, req)
		if err != nil {
			t.Fatalf("LeaveTenant tenant not found should succeed: %v", err)
		}
	})

	t.Run("TenantGetError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		tenants.EXPECT().Get(ctx, "t1").Return(nil, errors.New("db err"))
		_, err := svc.LeaveTenant(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("MembershipNotFound_Success", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		tenants.EXPECT().Get(ctx, "t1").Return(&iampb.Tenant{Id: "t1", OwnerAccountId: "owner"}, nil)
		memberships.EXPECT().GetByAccountTenant(ctx, "a1", "t1").Return(nil, derrors.ErrNotFound)
		_, err := svc.LeaveTenant(ctx, req)
		if err != nil {
			t.Fatalf("LeaveTenant no membership should succeed: %v", err)
		}
	})

	t.Run("MembershipGetError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		tenants.EXPECT().Get(ctx, "t1").Return(&iampb.Tenant{Id: "t1", OwnerAccountId: "owner"}, nil)
		memberships.EXPECT().GetByAccountTenant(ctx, "a1", "t1").Return(nil, errors.New("db err"))
		_, err := svc.LeaveTenant(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no claims"))
		_, err := svc.LeaveTenant(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})
}
