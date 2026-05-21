//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestCockroachSingleStarts validates the cockroach single-node start path (the
// recipe starts it via flags, not a config file): the official image comes up as
// a single node and answers SQL. Mirrors the planner's cockroach startScript.
func TestCockroachSingleStarts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "cockroachdb/cockroach:v24.2.0",
			Cmd:   []string{"start-single-node", "--insecure"},
			WaitingFor: wait.ForLog("CockroachDB node starting").
				WithStartupTimeout(120 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err, "cockroach must start single-node")
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	var out string
	ok := false
	for range 20 {
		code, reader, execErr := container.Exec(ctx, []string{
			"cockroach", "sql", "--insecure", "-e", "SELECT 1",
		})
		if execErr == nil && code == 0 {
			ok = true
			break
		}
		out = readAll(reader)
		time.Sleep(2 * time.Second)
	}
	require.True(t, ok, "cockroach query never succeeded: %s", out)
}
