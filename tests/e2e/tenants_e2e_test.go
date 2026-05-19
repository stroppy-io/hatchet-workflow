//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

func TestE2E_Tenant_ListMyTenantsReflectsMembership(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	a := env.SeedTenant(t, "A")
	b := env.SeedTenant(t, "B")

	resp, err := env.AdminClient.Tenant.ListMyTenants(ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	ids := map[string]bool{}
	for _, tn := range resp.Msg.GetTenants() {
		ids[tn.GetId().GetValue()] = true
	}
	require.True(t, ids[a.GetValue()])
	require.True(t, ids[b.GetValue()])
}

func TestE2E_Tenant_UpdateTenantIdentity(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	id := env.SeedTenant(t, "OriginalName")

	desc := "renamed via update"
	upd := &iampb.Tenant{
		Id:       id,
		Identity: &commonpb.Identity{Name: "RenamedOrg", Description: &desc},
	}
	resp, err := env.WithTenant(id.GetValue()).Tenant.UpdateTenant(ctx, connect.NewRequest(&iampb.UpdateTenantRequest{Tenant: upd}))
	require.NoError(t, err)
	require.Equal(t, "RenamedOrg", resp.Msg.GetIdentity().GetName())
}

func TestE2E_Tenant_DeleteTenantSoftDeleteHidesFromList(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	id := env.SeedTenant(t, "Doomed")

	_, err := env.WithTenant(id.GetValue()).Tenant.DeleteTenant(ctx, connect.NewRequest(&iampb.TenantId{Value: id.GetValue()}))
	require.NoError(t, err)

	resp, err := env.AdminClient.Tenant.ListMyTenants(ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	for _, tn := range resp.Msg.GetTenants() {
		require.NotEqual(t, id.GetValue(), tn.GetId().GetValue(), "deleted tenant must not appear in ListMyTenants")
	}
}
