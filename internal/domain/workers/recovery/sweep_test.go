package recovery_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/recovery"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

func TestRecoveryResetsRunningToReady(t *testing.T) {
	f := fixture.NewSystem(t)
	ctx := context.Background()

	dag := &systempb.Dag{Graph: &systempb.Dag_Graph{Nodes: []*systempb.Dag_Node{{
		Id:          "x",
		Type:        "Mock",
		MaxAttempts: 3,
	}}}}
	dag, err := f.System.SaveDag(ctx, dag)
	require.NoError(t, err)
	run, err := f.System.StartDagRun(ctx, system.StartDagRunInput{Dag: dag})
	require.NoError(t, err)

	_, err = f.F.Pool.Exec(ctx, `UPDATE node_runs SET status='NODE_RUN_STATUS_RUNNING' WHERE dag_run_id=$1`, run.GetId().GetValue())
	require.NoError(t, err)

	require.NoError(t, recovery.Run(ctx, f.F.Pool, zap.NewNop()))

	rows, err := f.F.Pool.Query(ctx, `SELECT status FROM node_runs WHERE dag_run_id=$1`, run.GetId().GetValue())
	require.NoError(t, err)
	defer rows.Close()
	var status string
	require.True(t, rows.Next())
	require.NoError(t, rows.Scan(&status))
	require.Equal(t, "NODE_RUN_STATUS_READY", status)
}
