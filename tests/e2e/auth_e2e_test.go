//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/sdk/client"
)

func TestE2E_Auth_LoginByEmail(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	resp, err := env.AnonClient.Auth.Login(ctx, connect.NewRequest(&iampb.LoginRequest{
		Email: env.AdminEmail, Password: env.AdminPass,
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.GetTokens().GetAccessToken())
	require.NotEmpty(t, resp.Msg.GetTokens().GetRefreshToken())
}

func TestE2E_Auth_LoginByUsername(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	resp, err := env.AnonClient.Auth.Login(ctx, connect.NewRequest(&iampb.LoginRequest{
		Email: env.AdminNick, Password: env.AdminPass,
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.GetTokens().GetAccessToken())
}

func TestE2E_Auth_LoginWrongPasswordRejected(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	_, err := env.AnonClient.Auth.Login(ctx, connect.NewRequest(&iampb.LoginRequest{
		Email: env.AdminEmail, Password: "wrong-password",
	}))
	require.Error(t, err)
}

func TestE2E_Auth_RefreshRotation(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	resp, err := env.AnonClient.Auth.Login(ctx, connect.NewRequest(&iampb.LoginRequest{
		Email: env.AdminEmail, Password: env.AdminPass,
	}))
	require.NoError(t, err)
	rt := resp.Msg.GetTokens().GetRefreshToken()
	require.NotEmpty(t, rt)

	r2, err := env.AnonClient.Auth.RefreshTokens(ctx, connect.NewRequest(&iampb.RefreshTokenRequest{RefreshToken: rt}))
	require.NoError(t, err)
	require.NotEmpty(t, r2.Msg.GetTokens().GetAccessToken())
	require.NotEqual(t, rt, r2.Msg.GetTokens().GetRefreshToken(), "refresh must rotate")
}

func TestE2E_Auth_RefreshReusedRevokesFamily(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	resp, err := env.AnonClient.Auth.Login(ctx, connect.NewRequest(&iampb.LoginRequest{
		Email: env.AdminEmail, Password: env.AdminPass,
	}))
	require.NoError(t, err)
	rt := resp.Msg.GetTokens().GetRefreshToken()

	// First refresh succeeds + rotates.
	_, err = env.AnonClient.Auth.RefreshTokens(ctx, connect.NewRequest(&iampb.RefreshTokenRequest{RefreshToken: rt}))
	require.NoError(t, err)
	// Reuse of the original (now revoked) refresh must fail.
	_, err = env.AnonClient.Auth.RefreshTokens(ctx, connect.NewRequest(&iampb.RefreshTokenRequest{RefreshToken: rt}))
	require.Error(t, err)
}

func TestE2E_Auth_LogoutInvalidatesRefresh(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	resp, err := env.AnonClient.Auth.Login(ctx, connect.NewRequest(&iampb.LoginRequest{
		Email: env.AdminEmail, Password: env.AdminPass,
	}))
	require.NoError(t, err)
	rt := resp.Msg.GetTokens().GetRefreshToken()

	_, err = env.AnonClient.Auth.Logout(ctx, connect.NewRequest(&iampb.LogoutRequest{RefreshToken: rt}))
	require.NoError(t, err)
	_, err = env.AnonClient.Auth.RefreshTokens(ctx, connect.NewRequest(&iampb.RefreshTokenRequest{RefreshToken: rt}))
	require.Error(t, err, "refresh after logout must fail")
}

func TestE2E_Auth_UpdatePassword_NewPasswordWorks_OldFails(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "pw-tnt")
	const newPass = "Str0ng!NewPass2"
	// UpdatePassword runs under UserService — Tenant middleware requires a
	// tenant scope. Use the seeded tenant.
	_, err := env.WithTenant(tnt.GetValue()).User.UpdatePassword(ctx, connect.NewRequest(&iampb.UpdatePasswordRequest{
		OldPassword: env.AdminPass, NewPassword: newPass, NewPasswordConfirmation: newPass,
	}))
	require.NoError(t, err)

	// Old password fails.
	anon := client.New(env.Server.URL)
	_, err = anon.Auth.Login(ctx, connect.NewRequest(&iampb.LoginRequest{Email: env.AdminEmail, Password: env.AdminPass}))
	require.Error(t, err)
	// New password works.
	resp, err := anon.Auth.Login(ctx, connect.NewRequest(&iampb.LoginRequest{Email: env.AdminEmail, Password: newPass}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.GetTokens().GetAccessToken())
}

func TestE2E_Auth_UpdatePasswordConfirmationMismatch(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "pw-mm")
	_, err := env.WithTenant(tnt.GetValue()).User.UpdatePassword(ctx, connect.NewRequest(&iampb.UpdatePasswordRequest{
		OldPassword:             env.AdminPass,
		NewPassword:             "Str0ng!Other",
		NewPasswordConfirmation: "Str0ng!Mismatch",
	}))
	require.Error(t, err)
}
