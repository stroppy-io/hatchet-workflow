// Package postgres is the Postgres-backed persistence for the control plane. It
// replaces the GORM store with a hand-written pgx + pgtx + bob layer following
// the komeet pattern: a single *pgxpool.Pool, a pgtx transaction manager
// (tx.Trm) the services use to run repo calls inside an ambient transaction, a
// ctx-aware TxDB executor, and a bob pool for typed query building.
//
// Each record type maps to a real table mirroring the queryable envelope
// columns (id, tenant_id, created_at, updated_at, plus any secondary keys) with
// the full proto message stored as a `data jsonb` column (canonical protojson).
// Upserts use `insert ... on conflict do update`; reads map pgx.ErrNoRows onto
// the domain not-found so services translate it to gRPC NotFound. The Store
// facade exposes typed repos with the SAME accessor names the GORM store used,
// so the integration layer wires this in identically.
package postgres

import (
	"context"

	"github.com/gopherex/pgtx"
	pgtxlib "github.com/gopherex/pgtx/pkg/tx"
	"github.com/jackc/pgx/v5/pgxpool"
	bobpgx "github.com/stephenafamo/bob/drivers/pgx"

	dbgen "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
)

// DB is the Postgres connection bundle: the raw pool, the pgtx transaction
// manager services run repo calls through, the ctx-aware TxDB executor (picks up
// the ambient transaction from ctx) and a bob pool over the same pool.
type DB struct {
	Pool      *pgxpool.Pool
	TxManager pgtxlib.Trm
	TxDB      pgtxlib.DB
	Bob       bobpgx.Pool
}

// Connect opens a pgx pool against dsn and builds the tx manager / executors.
func Connect(ctx context.Context, dsn string) (*DB, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	txManager, err := pgtx.NewTxManager(pool, pgtxlib.ReadCommitted())
	if err != nil {
		pool.Close()
		return nil, err
	}
	return &DB{
		Pool:      pool,
		TxManager: txManager,
		TxDB:      pgtx.NewTxDB(pool),
		Bob:       bobpgx.NewPool(pool),
	}, nil
}

// Trm returns the transaction manager services wire into their tx.Trm field so
// repo calls inside tx.Do* share one transaction.
func (db *DB) Trm() pgtxlib.Trm { return db.TxManager }

// q returns the sqld-generated typed query set bound to the ctx-aware TxDB
// executor. Because db.TxDB picks up the ambient transaction from ctx, every
// generated query func issued through it runs in the service's open
// transaction (inside tx.Do*) or on the pool (auto-commit) otherwise.
func (db *DB) q() *dbgen.Queries { return dbgen.New(db.TxDB) }

// Close releases the underlying pool.
func (db *DB) Close() {
	if db != nil && db.Pool != nil {
		db.Pool.Close()
	}
}

// Ping verifies connectivity.
func (db *DB) Ping(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}
