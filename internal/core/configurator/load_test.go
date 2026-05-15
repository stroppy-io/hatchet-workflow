package configurator_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/core/configurator"
)

func TestLoadYAMLWithEnvSubstitution(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	body := `
server:
  http_addr: ":8080"
postgres:
  dsn: "postgres://stroppy:${PG_PASSWORD}@127.0.0.1:5432/stroppy?sslmode=disable"
  max_conns: 25
auth:
  jwt_secret_env: "JWT_SECRET"
  access_ttl: "15m"
  refresh_ttl: "720h"
workers:
  node_workers: 4
  scheduler_tick: "5s"
  webhook_workers: 2
  recovery_on_start: true
log:
  level: "info"
  format: "json"
`
	if err := os.WriteFile(cfgPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PG_PASSWORD", "secret123")

	cfg, err := configurator.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.HTTPAddr != ":8080" {
		t.Errorf("http_addr: %q", cfg.Server.HTTPAddr)
	}
	if cfg.Postgres.DSN != "postgres://stroppy:secret123@127.0.0.1:5432/stroppy?sslmode=disable" {
		t.Errorf("dsn substitution: %q", cfg.Postgres.DSN)
	}
	if cfg.Auth.AccessTTL != 15*time.Minute {
		t.Errorf("access_ttl: %v", cfg.Auth.AccessTTL)
	}
	if !cfg.Workers.RecoveryOnStart {
		t.Errorf("recovery_on_start should be true")
	}
}
