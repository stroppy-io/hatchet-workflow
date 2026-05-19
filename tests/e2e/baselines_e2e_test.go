//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
)

func TestE2E_Baselines_SetGetListDelete(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "baseline-tenant")
	cli := env.WithTenant(tnt.GetValue())
	run := createBasicRun(t, env, tnt.GetValue(), "baseline-src")

	set, err := cli.Baseline.SetBaseline(ctx, connect.NewRequest(&testingpb.SetBaselineRequest{
		TenantId: tnt, Name: "prod", TestRunId: run.GetId(),
	}))
	require.NoError(t, err)

	got, err := cli.Baseline.GetBaseline(ctx, connect.NewRequest(&testingpb.GetBaselineRequest{
		TenantId: tnt, Name: "prod",
	}))
	require.NoError(t, err)
	require.Equal(t, "prod", got.Msg.GetName())

	lst, err := cli.Baseline.ListBaselines(ctx, connect.NewRequest(tnt))
	require.NoError(t, err)
	require.NotEmpty(t, lst.Msg.GetBaselines())

	_, err = cli.Baseline.DeleteBaseline(ctx, connect.NewRequest(set.Msg.GetId()))
	require.NoError(t, err)
}

func TestE2E_Baselines_TenantIsolation(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tntA := env.SeedTenant(t, "bl-iso-A")
	tntB := env.SeedTenant(t, "bl-iso-B")
	runA := createBasicRun(t, env, tntA.GetValue(), "bl-src-A")

	a := env.WithTenant(tntA.GetValue())
	b := env.WithTenant(tntB.GetValue())
	created, err := a.Baseline.SetBaseline(ctx, connect.NewRequest(&testingpb.SetBaselineRequest{
		TenantId: tntA, Name: "only-A", TestRunId: runA.GetId(),
	}))
	require.NoError(t, err)
	lstB, err := b.Baseline.ListBaselines(ctx, connect.NewRequest(tntB))
	require.NoError(t, err)
	for _, bl := range lstB.Msg.GetBaselines() {
		require.NotEqual(t, created.Msg.GetId().GetValue(), bl.GetId().GetValue())
	}
}
