package app

import (
	"context"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/migrations"
)

// applyMigrations runs the embedded SQL migrations in lexical (timestamp) order.
// They are written idempotently by the generator; applying them on every boot
// keeps a fresh database in sync without a separate migrate step.
func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	files, err := fs.Glob(migrations.Content, "*.sql")
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, f := range files {
		body, rerr := migrations.Content.ReadFile(f)
		if rerr != nil {
			return fmt.Errorf("read %s: %w", f, rerr)
		}
		if _, eerr := pool.Exec(ctx, string(body)); eerr != nil {
			return fmt.Errorf("exec %s: %w", f, eerr)
		}
	}
	return nil
}
