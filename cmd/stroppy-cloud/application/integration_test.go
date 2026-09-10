//go:build integration

// Integration tests run the assembled application against a real Postgres
// (testcontainers) and a fake Graphene door served in-process. Run with
// `make test-db` (go test -tags=integration ./cmd/... -p 1). Excluded from
// the default `go test ./...`.
package application

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gopherex/xlog"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
)

// testDB is the shared, migrated connection to the throwaway Postgres. Tests
// isolate by fresh uuids/slugs, so one database serves the package.
var (
	testDB  *postgres.Client
	testLog = xlog.NewConsole(xlog.WithLevel(xlog.WarnLevel), xlog.WithWriter(os.Stderr))
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("stroppy"),
		tcpostgres.WithUsername("stroppy"),
		tcpostgres.WithPassword("stroppy"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(90*time.Second)),
	)
	if err != nil {
		panic("start postgres container: " + err.Error())
	}
	host, err := container.Host(ctx)
	if err != nil {
		panic("container host: " + err.Error())
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		panic("container port: " + err.Error())
	}
	portNum, err := strconv.Atoi(port.Port())
	if err != nil {
		panic("container port: " + err.Error())
	}
	cfg := postgres.Config{Host: host, Port: portNum, User: "stroppy", Password: "stroppy", Database: "stroppy", SSLMode: "disable", QueryLogLevel: "error"}
	db, err := postgres.New(ctx, &cfg, testLog)
	if err != nil {
		panic("connect: " + err.Error())
	}
	if err := db.Migrate(ctx); err != nil {
		panic("migrate: " + err.Error())
	}
	testDB = db

	code := m.Run()

	db.Close()
	_ = container.Terminate(ctx)
	os.Exit(code)
}
