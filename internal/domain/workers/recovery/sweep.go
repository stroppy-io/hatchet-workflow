package recovery

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func Run(ctx context.Context, pool *pgxpool.Pool, log *zap.Logger) error {
	statements := []string{
		`
UPDATE node_runs
SET status = 'NODE_RUN_STATUS_READY',
    attempt = attempt + 1,
    error = COALESCE(NULLIF(error, ''), 'process_restart'),
    updated_at = now()
WHERE status IN ('NODE_RUN_STATUS_RUNNING', 'NODE_RUN_STATUS_CANCELING')
  AND deleted_at IS NULL;
`,
		`
UPDATE webhook_deliveries
SET state = 'PENDING', last_error = 'restart_requeue', updated_at = now()
WHERE state = 'IN_FLIGHT';
`,
		`
UPDATE agents
SET status = 'AGENT_STATUS_UNKNOWN', updated_at = now()
WHERE status = 'AGENT_STATUS_BUSY' AND last_seen_at < now() - interval '60 seconds';
`,
		`
WITH agg AS (
  SELECT dag_run_id,
         bool_and(status IN ('NODE_RUN_STATUS_DONE','NODE_RUN_STATUS_SKIPPED')) AS all_done,
         bool_or(status='NODE_RUN_STATUS_FAILED') AS any_failed
  FROM node_runs WHERE deleted_at IS NULL GROUP BY dag_run_id
)
UPDATE dag_runs d
SET status = CASE WHEN agg.any_failed THEN 'DAG_RUN_STATUS_FAILED'
                  WHEN agg.all_done   THEN 'DAG_RUN_STATUS_SUCCEEDED'
                  ELSE 'DAG_RUN_STATUS_RUNNING' END,
    updated_at = now()
FROM agg
WHERE d.id = agg.dag_run_id AND d.status = 'DAG_RUN_STATUS_RUNNING';
`,
	}

	for i, stmt := range statements {
		ct, err := pool.Exec(ctx, stmt)
		if err != nil {
			log.Warn("recovery sweep statement skipped", zap.Int("idx", i), zap.Error(err))
			continue
		}
		log.Info("recovery sweep statement applied", zap.Int("idx", i), zap.Int64("rows_affected", ct.RowsAffected()))
	}
	return nil
}
