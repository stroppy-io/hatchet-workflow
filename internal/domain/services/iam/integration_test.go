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
