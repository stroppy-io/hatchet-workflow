package system_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

func TestCancelDagRun(t *testing.T) {
	f := fixture.NewSystem(t)
	ctx := context.Background()

	dag := &systempb.Dag{
		Graph: &systempb.Dag_Graph{
			Nodes: []*systempb.Dag_Node{{Id: "a", MaxAttempts: 1}},
		},
	}
	dag, err := f.System.SaveDag(ctx, dag)
	require.NoError(t, err)

	run, err := f.System.StartDagRun(ctx, system.StartDagRunInput{Dag: dag})
	require.NoError(t, err)

	err = f.System.CancelDagRun(ctx, run.GetId())
	require.NoError(t, err)

	got, err := f.System.GetDagRun(ctx, run.GetId())
	require.NoError(t, err)
	require.True(t, got.GetCancelRequested(), "cancel_requested must be true")
}

func TestCancelDagRun_NotFound(t *testing.T) {
	f := fixture.NewSystem(t)
	ctx := context.Background()

	err := f.System.CancelDagRun(ctx, &systempb.DagRunId{Value: "nonexistent-id"})
	require.Error(t, err)
}
