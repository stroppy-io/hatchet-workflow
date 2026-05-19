//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
	"github.com/stroppy-io/stroppy-cloud/internal/sdk/client"
)

func TestE2E_Shared_TestRun_CreateShare_ReturnsToken(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "shared-tr")
	run := createBasicRun(t, env, tnt.GetValue(), "to-share")
	cli := env.WithTenant(tnt.GetValue())
	resp, err := cli.SharedTestRun.CreateSharedTestRun(ctx, connect.NewRequest(&testingpb.CreateSharedTestRunRequest{
		TestRunId: run.GetId(),
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.GetToken())
}

func TestE2E_Shared_TestRun_GetByToken_PublicNoAuth(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "shared-tr-pub")
	run := createBasicRun(t, env, tnt.GetValue(), "share-pub")
	cli := env.WithTenant(tnt.GetValue())
	created, err := cli.SharedTestRun.CreateSharedTestRun(ctx, connect.NewRequest(&testingpb.CreateSharedTestRunRequest{
		TestRunId: run.GetId(),
	}))
	require.NoError(t, err)

	anon := client.New(env.Server.URL)
	resp, err := anon.SharedTestRun.GetByToken(ctx, connect.NewRequest(&testingpb.GetSharedTestRunByTokenRequest{
		Token: created.Msg.GetToken(),
	}))
	require.NoError(t, err)
	require.Equal(t, run.GetId().GetValue(), resp.Msg.GetTestRunId().GetValue())
}

func TestE2E_Shared_TestRun_Revoke_GetByTokenReturnsNotFound(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "shared-tr-rev")
	run := createBasicRun(t, env, tnt.GetValue(), "share-rev")
	cli := env.WithTenant(tnt.GetValue())
	created, err := cli.SharedTestRun.CreateSharedTestRun(ctx, connect.NewRequest(&testingpb.CreateSharedTestRunRequest{
		TestRunId: run.GetId(),
	}))
	require.NoError(t, err)
	_, err = cli.SharedTestRun.RevokeSharedTestRun(ctx, connect.NewRequest(created.Msg.GetId()))
	require.NoError(t, err)
	anon := client.New(env.Server.URL)
	_, err = anon.SharedTestRun.GetByToken(ctx, connect.NewRequest(&testingpb.GetSharedTestRunByTokenRequest{
		Token: created.Msg.GetToken(),
	}))
	require.Error(t, err)
}

func TestE2E_Shared_SuiteRun_CreateGetRevoke(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "shared-sr")
	sr := launchedSuite(t, env, tnt.GetValue())
	cli := env.WithTenant(tnt.GetValue())

	created, err := cli.SharedSuiteRun.CreateSharedSuiteRun(ctx, connect.NewRequest(&testingpb.CreateSharedSuiteRunRequest{
		TestSuiteRunId: sr.GetId(),
	}))
	require.NoError(t, err)
	require.NotEmpty(t, created.Msg.GetToken())

	anon := client.New(env.Server.URL)
	got, err := anon.SharedSuiteRun.GetByToken(ctx, connect.NewRequest(&testingpb.GetSharedSuiteRunByTokenRequest{
		Token: created.Msg.GetToken(),
	}))
	require.NoError(t, err)
	require.Equal(t, sr.GetId().GetValue(), got.Msg.GetTestSuiteRunId().GetValue())

	_, err = cli.SharedSuiteRun.RevokeSharedSuiteRun(ctx, connect.NewRequest(created.Msg.GetId()))
	require.NoError(t, err)
	_, err = anon.SharedSuiteRun.GetByToken(ctx, connect.NewRequest(&testingpb.GetSharedSuiteRunByTokenRequest{
		Token: created.Msg.GetToken(),
	}))
	require.Error(t, err)
}
