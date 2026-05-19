package admin_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/admin"
	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/admin"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

func TestAdminService_TenantLifecycle(t *testing.T) {
	f := fixture.NewIAM(t)
	svc := admin.NewAdminService(f.IAM)
	ctx := context.Background()

	owner, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "owner@e.com", Nickname: "owner"}, "P@ss1234!")
	require.NoError(t, err)

	created, err := svc.CreateTenant(ctx, &adminpb.AdminCreateTenantRequest{
		Tenant:      &iampb.Tenant{Identity: &commonpb.Identity{Name: "AdminMade"}},
		OwnerUserId: owner.GetId(),
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.GetId().GetValue())

	listResp, err := svc.ListAllTenants(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(listResp.GetTenants()), 1)

	_, err = svc.DeleteTenantHard(ctx, created.GetId())
	require.NoError(t, err)
}

func TestAdminService_UserLifecycle(t *testing.T) {
	f := fixture.NewIAM(t)
	svc := admin.NewAdminService(f.IAM)
	ctx := context.Background()

	u, err := svc.CreateUser(ctx, &adminpb.AdminCreateUserRequest{
		User:     &iampb.User{Email: "new@e.com", Nickname: "newu"},
		Password: "FreshP@ss!",
	})
	require.NoError(t, err)
	require.NotEmpty(t, u.GetId().GetValue())

	// Login works with assigned password.
	_, err = f.IAM.Login(ctx, "new@e.com", "FreshP@ss!")
	require.NoError(t, err)

	_, err = svc.ResetUserPassword(ctx, &adminpb.AdminResetPasswordRequest{
		UserId:      u.GetId(),
		NewPassword: "Reset@123!",
	})
	require.NoError(t, err)

	// Old password rejected.
	_, err = f.IAM.Login(ctx, "new@e.com", "FreshP@ss!")
	require.Error(t, err)
	// New password works.
	_, err = f.IAM.Login(ctx, "new@e.com", "Reset@123!")
	require.NoError(t, err)

	// Delete.
	_, err = svc.DeleteUser(ctx, u.GetId())
	require.NoError(t, err)

	// Login no longer possible.
	_, err = f.IAM.Login(ctx, "new@e.com", "Reset@123!")
	require.Error(t, err)
}

func TestAdminService_ListAllUsers(t *testing.T) {
	f := fixture.NewIAM(t)
	svc := admin.NewAdminService(f.IAM)
	ctx := context.Background()

	_, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "a@e.com", Nickname: "a"}, "P@ss1234!")
	require.NoError(t, err)
	_, err = f.IAM.CreateUser(ctx, &iampb.User{Email: "b@e.com", Nickname: "b"}, "P@ss1234!")
	require.NoError(t, err)

	resp, err := svc.ListAllUsers(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(resp.GetUsers()), 2)
}
