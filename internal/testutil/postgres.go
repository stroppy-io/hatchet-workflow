// Package testutil provides the integration-test harness: a throwaway PostgreSQL
// testcontainer with the full ratel schema applied to a template database, cloned
// per test (~10ms) for isolation. Mirrors komeet-backend/internal/testutil.
package testutil

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	pgmodule "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/yaroher/ratel/pkg/ddl"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

const (
	templateDBName = "stroppy_template"
	testDBPrefix   = "test_"
)

const maxConcurrentClones = 8

// schemaTables are every ratel table, in no particular order; SchemaSortedStatements
// resolves the FK dependency order. Add new model tables here.
func schemaTables() []ddl.SchemaSqler {
	return []ddl.SchemaSqler{
		models.Tenants, models.Accounts, models.TenantMembers,
		models.SettingsItems, models.Webhooks, models.ApiTokens,
		models.Packages, models.Presets, models.Suites,
		models.TestRuns, models.SuiteRuns, models.Dags,
		models.NetworkAllocations, models.Agents,
	}
}

// PostgresContainer wraps a testcontainer with an admin pool + template DB support.
type PostgresContainer struct {
	*pgmodule.PostgresContainer
	Pool     *pgxpool.Pool // connects to the default "stroppy" db (admin DDL)
	connStr  string
	cloneSem chan struct{}
}

// NewPostgresContainer starts a PostgreSQL 16 container. Caller must defer Close().
func NewPostgresContainer(ctx context.Context) (*PostgresContainer, error) {
	container, err := pgmodule.Run(ctx,
		"postgres:16-alpine",
		pgmodule.WithDatabase("stroppy"),
		pgmodule.WithUsername("test"),
		pgmodule.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres container: %w", err)
	}
	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("connection string: %w", err)
	}
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &PostgresContainer{
		PostgresContainer: container,
		Pool:              pool,
		connStr:           connStr,
		cloneSem:          make(chan struct{}, maxConcurrentClones),
	}, nil
}

// CreateTemplateDB builds a template database with the full ratel schema, used by
// NewTestDB to clone per-test databases. Call once in TestMain.
func (c *PostgresContainer) CreateTemplateDB(ctx context.Context) error {
	_, _ = c.Pool.Exec(ctx, "DROP DATABASE IF EXISTS "+templateDBName)
	if _, err := c.Pool.Exec(ctx, "CREATE DATABASE "+templateDBName); err != nil {
		return fmt.Errorf("create template db: %w", err)
	}

	stmts, err := ddl.SchemaSortedStatements(schemaTables()...)
	if err != nil {
		return fmt.Errorf("build schema: %w", err)
	}
	tmplPool, err := pgxpool.New(ctx, replaceDBName(c.connStr, templateDBName))
	if err != nil {
		return fmt.Errorf("connect template db: %w", err)
	}
	for _, s := range stmts {
		if _, err := tmplPool.Exec(ctx, s); err != nil {
			tmplPool.Close()
			return fmt.Errorf("apply schema stmt %q: %w", s, err)
		}
	}
	tmplPool.Close() // PG requires 0 connections to use a db as template.

	if _, err := c.Pool.Exec(ctx, fmt.Sprintf("ALTER DATABASE %s IS_TEMPLATE true", templateDBName)); err != nil {
		return fmt.Errorf("mark template: %w", err)
	}
	if _, err := c.Pool.Exec(ctx, fmt.Sprintf("ALTER DATABASE %s ALLOW_CONNECTIONS false", templateDBName)); err != nil {
		return fmt.Errorf("disable template connections: %w", err)
	}
	return nil
}

// TestDB is an isolated database for a single test, cloned from the template.
type TestDB struct {
	Pool   *pgxpool.Pool
	DBName string
}

// NewTestDB clones a fresh database from the template and registers cleanup.
// Safe for parallel tests.
func (c *PostgresContainer) NewTestDB(t *testing.T) *TestDB {
	t.Helper()
	ctx := context.Background()
	dbName := testDBPrefix + strings.ReplaceAll(uuid.New().String(), "-", "")[:16]

	c.cloneSem <- struct{}{}
	_, err := c.Pool.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", dbName, templateDBName))
	<-c.cloneSem
	if err != nil {
		t.Fatalf("create test db %s: %v", dbName, err)
	}

	cfg, err := pgxpool.ParseConfig(replaceDBName(c.connStr, dbName))
	if err != nil {
		t.Fatalf("parse test db config: %v", err)
	}
	cfg.MaxConns = 4
	cfg.MinConns = 0
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect test db %s: %v", dbName, err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = c.Pool.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", dbName))
	})
	return &TestDB{Pool: pool, DBName: dbName}
}

// Close terminates the container.
func (c *PostgresContainer) Close(ctx context.Context) error {
	if c.Pool != nil {
		c.Pool.Close()
	}
	if c.PostgresContainer != nil {
		return c.PostgresContainer.Terminate(ctx)
	}
	return nil
}

func replaceDBName(connStr, newDB string) string {
	lastSlash := strings.LastIndex(connStr, "/")
	if lastSlash == -1 {
		return connStr + "/" + newDB
	}
	rest := connStr[lastSlash+1:]
	if qmark := strings.Index(rest, "?"); qmark >= 0 {
		return connStr[:lastSlash+1] + newDB + rest[qmark:]
	}
	return connStr[:lastSlash+1] + newDB
}
