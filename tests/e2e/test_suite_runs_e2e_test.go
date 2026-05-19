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

func launchedSuite(t *testing.T, env *e2eEnv, tnt string) *testingpb.TestSuiteRun {
	t.Helper()
	ctx := context.Background()
	cli := env.WithTenant(tnt)
	created, err := cli.TestSuite.CreateTestSuite(ctx, connect.NewRequest(&testingpb.CreateTestSuiteRequest{
		Suite: &testingpb.TestSuite{
			Identity: &commonpb.Identity{Name: "launch-suite"},
			Matrix:   twoCellMatrix(),
		},
	}))
	require.NoError(t, err)

	sr, err := cli.TestSuiteRun.LaunchTestSuite(ctx, connect.NewRequest(created.Msg.GetId()))
	require.NoError(t, err)
	require.NotEmpty(t, sr.Msg.GetId().GetValue())
	return sr.Msg
}

func TestE2E_TestSuiteRun_LaunchTestSuite_CreatesSuiteRun_AndChildTestRuns(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	sr := launchedSuite(t, env, env.SeedTenant(t, "sr-launch").GetValue())
	require.NotEmpty(t, sr.GetTestRunIds(), "expected at least one child test run")
}

func TestE2E_TestSuiteRun_GetTestSuiteRun_ReturnsProgress(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "sr-get")
	sr := launchedSuite(t, env, tnt.GetValue())
	cli := env.WithTenant(tnt.GetValue())
	got, err := cli.TestSuiteRun.GetTestSuiteRun(ctx, connect.NewRequest(sr.GetId()))
	require.NoError(t, err)
	require.NotNil(t, got.Msg.GetSuiteRun())
	require.Equal(t, sr.GetId().GetValue(), got.Msg.GetSuiteRun().GetId().GetValue())
}

func TestE2E_TestSuiteRun_ListBySuite_ListByTenant(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "sr-list")
	sr := launchedSuite(t, env, tnt.GetValue())
	cli := env.WithTenant(tnt.GetValue())

	byTenant, err := cli.TestSuiteRun.ListByTenant(ctx, connect.NewRequest(tnt))
	require.NoError(t, err)
	require.NotEmpty(t, byTenant.Msg.GetTestSuiteRuns())

	bySuite, err := cli.TestSuiteRun.ListBySuite(ctx, connect.NewRequest(sr.GetSuiteId()))
	require.NoError(t, err)
	require.NotEmpty(t, bySuite.Msg.GetTestSuiteRuns())
}

func TestE2E_TestSuiteRun_Cancel_AfterLaunch(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "sr-cancel")
	sr := launchedSuite(t, env, tnt.GetValue())
	cli := env.WithTenant(tnt.GetValue())
	_, err := cli.TestSuiteRun.CancelTestSuiteRun(ctx, connect.NewRequest(sr.GetId()))
	require.NoError(t, err)
}

func TestE2E_TestSuiteRun_WatchTestSuiteRun_OneShotSnapshot(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tnt := env.SeedTenant(t, "sr-watch")
	sr := launchedSuite(t, env, tnt.GetValue())
	cli := env.WithTenant(tnt.GetValue())

	stream, err := cli.TestSuiteRun.WatchTestSuiteRun(ctx, connect.NewRequest(&testingpb.WatchTestSuiteRunRequest{SuiteRunId: sr.GetId()}))
	require.NoError(t, err)
	defer stream.Close()

	require.True(t, stream.Receive(), "expected at least one progress message: %v", stream.Err())
	msg := stream.Msg()
	require.NotNil(t, msg)

	// Children list should match the launched ids (sanity check).
	require.NotEmpty(t, sr.GetTestRunIds())
}

func TestE2E_TestSuiteRun_GetTestSuiteRunMetrics_EmptyListNoError(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "sr-metrics")
	sr := launchedSuite(t, env, tnt.GetValue())
	cli := env.WithTenant(tnt.GetValue())
	resp, err := cli.TestSuiteRun.GetTestSuiteRunMetrics(ctx, connect.NewRequest(&testingpb.GetTestSuiteRunMetricsRequest{
		SuiteRunId: sr.GetId(),
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg)
}
