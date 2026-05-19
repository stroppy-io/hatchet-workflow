package pgcontainer

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	pgxinfra "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/migrations"
)

var (
	once    sync.Once
	rootDSN string
	rootErr error
	rootCnt testcontainers.Container
)

func Shared(t *testing.T) string {
	t.Helper()
	once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		c, err := postgres.Run(ctx,
			"postgres:18-alpine",
			postgres.WithDatabase("stroppy"),
			postgres.WithUsername("stroppy"),
			postgres.WithPassword("stroppy"),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).WithStartupTimeout(2*time.Minute),
			),
		)
		if err != nil {
			rootErr = err
			return
		}
		rootCnt = c
		dsn, err := c.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			rootErr = err
			return
		}
		rootDSN = dsn
	})
	if rootErr != nil {
		t.Fatalf("pgcontainer: shared container start failed: %v", rootErr)
	}
	return rootDSN
}

// Bootstrap returns a pool that is logically isolated from other tests in the
// same package: the shared container is migrated once (idempotent via Atlas
// revisions), and every Bootstrap() call TRUNCATEs all user-data tables on
// t.Cleanup so leftover rows don't bleed into the next test's unique
// constraints. Migration DDL hard-codes `public` (Atlas pins schema in the
// generated SQL), so true schema-per-test isn't viable without rewriting
// every migration; TRUNCATE is the next-best isolation.
func Bootstrap(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := Shared(t)
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("pgcontainer: parse config: %v", err)
	}
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		registerProtoTypes(conn.TypeMap())
		return nil
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("pgcontainer: pool: %v", err)
	}

	migrateOnce(t, pool)
	truncateAll(t, pool)

	t.Cleanup(func() {
		truncateAll(t, pool)
		pool.Close()
	})
	return pool
}

var migrateOnceFlag sync.Once
var migrateOnceErr error

// migrateOnce applies migrations the first time any test in the process
// touches the shared container. Subsequent Bootstraps reuse the schema.
func migrateOnce(t *testing.T, pool *pgxpool.Pool) {
	migrateOnceFlag.Do(func() {
		migrateOnceErr = pgxinfra.MigrateAtlas(pool, migrations.Content)
	})
	if migrateOnceErr != nil {
		t.Fatalf("pgcontainer: migrate: %v", migrateOnceErr)
	}
}

// truncateAll wipes every user-data table in `public` except the Atlas
// revision-tracking table. Order doesn't matter — CASCADE handles FKs.
func truncateAll(t *testing.T, pool *pgxpool.Pool) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	rows, err := pool.Query(ctx, `
		SELECT tablename FROM pg_tables
		WHERE schemaname = 'public'
		  AND tablename NOT LIKE 'atlas_%'
	`)
	if err != nil {
		t.Fatalf("pgcontainer: list tables: %v", err)
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			t.Fatalf("pgcontainer: scan table name: %v", err)
		}
		names = append(names, fmt.Sprintf("%q", n))
	}
	rows.Close()
	if len(names) == 0 {
		return
	}
	stmt := "TRUNCATE TABLE " + strings.Join(names, ", ") + " RESTART IDENTITY CASCADE"
	if _, err := pool.Exec(ctx, stmt); err != nil {
		t.Fatalf("pgcontainer: truncate: %v", err)
	}
}

// registerProtoTypes registers custom pgx codecs needed for proto message
// fields that are stored in PostgreSQL text columns. This is necessary because
// ratel-generated scanners use *anypb.Any directly for text NOT NULL columns.
func registerProtoTypes(tm *pgtype.Map) {
	tm.RegisterType(&pgtype.Type{
		Name:  "text",
		OID:   pgtype.TextOID,
		Codec: anypbCodec{},
	})
}
