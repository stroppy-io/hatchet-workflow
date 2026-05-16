package pgcontainer

import (
	"context"
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
	if err := pgxinfra.MigrateAtlas(pool, migrations.Content); err != nil {
		pool.Close()
		t.Fatalf("pgcontainer: migrate: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
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
