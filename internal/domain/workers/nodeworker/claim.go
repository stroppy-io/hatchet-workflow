// Raw SQL is required here for SKIP LOCKED, which ratel does not (yet) support.
// All other database access in this binary goes through ratel — see CLAUDE.md.
package nodeworker

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type claimedNode struct {
	ID       string
	DagRunID string
	NodeID   string
	Attempt  int32
}

func claimReadyNode(ctx context.Context, pool *pgxpool.Pool, readyStatus, runningStatus string) (*claimedNode, error) {
	const q = `
WITH ready AS (
    SELECT id FROM node_runs
    WHERE status = $1 AND deleted_at IS NULL
    ORDER BY created_at NULLS FIRST, id
    FOR UPDATE SKIP LOCKED LIMIT 1
)
UPDATE node_runs n
SET status = $2,
    started_at = COALESCE(n.started_at, now()),
    updated_at = now()
FROM ready
WHERE n.id = ready.id
RETURNING n.id, n.dag_run_id, n.node_id, n.attempt;
`
	row := pool.QueryRow(ctx, q, readyStatus, runningStatus)
	var n claimedNode
	if err := row.Scan(&n.ID, &n.DagRunID, &n.NodeID, &n.Attempt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &n, nil
}
