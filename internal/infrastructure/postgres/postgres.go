package postgres

import (
	"context"
	"fmt"
	"hash/fnv"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yaroher/ratel/pkg/migrate"

	"github.com/stroppy-io/stroppy-cloud/internal/core/configurator"
	"github.com/stroppy-io/stroppy-cloud/internal/core/logger"
)

const migrationSchema = "public"

type MigrationContent = fs.FS

func New(cfg configurator.PostgresConfig) (*pgxpool.Pool, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("postgres: DSN required")
	}
	pcfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse DSN: %w", err)
	}
	if cfg.MaxConns > 0 {
		pcfg.MaxConns = int32(cfg.MaxConns)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), pcfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: new pool: %w", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return pool, nil
}

// MigrateAtlas applies the given migration filesystems to the pool using ratel's
// Atlas-backed migrator. The ratel v0.4.25 signature is:
//
//	migrate.Migrate(pool, lg, schema, migrations ...fs.FS) error
func MigrateAtlas(pool *pgxpool.Pool, migrations ...MigrationContent) error {
	return migrate.Migrate(pool, logger.Global().Named("migrate"), migrationSchema, migrations...)
}

func MigrateWithLock(pool *pgxpool.Pool, lockName string, migrations ...MigrationContent) error {
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("postgres: acquire conn for lock: %w", err)
	}
	defer conn.Release()

	lockKey := int64(hash64(lockName))
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lockKey); err != nil {
		return fmt.Errorf("postgres: advisory_lock: %w", err)
	}
	defer func() {
		_, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", lockKey)
	}()
	return MigrateAtlas(pool, migrations...)
}

func hash64(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}
