//go:build integration

package integration

import (
	"context"
	"fmt"
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
func mysqlConfigBootCase(t *testing.T, image, client string, db *domain.Database, memMB int) {
	t.Helper()
	ctx := context.Background()

	cfg, err := render.RenderDatabase(db, memMB)
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
				ContainerFilePath: "/etc/mysql/mysql.conf.d/zz-stroppy.cnf",
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
	for _, ver := range []string{"8.0", "8.4"} {
		for _, mem := range []int{2048, 16384} {
			t.Run(fmt.Sprintf("mysql%s_%dMB", ver, mem), func(t *testing.T) {
				t.Parallel()
				mysqlConfigBootCase(t, "mysql:"+ver, "mysql",
					&domain.Database{Kind: domain.Database_KIND_MYSQL, Version: ver}, mem)
			})
		}
	}
}

func TestMariaDBSingleConfigBoots(t *testing.T) {
	t.Parallel()
	for _, ver := range []string{"10.11", "11.4"} {
		for _, mem := range []int{2048, 16384} {
			t.Run(fmt.Sprintf("mariadb%s_%dMB", ver, mem), func(t *testing.T) {
				t.Parallel()
				mysqlConfigBootCase(t, "mariadb:"+ver, "mariadb",
					&domain.Database{Kind: domain.Database_KIND_MARIADB, Version: ver}, mem)
			})
		}
	}
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
