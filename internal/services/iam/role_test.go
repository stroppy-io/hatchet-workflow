package iam

// role_test.go: tests for role management operations in service.go:
// CreateRole, GetRole, ListRoles, UpdateRole, DeleteRole.

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
)

// -------- CreateRole --------

func TestCreateRole(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	roles := NewMockRoleRepo(ctrl)
	svc := NewIamService(IamDeps{Tx: &MockTrm{}, Clock: FakeClock{}, Roles: roles})
	ctx := context.Background()

	t.Run("SuccessTenantScope", func(t *testing.T) {
		req := &api.CreateRoleRequest{Name: "Role", Scope: iampb.Scope_SCOPE_TENANT, TenantId: "t1"}
		roles.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		resp, err := svc.CreateRole(ctx, req)
		if err != nil {
			t.Fatalf("CreateRole: %v", err)
		}
		if resp.Role.Name != "Role" {
			t.Errorf("expected Role, got %s", resp.Role.Name)
		}
	})

	t.Run("SuccessPlatformScope", func(t *testing.T) {
		req := &api.CreateRoleRequest{Name: "Platform Role", Scope: iampb.Scope_SCOPE_PLATFORM}
		roles.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		resp, err := svc.CreateRole(ctx, req)
		if err != nil {
			t.Fatalf("CreateRole platform: %v", err)
		}
		if resp.Role.Name != "Platform Role" {
			t.Errorf("expected Platform Role, got %s", resp.Role.Name)
		}
	})

	t.Run("TenantScope_NoTenantId", func(t *testing.T) {
		req := &api.CreateRoleRequest{Scope: iampb.Scope_SCOPE_TENANT, TenantId: ""}
		_, err := svc.CreateRole(ctx, req)
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("PlatformScope_WithTenantId", func(t *testing.T) {
		req := &api.CreateRoleRequest{Scope: iampb.Scope_SCOPE_PLATFORM, TenantId: "t1"}
		_, err := svc.CreateRole(ctx, req)
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("UnknownScope", func(t *testing.T) {
		req := &api.CreateRoleRequest{Scope: 0, TenantId: ""}
		_, err := svc.CreateRole(ctx, req)
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("CreateError", func(t *testing.T) {
		req := &api.CreateRoleRequest{Name: "R", Scope: iampb.Scope_SCOPE_TENANT, TenantId: "t1"}
		roles.EXPECT().Create(ctx, gomock.Any()).Return(errors.New("db err"))
		_, err := svc.CreateRole(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- GetRole --------

func TestGetRole(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	roles := NewMockRoleRepo(ctrl)
	svc := NewIamService(IamDeps{Tx: &MockTrm{}, Clock: FakeClock{}, Roles: roles})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		roles.EXPECT().Get(ctx, "r1").Return(&iampb.Role{Id: "r1", Name: "Role 1"}, nil)
		resp, err := svc.GetRole(ctx, &api.GetRoleRequest{Id: "r1"})
		if err != nil {
			t.Fatalf("GetRole: %v", err)
		}
		if resp.Role.Id != "r1" {
			t.Errorf("expected r1, got %s", resp.Role.Id)
		}
	})

	t.Run("Error", func(t *testing.T) {
		roles.EXPECT().Get(ctx, "r1").Return(nil, errors.New("db err"))
		_, err := svc.GetRole(ctx, &api.GetRoleRequest{Id: "r1"})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		roles.EXPECT().Get(ctx, "r1").Return(nil, derrors.ErrNotFound)
		_, err := svc.GetRole(ctx, &api.GetRoleRequest{Id: "r1"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})
}

// -------- ListRoles --------

func TestListRoles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	roles := NewMockRoleRepo(ctrl)
	svc := NewIamService(IamDeps{Tx: &MockTrm{}, Clock: FakeClock{}, Roles: roles})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		roles.EXPECT().List(ctx, "t1", uint32(10), "").Return([]*iampb.Role{
			{Id: "r1"}, {Id: "r2"},
		}, "next", nil)
		resp, err := svc.ListRoles(ctx, &api.ListRolesRequest{TenantId: "t1", PageSize: 10})
		if err != nil {
			t.Fatalf("ListRoles: %v", err)
		}
		if len(resp.Roles) != 2 {
			t.Errorf("expected 2 roles, got %d", len(resp.Roles))
		}
	})

	t.Run("Error", func(t *testing.T) {
		roles.EXPECT().List(ctx, "t1", gomock.Any(), gomock.Any()).Return(nil, "", errors.New("db err"))
		_, err := svc.ListRoles(ctx, &api.ListRolesRequest{TenantId: "t1"})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- UpdateRole --------

func TestUpdateRole(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	roles := NewMockRoleRepo(ctrl)
	svc := NewIamService(IamDeps{Tx: &MockTrm{}, Clock: FakeClock{}, Roles: roles})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		name := "Updated Role"
		req := &api.UpdateRoleRequest{Id: "r1", Name: &name}
		existing := &iampb.Role{Id: "r1", Name: "Old Role"}
		roles.EXPECT().Get(ctx, "r1").Return(existing, nil)
		roles.EXPECT().Update(ctx, existing).Return(nil)
		resp, err := svc.UpdateRole(ctx, req)
		if err != nil {
			t.Fatalf("UpdateRole: %v", err)
		}
		if resp.Role.Name != "Updated Role" {
			t.Errorf("expected Updated Role, got %s", resp.Role.Name)
		}
	})

	t.Run("UpdatePermissions", func(t *testing.T) {
		perms := []*iampb.Permission{{Resource: iampb.Resource_RESOURCE_TENANT, Action: iampb.Action_ACTION_READ}}
		req := &api.UpdateRoleRequest{Id: "r1", Permissions: perms}
		existing := &iampb.Role{Id: "r1"}
		roles.EXPECT().Get(ctx, "r1").Return(existing, nil)
		roles.EXPECT().Update(ctx, existing).Return(nil)
		resp, err := svc.UpdateRole(ctx, req)
		if err != nil {
			t.Fatalf("UpdateRole permissions: %v", err)
		}
		if len(resp.Role.Permissions) != 1 {
			t.Errorf("expected 1 permission, got %d", len(resp.Role.Permissions))
		}
	})

	t.Run("SystemRoleCannotBeEdited", func(t *testing.T) {
		roles.EXPECT().Get(ctx, "sys").Return(&iampb.Role{Id: "sys", IsSystem: true}, nil)
		_, err := svc.UpdateRole(ctx, &api.UpdateRoleRequest{Id: "sys"})
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition, got %v", err)
		}
	})

	t.Run("GetError", func(t *testing.T) {
		roles.EXPECT().Get(ctx, "r1").Return(nil, errors.New("db err"))
		_, err := svc.UpdateRole(ctx, &api.UpdateRoleRequest{Id: "r1"})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("UpdateError", func(t *testing.T) {
		existing := &iampb.Role{Id: "r1"}
		roles.EXPECT().Get(ctx, "r1").Return(existing, nil)
		roles.EXPECT().Update(ctx, existing).Return(errors.New("db err"))
		_, err := svc.UpdateRole(ctx, &api.UpdateRoleRequest{Id: "r1"})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// -------- DeleteRole --------

func TestDeleteRole(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	roles := NewMockRoleRepo(ctrl)
	svc := NewIamService(IamDeps{Tx: &MockTrm{}, Clock: FakeClock{}, Roles: roles})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		roles.EXPECT().Get(ctx, "r1").Return(&iampb.Role{Id: "r1"}, nil)
		roles.EXPECT().Delete(ctx, "r1").Return(nil)
		_, err := svc.DeleteRole(ctx, &api.DeleteRoleRequest{Id: "r1"})
		if err != nil {
			t.Fatalf("DeleteRole: %v", err)
		}
	})

	t.Run("NotFound_Success", func(t *testing.T) {
		roles.EXPECT().Get(ctx, "r1").Return(nil, derrors.ErrNotFound)
		_, err := svc.DeleteRole(ctx, &api.DeleteRoleRequest{Id: "r1"})
		if err != nil {
			t.Fatalf("DeleteRole not found should succeed: %v", err)
		}
	})

	t.Run("SystemRoleCannotBeDeleted", func(t *testing.T) {
		roles.EXPECT().Get(ctx, "sys").Return(&iampb.Role{Id: "sys", IsSystem: true}, nil)
		_, err := svc.DeleteRole(ctx, &api.DeleteRoleRequest{Id: "sys"})
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition, got %v", err)
		}
	})

	t.Run("GetError", func(t *testing.T) {
		roles.EXPECT().Get(ctx, "r1").Return(nil, errors.New("db err"))
		_, err := svc.DeleteRole(ctx, &api.DeleteRoleRequest{Id: "r1"})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("DeleteError", func(t *testing.T) {
		roles.EXPECT().Get(ctx, "r1").Return(&iampb.Role{Id: "r1"}, nil)
		roles.EXPECT().Delete(ctx, "r1").Return(errors.New("db err"))
		_, err := svc.DeleteRole(ctx, &api.DeleteRoleRequest{Id: "r1"})
		if err == nil {
			t.Error("expected error")
		}
	})
}
