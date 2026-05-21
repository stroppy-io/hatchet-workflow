//go:build integration

// Package integration runs the DB-engine matrix: it validates that the configs +
// recipes the planner/render layer produce actually work against real database
// engines in Docker, across every supported engine and cluster form. This file is
// the config-validation level (the rendered config boots the official engine image
// + accepts a connection); the full systemd-pipeline level lives alongside it.
package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	pgmodule "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/render"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	renderpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/render"
)

// writeConfigItem writes a rendered config item's file body to a temp file and
// returns the path.
func writeConfigItem(t *testing.T, cfg *renderpb.Config, itemID string) string {
	t.Helper()
	for _, it := range cfg.GetItems() {
		if it.GetId() == itemID {
			p := filepath.Join(t.TempDir(), itemID)
			if err := os.WriteFile(p, []byte(it.GetFile().GetContent().GetText()), 0o644); err != nil {
				t.Fatalf("write config: %v", err)
			}
			return p
		}
	}
	t.Fatalf("config item %q not found", itemID)
	return ""
}

// TestPostgresSingleConfigBoots renders the single-postgres config across every
// supported version and a range of memory budgets (percent params like
// shared_buffers exercised small→large) and verifies a real postgres boots with
// each rendered postgresql.conf and accepts connections.
func TestPostgresSingleConfigBoots(t *testing.T) {
	t.Parallel()
	cases := []struct {
		version string
		image   string
		memMB   int
	}{
		{"16", "postgres:16-alpine", 2048},
		{"16", "postgres:16-alpine", 65536},
		{"17", "postgres:17-alpine", 4096},
		{"17", "postgres:17-alpine", 32768},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("pg%s_%dMB", tc.version, tc.memMB), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			cfg, err := render.RenderDatabase(&domain.Database{Kind: domain.Database_KIND_POSTGRES, Version: tc.version}, tc.memMB)
			require.NoError(t, err)
			confPath := writeConfigItem(t, cfg, "postgresql.conf")

			container, err := pgmodule.Run(ctx, tc.image,
				pgmodule.WithConfigFile(confPath),
				pgmodule.WithDatabase("stroppy"),
				pgmodule.WithUsername("stroppy"),
				pgmodule.WithPassword("stroppy"),
				testcontainers.WithWaitStrategy(
					wait.ForLog("database system is ready to accept connections").
						WithOccurrence(2).WithStartupTimeout(60*time.Second),
				),
			)
			require.NoError(t, err, "postgres %s must boot with rendered config (mem=%dMB)", tc.version, tc.memMB)
			t.Cleanup(func() { _ = container.Terminate(ctx) })

			connStr, err := container.ConnectionString(ctx, "sslmode=disable")
			require.NoError(t, err)
			pool, err := pgxpool.New(ctx, connStr)
			require.NoError(t, err)
			defer pool.Close()

			var one int
			require.NoError(t, pool.QueryRow(ctx, "SELECT 1").Scan(&one))
			require.Equal(t, 1, one)
		})
	}
}
