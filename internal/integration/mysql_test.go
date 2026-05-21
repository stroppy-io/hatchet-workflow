//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/render"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

// mysqlConfigBootCase renders the single config for a mysql-family engine, drops
// it into the image's conf.d, and asserts the server boots with it and answers a
// query (run via the image's own client, so no Go driver dependency).
func mysqlConfigBootCase(t *testing.T, image, client string, db *domain.Database) {
	t.Helper()
	ctx := context.Background()

	cfg, err := render.RenderDatabase(db, 4096)
	require.NoError(t, err)
	confPath := writeConfigItem(t, cfg, "my.cnf")

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: image,
			Env: map[string]string{
				"MYSQL_ROOT_PASSWORD": "stroppy",
				"MYSQL_DATABASE":      "stroppy",
			},
			Files: []testcontainers.ContainerFile{{
				HostFilePath:      confPath,
				ContainerFilePath: "/etc/mysql/conf.d/stroppy.cnf",
				FileMode:          0o644,
			}},
			WaitingFor: wait.ForLog("ready for connections").
				WithOccurrence(2).WithStartupTimeout(120 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err, "%s must boot with the rendered my.cnf", image)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	// The container is up with our config (config is valid). Root auth is set up a
	// moment after the first "ready for connections", so retry the query.
	var out string
	ok := false
	for range 20 {
		code, reader, execErr := container.Exec(ctx, []string{client, "-uroot", "-pstroppy", "-e", "SELECT 1"})
		if execErr == nil && code == 0 {
			ok = true
			break
		}
		out = readAll(reader)
		time.Sleep(2 * time.Second)
	}
	require.True(t, ok, "mysql query never succeeded: %s", out)
}

func TestMySQLSingleConfigBoots(t *testing.T) {
	t.Parallel()
	mysqlConfigBootCase(t, "mysql:8.4", "mysql", &domain.Database{Kind: domain.Database_KIND_MYSQL, Version: "8.4"})
}

func TestMariaDBSingleConfigBoots(t *testing.T) {
	t.Parallel()
	mysqlConfigBootCase(t, "mariadb:11.4", "mariadb", &domain.Database{Kind: domain.Database_KIND_MARIADB, Version: "11.4"})
}

// readAll drains an exec output reader (used only for diagnostics on failure).
func readAll(r interface{ Read([]byte) (int, error) }) string {
	var b strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		b.Write(buf[:n])
		if err != nil {
			break
		}
	}
	return b.String()
}
