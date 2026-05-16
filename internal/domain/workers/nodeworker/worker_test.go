package nodeworker_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

func TestWorkerCompletesTwoNodeDag(t *testing.T) {
	f := fixture.NewSystem(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Build dag: A → B. Real Dag_Node fields are Id, Deps, MaxAttempts.
	dag := &systempb.Dag{
		Graph: &systempb.Dag_Graph{
			Nodes: []*systempb.Dag_Node{
				{Id: "a", MaxAttempts: 1},
				{Id: "b", Deps: []string{"a"}, MaxAttempts: 1},
			},
		},
	}
	dag, err := f.System.SaveDag(ctx, dag)
	require.NoError(t, err)

	run, err := f.System.StartDagRun(ctx, system.StartDagRunInput{Dag: dag})
	require.NoError(t, err)

	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()
	go f.Worker.Run(workerCtx)

	deadline := time.Now().Add(15 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("dag_run did not complete in time")
		}
		got, err := f.System.GetDagRun(ctx, run.GetId())
		require.NoError(t, err)
		if got.GetStatus() == systempb.DagRunStatus_DAG_RUN_STATUS_SUCCEEDED {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
