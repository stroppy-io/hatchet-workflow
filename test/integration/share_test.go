//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/gopherex/xlog"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/valkey"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	metricspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/metrics"
	"github.com/stroppy-io/stroppy-cloud/internal/services/share"
)

// fakeShareConfig satisfies share.Config (the public base URL for links).
type fakeShareConfig struct{ base string }

func (c fakeShareConfig) ShareBaseURL() string { return c.base }

// fakeMetricsFetcher satisfies share.MetricsFetcher. It echoes the requested
// runID back as the snapshot's RunId so the test can assert metrics are frozen
// alongside the run.
type fakeMetricsFetcher struct{}

func (fakeMetricsFetcher) GetRunMetrics(_ context.Context, runID string) (*metricspb.RunMetrics, error) {
	return &metricspb.RunMetrics{RunId: runID}, nil
}

// TestShareStoreRoundTrip exercises the REAL share.Store against a real Valkey
// (valkey.NewValkey -> CreateShareLink -> GetSharedRun). It proves a created
// share token persists in Valkey, returns a URL with the configured base, the
// frozen snapshot round-trips (run id + metrics), and an unknown token yields a
// not-found error.
func TestShareStoreRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "valkey/valkey:8-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor: wait.ForListeningPort(nat.Port("6379/tcp")).
				WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err, "valkey must start")
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	host, err := container.Host(ctx)
	require.NoError(t, err)
	mappedPort, err := container.MappedPort(ctx, nat.Port("6379/tcp"))
	require.NoError(t, err)
	addr := fmt.Sprintf("%s:%s", host, mappedPort.Port())

	logger := xlog.NewConsole()

	// Real valkey client. The valkey/valkey:8-alpine image runs without auth, so
	// the password must be empty: a non-empty password makes valkey-go send AUTH,
	// which the server rejects (closing the connection -> EOF) when no password
	// is configured. The Config's validate:"required" tag is only enforced by an
	// external validator, not by NewValkey, so an empty value is fine here.
	vk, probe, err := valkey.NewValkey(&valkey.Config{
		Mode:      valkey.ModeStandalone,
		Addresses: []string{addr},
		Password:  "",
	}, logger)
	require.NoError(t, err, "valkey client must build")
	t.Cleanup(vk.Close)
	require.True(t, probe.Check(ctx).OK(), "valkey ping must report Up")

	store := share.New(logger, vk, fakeMetricsFetcher{}, fakeShareConfig{base: "https://share.stroppy.io/r/"})

	run := &models.TestRun{
		Entity: &models.Entity{Id: &models.Ulid{Value: "01J0RUNID000000000000000A"}},
		Name:   strptr("integration share run"),
	}

	// CreateShareLink -> non-empty token + url with the configured base.
	token, url, err := store.CreateShareLink(ctx, run)
	require.NoError(t, err)
	require.NotEmpty(t, token, "token must be non-empty")
	require.Equal(t, "https://share.stroppy.io/r/"+token, url, "url must be base + token")

	// GetSharedRun -> frozen snapshot round-trips.
	snap, err := store.GetSharedRun(ctx, token)
	require.NoError(t, err)
	require.NotNil(t, snap.GetRun())
	require.Equal(t, run.GetEntity().GetId().GetValue(),
		snap.GetRun().GetEntity().GetId().GetValue(), "run id must round-trip")
	require.Equal(t, "integration share run", snap.GetRun().GetName())
	// Metrics were snapshotted alongside the run.
	require.NotNil(t, snap.GetMetrics())
	require.Equal(t, run.GetEntity().GetId().GetValue(), snap.GetMetrics().GetRunId(),
		"frozen metrics must reference the same run id")

	// GetSharedRun for an unknown token -> not-found error.
	_, err = store.GetSharedRun(ctx, "this-token-does-not-exist")
	require.Error(t, err, "unknown token must error")
	require.Contains(t, err.Error(), "not found")
}

func strptr(s string) *string { return &s }
