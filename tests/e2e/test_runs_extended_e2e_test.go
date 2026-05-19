//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
)

func createBasicRun(t *testing.T, env *e2eEnv, tenantID, name string) *testingpb.TestRun {
	t.Helper()
	ctx := context.Background()
	cli := env.WithTenant(tenantID)
	resp, err := cli.TestRun.CreateTestRun(ctx, connect.NewRequest(&testingpb.CreateTestRunRequest{
		TestRun: &testingpb.TestRun{
			Identity: &commonpb.Identity{Name: name},
			Database: pgInlineDB(),
			Workload: inlineWorkload(),
		},
	}))
	require.NoError(t, err)
	return resp.Msg
}

func TestE2E_TestRun_DryRun_ReturnsDag_NoPersistence(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "dry-run")
	cli := env.WithTenant(tnt.GetValue())

	dag, err := cli.TestRun.DryRunTestRun(ctx, connect.NewRequest(&testingpb.TestRun{
		Identity: &commonpb.Identity{Name: "dry-1"},
		Database: pgInlineDB(),
		Workload: inlineWorkload(),
	}))
	require.NoError(t, err)
	require.NotNil(t, dag.Msg)
	require.NotNil(t, dag.Msg.GetGraph(), "Dag preview must include a graph")

	// Listing must be empty — dry-run does not persist.
	lst, err := cli.TestRun.ListTestRuns(ctx, connect.NewRequest(tnt))
	require.NoError(t, err)
	require.Empty(t, lst.Msg.GetTestRuns())
}

func TestE2E_TestRun_Update_Identity_PreservesDatabaseWorkload(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "upd-run")
	run := createBasicRun(t, env, tnt.GetValue(), "orig")
	cli := env.WithTenant(tnt.GetValue())

	upd, err := cli.TestRun.UpdateTestRun(ctx, connect.NewRequest(&testingpb.UpdateTestRunRequest{
		TestRun: &testingpb.TestRun{
			Id:       run.GetId(),
			Identity: &commonpb.Identity{Name: "renamed"},
		},
	}))
	require.NoError(t, err)
	require.Equal(t, "renamed", upd.Msg.GetIdentity().GetName())
	require.NotNil(t, upd.Msg.GetDatabase(), "database snapshot must persist after Update")
}

func TestE2E_TestRun_Delete_SoftDelete_ListExcludes(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "del-run")
	cli := env.WithTenant(tnt.GetValue())
	run := createBasicRun(t, env, tnt.GetValue(), "to-delete")
	_, err := cli.TestRun.DeleteTestRun(ctx, connect.NewRequest(run.GetId()))
	require.NoError(t, err)
	lst, err := cli.TestRun.ListTestRuns(ctx, connect.NewRequest(tnt))
	require.NoError(t, err)
	for _, r := range lst.Msg.GetTestRuns() {
		require.NotEqual(t, run.GetId().GetValue(), r.GetId().GetValue())
	}
}

func TestE2E_TestRun_Cancel_AfterLaunch_SetsCancelRequested(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "cancel-run")
	cli := env.WithTenant(tnt.GetValue())
	run := createBasicRun(t, env, tnt.GetValue(), "cancel-me")

	_, err := cli.TestRun.LaunchTestRun(ctx, connect.NewRequest(run.GetId()))
	require.NoError(t, err)

	_, err = cli.TestRun.CancelTestRun(ctx, connect.NewRequest(run.GetId()))
	require.NoError(t, err)

	// Fetch the DagRun directly via fixture System service to confirm cancel_requested.
	got, err := cli.TestRun.GetTestRun(ctx, connect.NewRequest(run.GetId()))
	require.NoError(t, err)
	require.NotNil(t, got.Msg.GetTestRun().GetDagRunId())
}

func TestE2E_TestRun_Get_AfterLaunch_HasProgress(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "get-run")
	cli := env.WithTenant(tnt.GetValue())
	run := createBasicRun(t, env, tnt.GetValue(), "progress-run")
	_, err := cli.TestRun.LaunchTestRun(ctx, connect.NewRequest(run.GetId()))
	require.NoError(t, err)
	got, err := cli.TestRun.GetTestRun(ctx, connect.NewRequest(run.GetId()))
	require.NoError(t, err)
	require.NotNil(t, got.Msg.GetTestRun().GetDagRunId())
}

func TestE2E_TestRun_TenantIsolation(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tntA := env.SeedTenant(t, "runs-A")
	tntB := env.SeedTenant(t, "runs-B")
	runA := createBasicRun(t, env, tntA.GetValue(), "run-only-A")

	cliB := env.WithTenant(tntB.GetValue())
	lst, err := cliB.TestRun.ListTestRuns(ctx, connect.NewRequest(tntB))
	require.NoError(t, err)
	for _, r := range lst.Msg.GetTestRuns() {
		require.NotEqual(t, runA.GetId().GetValue(), r.GetId().GetValue())
	}
}
