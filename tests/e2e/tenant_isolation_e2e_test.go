//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// TestE2E_Isolation_FullSweep creates two tenants with the same admin owner
// and verifies that the catalog + testrun list endpoints scope by tenant.
// The platform admin user owns both tenants, so we use the X-Tenant-Id header
// to pick the active tenant.
func TestE2E_Isolation_FullSweep(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tntA := env.SeedTenant(t, "iso-full-A")
	tntB := env.SeedTenant(t, "iso-full-B")
	a := env.WithTenant(tntA.GetValue())
	b := env.WithTenant(tntB.GetValue())

	// Populate A with: 1 db preset + 1 test run + 1 api token + 1 additional member.
	dbPresetA, err := a.DatabasePreset.CreateDatabasePreset(ctx, connect.NewRequest(&catalogpb.CreateDatabasePresetRequest{
		Preset: newDBPreset("A-preset"),
	}))
	require.NoError(t, err)
	runA := createBasicRun(t, env, tntA.GetValue(), "A-run")
	tokA, err := a.ApiToken.CreateApiToken(ctx, connect.NewRequest(&iampb.CreateApiTokenRequest{Token: &iampb.ApiToken{Name: "A-token"}}))
	require.NoError(t, err)
	otherID, _ := env.LoginAs(t, "iso-mbr@e2e.test", "isombr", "Str0ng!Pass")
	memberA, err := a.TenantMember.AddMember(ctx, connect.NewRequest(&iampb.AddMemberRequest{
		TenantId: tntA, UserId: otherID, Role: iampb.TenantRole_TENANT_ROLE_VIEWER,
	}))
	require.NoError(t, err)

	// Populate B with: 1 db preset + 1 test run.
	dbPresetB, err := b.DatabasePreset.CreateDatabasePreset(ctx, connect.NewRequest(&catalogpb.CreateDatabasePresetRequest{
		Preset: newDBPreset("B-preset"),
	}))
	require.NoError(t, err)
	runB := createBasicRun(t, env, tntB.GetValue(), "B-run")

	// List endpoints scoped by tenant header must only return that tenant's rows.
	dpA, err := a.DatabasePreset.ListDatabasePresets(ctx, connect.NewRequest(tntA))
	require.NoError(t, err)
	for _, p := range dpA.Msg.GetDatabasePresets() {
		require.NotEqual(t, dbPresetB.Msg.GetId().GetValue(), p.GetId().GetValue(),
			"tenant A must not see tenant B's preset")
	}
	dpB, err := b.DatabasePreset.ListDatabasePresets(ctx, connect.NewRequest(tntB))
	require.NoError(t, err)
	for _, p := range dpB.Msg.GetDatabasePresets() {
		require.NotEqual(t, dbPresetA.Msg.GetId().GetValue(), p.GetId().GetValue())
	}

	trA, err := a.TestRun.ListTestRuns(ctx, connect.NewRequest(tntA))
	require.NoError(t, err)
	for _, r := range trA.Msg.GetTestRuns() {
		require.NotEqual(t, runB.GetId().GetValue(), r.GetId().GetValue())
	}
	trB, err := b.TestRun.ListTestRuns(ctx, connect.NewRequest(tntB))
	require.NoError(t, err)
	for _, r := range trB.Msg.GetTestRuns() {
		require.NotEqual(t, runA.GetId().GetValue(), r.GetId().GetValue())
	}

	// API tokens scoped to tenant A only.
	tksA, err := a.ApiToken.ListApiTokens(ctx, connect.NewRequest(tntA))
	require.NoError(t, err)
	foundTokA := false
	for _, tk := range tksA.Msg.GetApiTokens() {
		if tk.GetId().GetValue() == tokA.Msg.GetToken().GetId().GetValue() {
			foundTokA = true
		}
	}
	require.True(t, foundTokA)
	tksB, err := b.ApiToken.ListApiTokens(ctx, connect.NewRequest(tntB))
	require.NoError(t, err)
	for _, tk := range tksB.Msg.GetApiTokens() {
		require.NotEqual(t, tokA.Msg.GetToken().GetId().GetValue(), tk.GetId().GetValue())
	}

	// Members scoped to tenant A only.
	mbA, err := a.TenantMember.ListByTenant(ctx, connect.NewRequest(tntA))
	require.NoError(t, err)
	foundMember := false
	for _, m := range mbA.Msg.GetMembers() {
		if m.GetId().GetValue() == memberA.Msg.GetId().GetValue() {
			foundMember = true
		}
	}
	require.True(t, foundMember)
	mbB, err := b.TenantMember.ListByTenant(ctx, connect.NewRequest(tntB))
	require.NoError(t, err)
	for _, m := range mbB.Msg.GetMembers() {
		require.NotEqual(t, memberA.Msg.GetId().GetValue(), m.GetId().GetValue())
	}
}
