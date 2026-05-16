package system_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

func TestSubscribeProgress_DeliversNodeDoneEvents(t *testing.T) {
	f := fixture.NewSystem(t)
	ctx := context.Background()

	dag := &systempb.Dag{Graph: &systempb.Dag_Graph{Nodes: []*systempb.Dag_Node{
		{Id: "a", Type: "Mock", MaxAttempts: 1},
	}}}
	dag, err := f.System.SaveDag(ctx, dag)
	require.NoError(t, err)
	run, err := f.System.StartDagRun(ctx, system.StartDagRunInput{Dag: dag})
	require.NoError(t, err)

	ch, release, err := f.System.SubscribeProgress(ctx, run.GetId(), "")
	require.NoError(t, err)
	defer release()

	// Look up the node_run created by StartDagRun.
	nodeRuns, err := f.System.ListNodeRunsByDagRun(ctx, run.GetId())
	require.NoError(t, err)
	require.Len(t, nodeRuns, 1)

	// Use a non-nil anypb.Any to satisfy the NOT NULL output column constraint.
	empty, err := anypb.New(&structpb.Struct{})
	require.NoError(t, err)
	require.NoError(t, f.System.MarkNodeRunSucceeded(ctx, nodeRuns[0].GetId(), empty))

	select {
	case nr := <-ch:
		require.Equal(t, nodeRuns[0].GetId().GetValue(), nr.GetId().GetValue())
	case <-time.After(2 * time.Second):
		t.Fatal("no progress event received")
	}
}
