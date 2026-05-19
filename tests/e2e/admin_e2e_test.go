//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/admin"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/sdk/client"
)

func TestE2E_Admin_AdminCreateTenantPlaceholderOwner_SubstitutesCaller(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()

	resp, err := env.AdminClient.Admin.CreateTenant(ctx, connect.NewRequest(&adminpb.AdminCreateTenantRequest{
		Tenant:      &iampb.Tenant{Identity: &commonpb.Identity{Name: "admin-made"}},
		OwnerUserId: &iampb.UserId{Value: ids.Placeholder},
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.GetId().GetValue())
}

func TestE2E_Admin_ListAllTenantsAcrossOrgs(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	env.SeedTenant(t, "Acme")
	env.SeedTenant(t, "Globex")

	resp, err := env.AdminClient.Admin.ListAllTenants(ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(resp.Msg.GetTenants()), 2)
}

func TestE2E_Admin_ListAllUsers(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	env.LoginAs(t, "bob@e2e.test", "bobx", "Str0ng!Pass")

	resp, err := env.AdminClient.Admin.ListAllUsers(ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(resp.Msg.GetUsers()), 2)
}

func TestE2E_Admin_CreateUserAndDelete(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()

	createResp, err := env.AdminClient.Admin.CreateUser(ctx, connect.NewRequest(&adminpb.AdminCreateUserRequest{
		User:     &iampb.User{Email: "admincreated@e2e.test", Nickname: "ac1"},
		Password: "Str0ng!Pass",
	}))
	require.NoError(t, err)
	uid := createResp.Msg.GetId()

	_, err = env.AdminClient.Admin.DeleteUser(ctx, connect.NewRequest(uid))
	require.NoError(t, err)
}

func TestE2E_Admin_ResetUserPassword_OldFails_NewWorks(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()

	uid, userEmail, _ := env.LoginAsFull(t, "reset@e2e.test", "rset", "Str0ng!Old")
	const newPass = "Str0ng!Reset"
	_, err := env.AdminClient.Admin.ResetUserPassword(ctx, connect.NewRequest(&adminpb.AdminResetPasswordRequest{
		UserId: uid, NewPassword: newPass,
	}))
	require.NoError(t, err)

	// Old fails.
	_, err = env.AnonClient.Auth.Login(ctx, connect.NewRequest(&iampb.LoginRequest{Email: userEmail, Password: "Str0ng!Old"}))
	require.Error(t, err)
	// New works.
	resp, err := env.AnonClient.Auth.Login(ctx, connect.NewRequest(&iampb.LoginRequest{Email: userEmail, Password: newPass}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.GetTokens().GetAccessToken())
}

func TestE2E_Admin_NonAdminBlocked(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	_, plainToken := env.LoginAs(t, "plain@e2e.test", "plainuser", "Str0ng!Pass")

	plain := client.New(env.Server.URL, client.WithBearer(plainToken))
	_, err := plain.Admin.ListAllTenants(ctx, connect.NewRequest(&emptypb.Empty{}))
	require.Error(t, err)
	if cerr := new(connect.Error); err != nil {
		// Code must be PermissionDenied (or Unauthenticated, depending on middleware).
		cerrU, ok := err.(*connect.Error)
		require.True(t, ok, "expected *connect.Error, got %T (%v)", err, err)
		code := cerrU.Code()
		require.Truef(t, code == connect.CodePermissionDenied || code == connect.CodeUnauthenticated,
			"expected PermissionDenied or Unauthenticated, got %v", code)
		_ = cerr
	}
}
