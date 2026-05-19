//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

func TestE2E_Members_AddListUpdateRemoveMember(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "Acme")

	// Make a second user to add as member.
	memberID, _ := env.LoginAs(t, "member@e2e.test", "memberA", "Str0ng!Pass")

	// Add member with VIEWER role. Use X-Tenant-Id header to ensure tenant
	// middleware resolves tenant from header (avoiding body-reflection edge cases).
	tcli := env.WithTenant(tnt.GetValue())
	addResp, err := tcli.TenantMember.AddMember(ctx, connect.NewRequest(&iampb.AddMemberRequest{
		TenantId: tnt, UserId: memberID, Role: iampb.TenantRole_TENANT_ROLE_VIEWER,
	}))
	require.NoError(t, err)
	mid := addResp.Msg.GetId()
	require.NotEmpty(t, mid.GetValue())
	require.Equal(t, iampb.TenantRole_TENANT_ROLE_VIEWER, addResp.Msg.GetRole())

	// ListByTenant must contain admin (owner) + new member.
	lst, err := tcli.TenantMember.ListByTenant(ctx, connect.NewRequest(tnt))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(lst.Msg.GetMembers()), 2)

	// Update role to ADMIN.
	updResp, err := tcli.TenantMember.UpdateMemberRole(ctx, connect.NewRequest(&iampb.UpdateMemberRoleRequest{
		MemberId: mid, Role: iampb.TenantRole_TENANT_ROLE_ADMIN,
	}))
	require.NoError(t, err)
	require.Equal(t, iampb.TenantRole_TENANT_ROLE_ADMIN, updResp.Msg.GetRole())

	// Remove.
	_, err = tcli.TenantMember.RemoveMember(ctx, connect.NewRequest(mid))
	require.NoError(t, err)
	lst2, err := tcli.TenantMember.ListByTenant(ctx, connect.NewRequest(tnt))
	require.NoError(t, err)
	for _, m := range lst2.Msg.GetMembers() {
		require.NotEqual(t, mid.GetValue(), m.GetId().GetValue())
	}
}

func TestE2E_Members_ListByUser(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	a := env.SeedTenant(t, "A")
	b := env.SeedTenant(t, "B")

	memberID, _ := env.LoginAs(t, "u@e2e.test", "uByUser", "Str0ng!Pass")
	_, err := env.WithTenant(a.GetValue()).TenantMember.AddMember(ctx, connect.NewRequest(&iampb.AddMemberRequest{
		TenantId: a, UserId: memberID, Role: iampb.TenantRole_TENANT_ROLE_MEMBER,
	}))
	require.NoError(t, err)
	_, err = env.WithTenant(b.GetValue()).TenantMember.AddMember(ctx, connect.NewRequest(&iampb.AddMemberRequest{
		TenantId: b, UserId: memberID, Role: iampb.TenantRole_TENANT_ROLE_VIEWER,
	}))
	require.NoError(t, err)

	resp, err := env.WithTenant(a.GetValue()).TenantMember.ListByUser(ctx, connect.NewRequest(memberID))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(resp.Msg.GetMembers()), 2)
}

func TestE2E_Members_CrossTenantInvisible(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	a := env.SeedTenant(t, "AInvis")
	b := env.SeedTenant(t, "BInvis")

	memberID, _ := env.LoginAs(t, "mx@e2e.test", "mxOnlyA", "Str0ng!Pass")
	_, err := env.WithTenant(a.GetValue()).TenantMember.AddMember(ctx, connect.NewRequest(&iampb.AddMemberRequest{
		TenantId: a, UserId: memberID, Role: iampb.TenantRole_TENANT_ROLE_VIEWER,
	}))
	require.NoError(t, err)

	// In tenant B's member list, memberID must be absent.
	resp, err := env.WithTenant(b.GetValue()).TenantMember.ListByTenant(ctx, connect.NewRequest(b))
	require.NoError(t, err)
	for _, m := range resp.Msg.GetMembers() {
		require.NotEqual(t, memberID.GetValue(), m.GetUserId().GetValue(), "member of A must not appear in B's roster")
	}
}
