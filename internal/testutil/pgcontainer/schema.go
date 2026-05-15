package pgcontainer

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
)

func IsolatedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := Shared(t)
	schema := "t_" + strings.ToLower(ids.New())

	root, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgcontainer: root pool: %v", err)
	}
	defer root.Close()

	if _, err := root.Exec(context.Background(), fmt.Sprintf(`CREATE SCHEMA %q`, schema)); err != nil {
		t.Fatalf("pgcontainer: create schema: %v", err)
	}

	cfg, _ := pgxpool.ParseConfig(dsn)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("pgcontainer: scoped pool: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		cleanup, _ := pgxpool.New(context.Background(), dsn)
		defer cleanup.Close()
		_, _ = cleanup.Exec(context.Background(), fmt.Sprintf(`DROP SCHEMA %q CASCADE`, schema))
	})

	return pool
}
