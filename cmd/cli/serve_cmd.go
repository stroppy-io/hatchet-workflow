package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/stroppy-io/stroppy-cloud/internal/app"
)

// serveCmd boots the full control plane: postgres (gorm) store, the connect API
// for all services, the Temporal server worker, and the agent gateway (temporal
// gRPC proxy + binary/artifact cache + apt relay + the connect API + SPA), all on
// one listener.
func serveCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the control-plane server (connect API + Temporal worker + agent gateway)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := app.Config{
				DatabaseURL:          env("DATABASE_URL", "postgres://stroppy:stroppy@localhost:5432/stroppy?sslmode=disable"),
				JWTSecret:            env("JWT_SECRET", "stroppy-dev-secret"),
				TemporalHostPort:     env("TEMPORAL_HOSTPORT", "127.0.0.1:7233"),
				TemporalNS:           env("TEMPORAL_NAMESPACE", "default"),
				ListenAddr:           env("LISTEN_ADDR", addr),
				AgentServerAddr:      env("AGENT_SERVER_ADDR", "http://host.docker.internal:8080"),
				AttachNetwork:        os.Getenv("AGENT_ATTACH_NETWORK"),
				AgentImage:           env("AGENT_IMAGE", "stroppy-agent:latest"),
				AgentBinaryPath:      os.Getenv("AGENT_BINARY_PATH"),
				CacheDir:             env("STROPPY_BINARY_CACHE_DIR", "/var/lib/stroppy-cache/binaries"),
				StroppyUpstream:      env("STROPPY_UPSTREAM", "https://github.com/stroppy-io/stroppy/releases/download/v5.1.2/stroppy_linux_amd64.tar.gz"),
				StroppyGitHubRepo:    env("STROPPY_GITHUB_REPO", "stroppy-io/stroppy"),
				StroppyMinVersion:    os.Getenv("STROPPY_MIN_VERSION"),
				StroppyGitHubToken:   env("STROPPY_GITHUB_TOKEN", os.Getenv("GITHUB_TOKEN")),
				QuotaRefreshInterval: env("QUOTA_REFRESH_INTERVAL", "5m"),
				QuotaSnapshotTTL:     env("QUOTA_SNAPSHOT_TTL", "5m"),
				QuotaReservationTTL:  env("QUOTA_RESERVATION_TTL", "30m"),
				AptBackend:           os.Getenv("STROPPY_APT_CACHE_BACKEND"),
				MonitoringURL:        os.Getenv("MONITORING_URL"),
				MonitoringToken:      os.Getenv("MONITORING_TOKEN"),
				GrafanaBackend:       os.Getenv("GRAFANA_BACKEND"),
				PackageBlobDir:       env("STROPPY_PACKAGE_DIR", "/var/lib/stroppy-cache/packages"),
				AdminEmail:           env("STROPPY_ADMIN_EMAIL", "admin@stroppy.local"),
				AdminPassword:        os.Getenv("STROPPY_ADMIN_PASSWORD"),
			}

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			return app.Run(ctx, cfg)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", ":8080", "listen address")
	return cmd
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
