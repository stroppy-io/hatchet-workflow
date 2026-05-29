package iam

// membership_test.go: tests for membership operations in service.go:
// CreateMembership, GetMembership, ListMemberships, UpdateMembership, DeleteMembership,
// GetMyPermissions, ListPermissions.

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

// -------- CreateMembership --------

func TestCreateMembership(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	roles := NewMockRoleRepo(ctrl)
	memberships := NewMockMembershipRepo(ctrl)
	svc := NewIamService(IamDeps{
		Tx:    &utils.MockTrm{},
		Roles: roles, Memberships: memberships,
	})
	ctx := context.Background()
	req := &api.CreateMembershipRequest{AccountId: "a1", TenantId: "t1", RoleIds: []string{"r1"}}

	t.Run("Success", func(t *testing.T) {
		roles.EXPECT().GetMany(ctx, []string{"r1"}).Return([]*iampb.Role{
			{Id: "r1", Scope: iampb.Scope_SCOPE_TENANT, TenantId: "t1"},
		}, nil)
		memberships.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		resp, err := svc.CreateMembership(ctx, req)
		if err != nil {
			t.Fatalf("CreateMembership: %v", err)
		}
		if resp.Membership.AccountId != "a1" {
			t.Errorf("expected a1, got %s", resp.Membership.AccountId)
		}
	})

	t.Run("RoleNotFound", func(t *testing.T) {
		roles.EXPECT().GetMany(ctx, []string{"r1"}).Return([]*iampb.Role{}, nil)
		_, err := svc.CreateMembership(ctx, req)
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("ForeignRole", func(t *testing.T) {
		roles.EXPECT().GetMany(ctx, []string{"r1"}).Return([]*iampb.Role{
			{Id: "r1", Scope: iampb.Scope_SCOPE_TENANT, TenantId: "other"},
		}, nil)
		_, err := svc.CreateMembership(ctx, req)
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("GetManyError", func(t *testing.T) {
		roles.EXPECT().GetMany(ctx, []string{"r1"}).Return(nil, errors.New("db err"))
		_, err := svc.CreateMembership(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("CreateError", func(t *testing.T) {
		roles.EXPECT().GetMany(ctx, []string{"r1"}).Return([]*iampb.Role{
			{Id: "r1", Scope: iampb.Scope_SCOPE_TENANT, TenantId: "t1"},
		}, nil)
		memberships.EXPECT().Create(ctx, gomock.Any()).Return(errors.New("db err"))
		_, err := svc.CreateMembership(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("PlatformScopeRoleAllowed", func(t *testing.T) {
		// Platform roles can be attached to any tenant membership
		roles.EXPECT().GetMany(ctx, []string{"r1"}).Return([]*iampb.Role{
			{Id: "r1", Scope: iampb.Scope_SCOPE_PLATFORM},
		}, nil)
		memberships.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		_, err := svc.CreateMembership(ctx, req)
		if err != nil {
			t.Fatalf("CreateMembership platform role: %v", err)
		}
	})
}

// -------- GetMembership --------

func TestGetMembership(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	memberships := NewMockMembershipRepo(ctrl)
	svc := NewIamService(IamDeps{Tx: &utils.MockTrm{}, Memberships: memberships})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		memberships.EXPECT().Get(ctx, "m1").Return(&iampb.Membership{Id: "m1", AccountId: "a1", TenantId: "t1"}, nil)
		resp, err := svc.GetMembership(ctx, &api.GetMembershipRequest{Id: "m1"})
		if err != nil {
			t.Fatalf("GetMembership: %v", err)
		}
		if resp.Membership.Id != "m1" {
			t.Errorf("expected m1, got %s", resp.Membership.Id)
		}
	})

	t.Run("Error", func(t *testing.T) {
		memberships.EXPECT().Get(ctx, "m1").Return(nil, errors.New("db err"))
		_, err := svc.GetMembership(ctx, &api.GetMembershipRequest{Id: "m1"})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- ListMemberships --------

func TestListMemberships(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	memberships := NewMockMembershipRepo(ctrl)
	svc := NewIamService(IamDeps{Tx: &utils.MockTrm{}, Memberships: memberships})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		memberships.EXPECT().List(ctx, "t1", uint32(10), "").Return([]*iampb.Membership{
			{Id: "m1"}, {Id: "m2"},
		}, "next", nil)
		resp, err := svc.ListMemberships(ctx, &api.ListMembershipsRequest{TenantId: "t1", PageSize: 10})
		if err != nil {
			t.Fatalf("ListMemberships: %v", err)
		}
		if len(resp.Memberships) != 2 {
			t.Errorf("expected 2, got %d", len(resp.Memberships))
		}
	})

	t.Run("Error", func(t *testing.T) {
		memberships.EXPECT().List(ctx, "t1", gomock.Any(), gomock.Any()).Return(nil, "", errors.New("db err"))
		_, err := svc.ListMemberships(ctx, &api.ListMembershipsRequest{TenantId: "t1"})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- UpdateMembership --------

func TestUpdateMembership(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	roles := NewMockRoleRepo(ctrl)
	memberships := NewMockMembershipRepo(ctrl)
	svc := NewIamService(IamDeps{
		Tx:    &utils.MockTrm{},
		Roles: roles, Memberships: memberships,
	})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		req := &api.UpdateMembershipRequest{Id: "m1", RoleIds: []string{"r1", "r2"}}
		existing := &iampb.Membership{Id: "m1", TenantId: "t1"}
		memberships.EXPECT().Get(ctx, "m1").Return(existing, nil)
		roles.EXPECT().GetMany(ctx, req.RoleIds).Return([]*iampb.Role{
			{Id: "r1", TenantId: "t1"}, {Id: "r2", TenantId: "t1"},
		}, nil)
		memberships.EXPECT().Update(ctx, existing).Return(nil)
		resp, err := svc.UpdateMembership(ctx, req)
		if err != nil {
			t.Fatalf("UpdateMembership: %v", err)
		}
		if len(resp.Membership.RoleIds) != 2 {
			t.Errorf("expected 2 role ids, got %d", len(resp.Membership.RoleIds))
		}
	})

	t.Run("GetError", func(t *testing.T) {
		memberships.EXPECT().Get(ctx, "m1").Return(nil, errors.New("db err"))
		_, err := svc.UpdateMembership(ctx, &api.UpdateMembershipRequest{Id: "m1"})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ValidateRolesError", func(t *testing.T) {
		existing := &iampb.Membership{Id: "m1", TenantId: "t1"}
		memberships.EXPECT().Get(ctx, "m1").Return(existing, nil)
		roles.EXPECT().GetMany(ctx, gomock.Any()).Return(nil, errors.New("db err"))
		_, err := svc.UpdateMembership(ctx, &api.UpdateMembershipRequest{Id: "m1", RoleIds: []string{"r1"}})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("UpdateError", func(t *testing.T) {
		req := &api.UpdateMembershipRequest{Id: "m1", RoleIds: []string{"r1"}}
		existing := &iampb.Membership{Id: "m1", TenantId: "t1"}
		memberships.EXPECT().Get(ctx, "m1").Return(existing, nil)
		roles.EXPECT().GetMany(ctx, req.RoleIds).Return([]*iampb.Role{
			{Id: "r1", TenantId: "t1"},
		}, nil)
		memberships.EXPECT().Update(ctx, existing).Return(errors.New("db err"))
		_, err := svc.UpdateMembership(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- DeleteMembership --------

func TestDeleteMembership(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	memberships := NewMockMembershipRepo(ctrl)
	svc := NewIamService(IamDeps{Tx: &utils.MockTrm{}, Memberships: memberships})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		memberships.EXPECT().Delete(ctx, "m1").Return(nil)
		_, err := svc.DeleteMembership(ctx, &api.DeleteMembershipRequest{Id: "m1"})
		if err != nil {
			t.Fatalf("DeleteMembership: %v", err)
		}
	})

	t.Run("NotFoundIsSuccess", func(t *testing.T) {
		memberships.EXPECT().Delete(ctx, "m1").Return(derrors.ErrNotFound)
		_, err := svc.DeleteMembership(ctx, &api.DeleteMembershipRequest{Id: "m1"})
		if err != nil {
			t.Fatalf("DeleteMembership not found should succeed: %v", err)
		}
	})

	t.Run("Error", func(t *testing.T) {
		memberships.EXPECT().Delete(ctx, "m1").Return(errors.New("db err"))
		_, err := svc.DeleteMembership(ctx, &api.DeleteMembershipRequest{Id: "m1"})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- GetMyPermissions --------

func TestGetMyPermissions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	authz := NewMockAuthz(ctrl)
	svc := NewIamService(IamDeps{Tx: &utils.MockTrm{}, Authn: authn, Authz: authz})
	ctx := context.Background()
	req := &api.GetMyPermissionsRequest{TenantId: "t1"}

	t.Run("Success", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		authz.EXPECT().EffectivePermissions(ctx, "a1", "t1").Return([]*iampb.Permission{
			{Resource: iampb.Resource_RESOURCE_TENANT, Action: iampb.Action_ACTION_MANAGE},
		}, nil)
		resp, err := svc.GetMyPermissions(ctx, req)
		if err != nil {
			t.Fatalf("GetMyPermissions: %v", err)
		}
		if len(resp.Permissions) != 1 {
			t.Errorf("expected 1 permission, got %d", len(resp.Permissions))
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no claims"))
		_, err := svc.GetMyPermissions(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("PermissionsError", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
		authz.EXPECT().EffectivePermissions(ctx, "a1", "t1").Return(nil, errors.New("db err"))
		_, err := svc.GetMyPermissions(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- ListPermissions --------

func TestListPermissions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := NewMockPermissionCatalog(ctrl)
	svc := NewIamService(IamDeps{Tx: &utils.MockTrm{}, Catalog: catalog})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		catalog.EXPECT().Grantable(ctx).Return([]*api.CatalogEntry{
			{Permission: &iampb.Permission{Resource: iampb.Resource_RESOURCE_TENANT}},
		}, nil)
		resp, err := svc.ListPermissions(ctx, &api.ListPermissionsRequest{})
		if err != nil {
			t.Fatalf("ListPermissions: %v", err)
		}
		if len(resp.Entries) != 1 {
			t.Errorf("expected 1 entry, got %d", len(resp.Entries))
		}
	})

	t.Run("CatalogError", func(t *testing.T) {
		catalog.EXPECT().Grantable(ctx).Return(nil, errors.New("db err"))
		_, err := svc.ListPermissions(ctx, &api.ListPermissionsRequest{})
		if err == nil {
			t.Error("expected error")
		}
	})
}
