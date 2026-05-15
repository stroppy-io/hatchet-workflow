package iam_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

func TestCreateUser(t *testing.T) {
	f := fixture.NewIAM(t)

	user := &iampb.User{
		Email:    "alice@example.com",
		Nickname: "alice",
	}
	created, err := f.IAM.CreateUser(context.Background(), user, "P@ssw0rd!")
	require.NoError(t, err)
	require.NotEmpty(t, created.GetId().GetValue())

	got, err := f.IAM.GetUserByID(context.Background(), created.GetId())
	require.NoError(t, err)
	require.Equal(t, "alice@example.com", got.GetEmail())
}

func TestCreateUserDuplicateEmail(t *testing.T) {
	f := fixture.NewIAM(t)
	_, err := f.IAM.CreateUser(context.Background(), &iampb.User{Email: "dup@e.com", Nickname: "dup1"}, "P@ssw0rd!")
	require.NoError(t, err)
	_, err = f.IAM.CreateUser(context.Background(), &iampb.User{Email: "dup@e.com", Nickname: "dup2"}, "P@ssw0rd!")
	require.Error(t, err)
}

func TestCreateTenantAndMember(t *testing.T) {
	f := fixture.NewIAM(t)
	ctx := context.Background()

	user, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "owner@e.com", Nickname: "owner"}, "P@ssw0rd!")
	require.NoError(t, err)

	tenant, err := f.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "Acme"}}, user.GetId())
	require.NoError(t, err)
	require.NotEmpty(t, tenant.GetId().GetValue())

	ok, err := f.IAM.HasTenantRole(ctx, user.GetId(), tenant.GetId(), iampb.TenantRole_TENANT_ROLE_OWNER)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestLoginAndRefreshRotation(t *testing.T) {
	f := fixture.NewIAM(t)
	ctx := context.Background()

	_, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "user@e.com", Nickname: "user1"}, "P@ssw0rd!")
	require.NoError(t, err)

	pair1, err := f.IAM.Login(ctx, "user@e.com", "P@ssw0rd!")
	require.NoError(t, err)
	require.NotEmpty(t, pair1.GetAccessToken())
	require.NotEmpty(t, pair1.GetRefreshToken())

	pair2, err := f.IAM.RefreshTokens(ctx, pair1.GetRefreshToken())
	require.NoError(t, err)
	require.NotEqual(t, pair1.GetAccessToken(), pair2.GetAccessToken())
	require.NotEqual(t, pair1.GetRefreshToken(), pair2.GetRefreshToken())

	// Reuse old token → triggers family revoke
	_, err = f.IAM.RefreshTokens(ctx, pair1.GetRefreshToken())
	require.Error(t, err)

	// New token also invalid (family revoked)
	_, err = f.IAM.RefreshTokens(ctx, pair2.GetRefreshToken())
	require.Error(t, err)
}

func TestLoginWrongPassword(t *testing.T) {
	f := fixture.NewIAM(t)
	ctx := context.Background()

	_, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "u@e.com", Nickname: "u"}, "Correct123!")
	require.NoError(t, err)

	_, err = f.IAM.Login(ctx, "u@e.com", "wrong")
	require.Error(t, err)
}
