package postgres

import (
	"context"

	"github.com/gopherex/sqld/pkg/migrate"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/migrations"
)

// Migrate applies the embedded SQL migrations against the pool. It needs a live
// Postgres connection.
func (db *DB) Migrate(ctx context.Context) error {
	return migrate.Migrate(ctx, db.Pool, migrations.FS)
}
