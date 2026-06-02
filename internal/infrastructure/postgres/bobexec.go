package postgres

import (
	"context"
	"database/sql"

	pgtxlib "github.com/gopherex/pgtx/pkg/tx"
	"github.com/jackc/pgx/v5"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/scan"
)

// bobExec adapts the ctx-aware pgtx executor (db.TxDB) to bob.Executor. The key
// property: db.TxDB picks up the current transaction from ctx (pgtx/trm), so bob
// operations issued through this executor inside tx.Do* run IN THE SAME
// transaction the service manages. Outside a transaction they run on the pool
// (auto-commit).
type bobExec struct {
	db pgtxlib.DB
}

// Bobx returns a bob.Executor over the ctx-aware TxDB (tx-aware).
func (db *DB) Bobx() bob.Executor { return bobExec{db: db.TxDB} }

var _ bob.Executor = bobExec{}

func (e bobExec) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	tag, err := e.db.Exec(ctx, query, args...)
	return bobResult{tag.RowsAffected()}, err
}

func (e bobExec) QueryContext(ctx context.Context, query string, args ...any) (scan.Rows, error) {
	rs, err := e.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return bobRows{rs}, nil
}

// bobResult is sql.Result over pgconn.CommandTag (only RowsAffected is meaningful in pg).
type bobResult struct{ affected int64 }

func (r bobResult) LastInsertId() (int64, error) { return 0, nil }
func (r bobResult) RowsAffected() (int64, error) { return r.affected, nil }

// bobRows is scan.Rows over pgx.Rows (adds Columns/Close-with-error to pgx.Rows).
type bobRows struct{ pgx.Rows }

func (r bobRows) Close() error { r.Rows.Close(); return nil }

func (r bobRows) Columns() ([]string, error) {
	fds := r.Rows.FieldDescriptions()
	cols := make([]string, len(fds))
	for i, fd := range fds {
		cols[i] = fd.Name
	}
	return cols, nil
}
