//go:build integration

// Package integration runs the DB-engine matrix: it validates that the configs +
// recipes the planner/render layer produce actually work against real database
// engines in Docker, across every supported engine and cluster form. This file is
// the config-validation level (the rendered config boots the official engine image
// + accepts a connection); the full systemd-pipeline level lives alongside it.
package integration

import (
	"context"
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

// TestPostgresSingleConfigBoots renders the single-postgres config and verifies a
// real postgres:16 boots with it and accepts connections — i.e. the rendered
// postgresql.conf is valid for the engine.
func TestPostgresSingleConfigBoots(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cfg, err := render.RenderDatabase(&domain.Database{Kind: domain.Database_KIND_POSTGRES, Version: "16"}, 4096)
	require.NoError(t, err)
	confPath := writeConfigItem(t, cfg, "postgresql.conf")

	container, err := pgmodule.Run(ctx, "postgres:16-alpine",
		pgmodule.WithConfigFile(confPath),
		pgmodule.WithDatabase("stroppy"),
		pgmodule.WithUsername("stroppy"),
		pgmodule.WithPassword("stroppy"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err, "postgres must boot with the rendered config")
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()

	var one int
	require.NoError(t, pool.QueryRow(ctx, "SELECT 1").Scan(&one))
	require.Equal(t, 1, one)
}
