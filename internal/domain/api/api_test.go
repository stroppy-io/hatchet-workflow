package api

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping database test")
	}
	ctx := context.Background()
	pool, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("postgres.Open() failed: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func newTestApp(t *testing.T) *App {
	t.Helper()
	pool := testPool(t)
	app := New(Config{Pool: pool})
	return app
}

func postgresSingleConfig() types.RunConfig {
	topo := types.PostgresPresets[types.PostgresSingle]
	return types.RunConfig{
		ID:       "test-run-1",
		Provider: types.ProviderDocker,
		Network:  types.NetworkConfig{CIDR: "10.0.0.0/24"},
		Database: types.DatabaseConfig{
			Kind:     types.DatabasePostgres,
			Version:  "16",
			Postgres: &topo,
		},
		Monitor: types.MonitorConfig{},
		Stroppy: types.StroppyConfig{
			Version:  "latest",
			Workload: "ycsb",
			Duration: "30s",
			Workers:  4,
		},
	}
}

func TestNew_App(t *testing.T) {
	app := newTestApp(t)
	if app == nil {
		t.Fatal("expected non-nil app")
	}
}

func TestValidate_PostgresSingle(t *testing.T) {
	app := newTestApp(t)
	cfg := postgresSingleConfig()

	if err := app.Validate(cfg); err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}
}

func TestValidate_UnsupportedKind(t *testing.T) {
	app := newTestApp(t)
	cfg := postgresSingleConfig()
	cfg.Database.Kind = "cockroach"
	cfg.Database.Postgres = nil

	if err := app.Validate(cfg); err == nil {
		t.Fatal("expected error for unsupported kind, got nil")
	}
}

func TestDryRun_ReturnsJSON(t *testing.T) {
	app := newTestApp(t)
	cfg := postgresSingleConfig()

	data, _, err := app.DryRun(cfg)
	if err != nil {
		t.Fatalf("DryRun() error: %v", err)
	}

	if !json.Valid(data) {
		t.Fatal("DryRun() returned invalid JSON")
	}

	// Verify it decodes as a graph object.
	var graph map[string]any
	if err := json.Unmarshal(data, &graph); err != nil {
		t.Fatalf("unmarshal graph: %v", err)
	}
}

func TestDryRun_ReturnsResolvedConfig(t *testing.T) {
	app := newTestApp(t)
	cfg := postgresSingleConfig()

	// DryRun no longer builds/validates a DAG (execution lives in Temporal);
	// it returns the resolved config so the review step can preview it.
	_, resolved, err := app.DryRun(cfg)
	if err != nil {
		t.Fatalf("DryRun() error: %v", err)
	}
	if resolved == nil || resolved.ID != cfg.ID {
		t.Fatalf("DryRun() resolved config mismatch: %+v", resolved)
	}
}
