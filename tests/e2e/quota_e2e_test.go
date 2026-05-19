//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	opspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/ops"
)

func TestE2E_Quota_GetQuotas_ReturnsConfiguredOrEmpty(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "quota-get")
	cli := env.WithTenant(tnt.GetValue())
	resp, err := cli.Quota.GetQuotas(ctx, connect.NewRequest(&opspb.GetQuotasRequest{TenantId: tnt}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg)
}

func TestE2E_Quota_RefreshQuotas_NoError(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "quota-refresh")
	cli := env.WithTenant(tnt.GetValue())
	resp, err := cli.Quota.RefreshQuotas(ctx, connect.NewRequest(&opspb.RefreshQuotasRequest{TenantId: tnt}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg)
}
