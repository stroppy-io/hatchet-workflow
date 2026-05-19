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

func TestUpdatePasswordHappyPath(t *testing.T) {
	f := fixture.NewIAM(t)
	ctx := context.Background()

	created, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "pw@e.com", Nickname: "pw"}, "OldPass123!")
	require.NoError(t, err)

	// Old password works.
	_, err = f.IAM.Login(ctx, "pw@e.com", "OldPass123!")
	require.NoError(t, err)

	updated, err := f.IAM.UpdatePassword(ctx, created.GetId(), "OldPass123!", "NewPass456!", "NewPass456!")
	require.NoError(t, err)
	require.Equal(t, created.GetId().GetValue(), updated.GetId().GetValue())

	// Old password rejected, new password accepted.
	_, err = f.IAM.Login(ctx, "pw@e.com", "OldPass123!")
	require.Error(t, err)
	_, err = f.IAM.Login(ctx, "pw@e.com", "NewPass456!")
	require.NoError(t, err)
}

func TestUpdatePasswordWrongOld(t *testing.T) {
	f := fixture.NewIAM(t)
	ctx := context.Background()
	created, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "pw2@e.com", Nickname: "pw2"}, "OldPass123!")
	require.NoError(t, err)

	_, err = f.IAM.UpdatePassword(ctx, created.GetId(), "WrongOld!", "NewPass456!", "NewPass456!")
	require.Error(t, err)
}

func TestUpdatePasswordConfirmationMismatch(t *testing.T) {
	f := fixture.NewIAM(t)
	ctx := context.Background()
	created, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "pw3@e.com", Nickname: "pw3"}, "OldPass123!")
	require.NoError(t, err)

	_, err = f.IAM.UpdatePassword(ctx, created.GetId(), "OldPass123!", "NewPass456!", "DifferentConfirm!")
	require.Error(t, err)
}

// TestLoginByUsername: identifier without "@" resolved against nickname column.
func TestLoginByUsername(t *testing.T) {
	f := fixture.NewIAM(t)
	ctx := context.Background()

	_, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "admin@e.com", Nickname: "admin"}, "AdminP@ss!")
	require.NoError(t, err)

	pair, err := f.IAM.Login(ctx, "admin", "AdminP@ss!")
	require.NoError(t, err)
	require.NotEmpty(t, pair.GetAccessToken())
}

func TestLoginByUsernameUnknownNickname(t *testing.T) {
	f := fixture.NewIAM(t)
	ctx := context.Background()

	_, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "x@e.com", Nickname: "knownnick"}, "P@ss123!")
	require.NoError(t, err)

	_, err = f.IAM.Login(ctx, "unknownnick", "P@ss123!")
	require.Error(t, err)
}

func TestApiTokenCreateAndVerify(t *testing.T) {
	f := fixture.NewIAM(t)
	ctx := context.Background()

	u, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "t@e.com", Nickname: "t"}, "P@ss123!")
	require.NoError(t, err)

	tn, err := f.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "T"}}, u.GetId())
	require.NoError(t, err)

	token, plain, err := f.IAM.CreateApiToken(ctx, tn.GetId(), u.GetId(), "ci-pipeline")
	require.NoError(t, err)
	require.NotEmpty(t, plain)
	require.NotEmpty(t, token.GetId().GetValue())

	resolved, err := f.IAM.VerifyApiToken(ctx, plain)
	require.NoError(t, err)
	require.Equal(t, tn.GetId().GetValue(), resolved.GetTenantId().GetValue())
}
