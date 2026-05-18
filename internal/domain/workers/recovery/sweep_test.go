package recovery_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yaroher/ratel/pkg/pgx-ext/sqlexec"
	"go.uber.org/zap"

	agentsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/agent"
	opssvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/recovery"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

func TestRecoveryResetsRunningToReady(t *testing.T) {
	f := fixture.NewSystem(t)
	ctx := context.Background()
	exec := f.F.Executor.(*sqlexec.TxExecutor)

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

	agentSvc := agentsvc.New(exec, f.F.TxMgr, f.F.Events, agentsvc.NewHub(), agentsvc.NewCommandsRepo(exec, f.F.TxMgr), agentsvc.NewBootstrapTokenStore(make([]byte, 32)))
	webhookSvc := opssvc.NewWebhookService(exec, f.F.TxMgr, f.F.Events)
	systemSvc := system.New(exec, f.F.TxMgr, f.F.Events)

	require.NoError(t, recovery.Run(ctx, systemSvc, agentSvc, webhookSvc, zap.NewNop()))

	rows, err := f.F.Pool.Query(ctx, `SELECT status FROM node_runs WHERE dag_run_id=$1`, run.GetId().GetValue())
	require.NoError(t, err)
	defer rows.Close()
	var status string
	require.True(t, rows.Next())
	require.NoError(t, rows.Scan(&status))
	require.Equal(t, "NODE_RUN_STATUS_READY", status)
}
