//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
)

func TestE2E_Comparison_CompareRuns_ReturnsTypedDiff_PossiblyEmptyWithNoopMetrics(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "cmp-runs")
	cli := env.WithTenant(tnt.GetValue())

	a := createBasicRun(t, env, tnt.GetValue(), "run-A")
	b := createBasicRun(t, env, tnt.GetValue(), "run-B")
	resp, err := cli.Comparison.CompareRuns(ctx, connect.NewRequest(&testingpb.CompareRunsRequest{
		A: a.GetId(), B: b.GetId(),
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg)
}

func TestE2E_Comparison_CrossCompareBatch_ReturnsRowsForSuiteRun(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "cmp-cross")
	sr := launchedSuite(t, env, tnt.GetValue())
	cli := env.WithTenant(tnt.GetValue())
	resp, err := cli.Comparison.CrossCompareBatch(ctx, connect.NewRequest(&testingpb.CrossCompareBatchRequest{
		TestSuiteRunId: sr.GetId(),
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg)
}
