package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	temporalclient "go.temporal.io/sdk/client"
	temporallog "go.temporal.io/sdk/log"
	temporalworker "go.temporal.io/sdk/worker"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/web"

	agentworker "github.com/stroppy-io/stroppy-cloud/internal/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/api"
	"github.com/stroppy-io/stroppy-cloud/internal/gateway"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"github.com/stroppy-io/stroppy-cloud/internal/workflows"
)

var (
	configFile string
	dbPath     string
)

func main() {
	root := &cobra.Command{
		Use:           "stroppy-cloud",
		Short:         "Database testing orchestrator",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVarP(&configFile, "config", "c", "run.json", "path to run config JSON")
	root.PersistentFlags().StringVar(&dbPath, "db", "", "PostgreSQL DSN (e.g. postgres://stroppy:stroppy@localhost:5432/stroppy?sslmode=disable)")

	root.AddCommand(
		serveCmd(),
		runCmd(),
		validateCmd(),
		dryRunCmd(),
		agentCmd(),
		cloudCmd(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func serveCmd() *cobra.Command {
	var addr string
	var jwtSecret string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP server (agent API + external API + WS)",
		RunE: func(cmd *cobra.Command, args []string) error {
			dbDSN := envOrDefault("DATABASE_URL", dbPath)
			if dbDSN == "" {
				dbDSN = "postgres://stroppy:stroppy@localhost:5432/stroppy?sslmode=disable"
			}

			jwtSec := envOrDefault("JWT_SECRET", jwtSecret)
			if jwtSec == "" {
				jwtSec = "stroppy-dev-secret"
			}

			monitoringURL := os.Getenv("MONITORING_URL")     // empty = monitoring disabled
			monitoringToken := os.Getenv("MONITORING_TOKEN") // bearer token for vmauth
			grafanaURL := os.Getenv("GRAFANA_URL")           // empty = grafana disabled
			listenAddr := envOrDefault("LISTEN_ADDR", addr)

			ctx := context.Background()
			pool, err := postgres.Open(ctx, dbDSN)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer pool.Close()

			logger, _ := zap.NewDevelopment()

			// One-shot migration: round any pre-existing io-m3 disk sizes in
			// presets to a 93 GiB multiple so the YC API stops rejecting
			// runs that hit those sizes. Idempotent.
			if err := postgres.MigrateIOM3Presets(ctx, pool, logger); err != nil {
				logger.Warn("io-m3 preset migration failed (non-fatal)", zap.Error(err))
			}

			// --- Temporal: runs execute as workflows now (no DAG/scheduler) ---
			slogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			temporalHostPort := envOrDefault("TEMPORAL_HOSTPORT", "127.0.0.1:7233")
			temporalNS := envOrDefault("TEMPORAL_NAMESPACE", "default")
			tc, err := temporalclient.Dial(temporalclient.Options{
				HostPort:  temporalHostPort,
				Namespace: temporalNS,
				Logger:    temporallog.NewStructuredLogger(slogger),
			})
			if err != nil {
				return fmt.Errorf("dial temporal: %w", err)
			}
			defer tc.Close()

			app := api.New(api.Config{Pool: pool, Logger: logger})
			app.SetTemporal(tc)
			srv := api.NewServer(app, logger, pool, jwtSec, monitoringURL, monitoringToken, grafanaURL, listenAddr)

			// Docker deployer for the deploy activity. AttachNetwork joins agents
			// to the server's docker network so they reach the gateway in-network
			// (a host-gateway hairpin breaks the agent's gRPC Temporal connection
			// through docker's userland proxy).
			deployer, err := agent.NewDockerDeployer("")
			if err != nil {
				return fmt.Errorf("docker deployer: %w", err)
			}
			deployer.AttachNetwork = os.Getenv("AGENT_ATTACH_NETWORK")
			agentServerAddr := envOrDefault("AGENT_SERVER_ADDR", "http://host.docker.internal:8080")

			// Server worker: RunWorkflow + deploy/buildRecipe/teardown activities
			// on the shared "stroppy-cloud" task queue.
			w := temporalworker.New(tc, "stroppy-cloud", temporalworker.Options{})
			workflows.RegisterServer(w, &workflows.ServerActivities{
				Deployer:        deployer,
				SettingsFunc:    app.SettingsFunc(), // per-tenant cloud creds (YC etc)
				JWTIssuer:       app.JWTIssuer(),    // agent tokens for provisioned VMs
				ServerAddr:      agentServerAddr,
				MonitoringURL:   monitoringURL,
				MonitoringToken: monitoringToken,
				Logger:          logger,
			})
			if err := w.Start(); err != nil {
				return fmt.Errorf("start temporal worker: %w", err)
			}
			defer w.Stop()

			// Embed SPA into the server.
			spaFS, err := fs.Sub(web.Dist, "dist")
			if err == nil {
				srv.SetSPA(spaFS)
				logger.Info("SPA embedded and served at /")
			}

			// Agent gateway: ONE port serves everything — the Temporal gRPC proxy
			// + agent binary/artifact cache/apt relay (agent-facing) AND the
			// control-plane UI + REST API (via HTTPFallback = the chi router). So
			// the frontend stays on the same address as before.
			gw, err := gateway.New(gateway.Config{
				TemporalHostPort:  temporalHostPort,
				AgentBinaryPath:   os.Getenv("AGENT_BINARY_PATH"),
				CacheDir:          envOrDefault("STROPPY_BINARY_CACHE_DIR", "/var/lib/stroppy-cache/binaries"),
				Artifacts:         map[string]string{"stroppy": os.Getenv("STROPPY_UPSTREAM")},
				AptBackend:        os.Getenv("STROPPY_APT_CACHE_BACKEND"),
				MonitoringBackend: monitoringURL, // relay agent /insert/* → vmauth for cloud VMs
				HTTPFallback:      srv.Router(),
				Logger:            slogger,
			})
			if err != nil {
				return fmt.Errorf("build gateway: %w", err)
			}
			gwAddr := envOrDefault("AGENT_GATEWAY_ADDR", listenAddr)
			gwLis, err := net.Listen("tcp", gwAddr)
			if err != nil {
				return fmt.Errorf("listen gateway %s: %w", gwAddr, err)
			}
			go func() {
				logger.Info("server listening (gateway + UI + API)", zap.String("addr", gwAddr))
				if err := gw.Serve(gwLis); err != nil {
					logger.Error("gateway serve error", zap.Error(err))
				}
			}()

			// Optional: also expose the plain control-plane API on a second port
			// (e.g. for the e2e gRPC/HTTP client) when API_ADDR is set.
			if apiAddr := os.Getenv("API_ADDR"); apiAddr != "" && apiAddr != gwAddr {
				httpSrv := &http.Server{Addr: apiAddr, Handler: srv.Router()}
				go func() {
					logger.Info("api server listening", zap.String("addr", apiAddr))
					if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
						logger.Error("api server error", zap.Error(err))
					}
				}()
			}

			sigCtx := signalCtx()
			<-sigCtx.Done()
			logger.Info("shutting down")
			gw.Close()
			return nil
		},
	}
	cmd.Flags().StringVar(&addr, "addr", ":8080", "listen address")
	cmd.Flags().StringVar(&jwtSecret, "jwt-secret", "", "JWT signing secret (default: stroppy-dev-secret)")
	return cmd
}

func runCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Execute a full test run from config (local, no server)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := api.LoadConfig(configFile)
			if err != nil {
				return err
			}

			dbDSN := envOrDefault("DATABASE_URL", dbPath)
			if dbDSN == "" {
				return fmt.Errorf("--db flag or DATABASE_URL env is required (PostgreSQL DSN)")
			}

			ctx := signalCtx()
			pool, err := postgres.Open(ctx, dbDSN)
			if err != nil {
				return err
			}
			defer pool.Close()

			logger, _ := zap.NewDevelopment()
			app := api.New(api.Config{Pool: pool, Logger: logger})

			// CLI runs use an empty tenant ID.
			return app.Start(ctx, "", cfg)
		},
	}
}

func validateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate a run config without executing",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := api.LoadConfig(configFile)
			if err != nil {
				return err
			}

			dbDSN := envOrDefault("DATABASE_URL", dbPath)
			if dbDSN == "" {
				return fmt.Errorf("--db flag or DATABASE_URL env is required (PostgreSQL DSN)")
			}

			ctx := context.Background()
			pool, err := postgres.Open(ctx, dbDSN)
			if err != nil {
				return err
			}
			defer pool.Close()

			logger, _ := zap.NewDevelopment()
			app := api.New(api.Config{Pool: pool, Logger: logger})

			if err := app.Validate(cfg); err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}
			fmt.Println("config is valid")
			return nil
		},
	}
}

func dryRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dry-run",
		Short: "Print the execution DAG as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := api.LoadConfig(configFile)
			if err != nil {
				return err
			}

			dbDSN := envOrDefault("DATABASE_URL", dbPath)
			if dbDSN == "" {
				return fmt.Errorf("--db flag or DATABASE_URL env is required (PostgreSQL DSN)")
			}

			ctx := context.Background()
			pool, err := postgres.Open(ctx, dbDSN)
			if err != nil {
				return err
			}
			defer pool.Close()

			logger, _ := zap.NewDevelopment()
			app := api.New(api.Config{Pool: pool, Logger: logger})

			data, _, err := app.DryRun(cfg)
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		},
	}
}

func agentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Run in agent mode on a target machine (a Temporal worker reached via the server's gRPC proxy)",
		RunE: func(cmd *cobra.Command, args []string) error {
			// The agent is told ONLY the server address (a full URL). Temporal is
			// reached through the gateway's transparent gRPC proxy at that same
			// address, so dialing the server IS dialing Temporal — but the gRPC
			// client wants a bare host:port, so strip the scheme. The agent runs
			// its activities on its OWN task queue so the workflow can pin this
			// machine's steps to exactly this agent (deterministic placement).
			serverAddr := envOr("STROPPY_SERVER_ADDR", "http://127.0.0.1:8080")
			namespace := envOr("TEMPORAL_NAMESPACE", "default")
			// The agent listens on a per-machine queue ("stroppy-agent-<machineID>")
			// so the workflow can pin this machine's steps to exactly this agent
			// (workflows.AgentQueue). Docker sets AGENT_TASK_QUEUE explicitly; cloud
			// VMs only get STROPPY_MACHINE_ID via cloud-init, so derive the queue
			// from it when AGENT_TASK_QUEUE is unset. A bare "stroppy-agent" (no
			// machine id) would never match the workflow's session queue.
			taskQueue := os.Getenv("AGENT_TASK_QUEUE")
			if taskQueue == "" {
				if machineID := os.Getenv("STROPPY_MACHINE_ID"); machineID != "" {
					taskQueue = "stroppy-agent-" + machineID
				} else {
					taskQueue = "stroppy-agent"
				}
			}
			hostPort := grpcHostPort(serverAddr)

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			logger.Info("starting stroppy agent",
				"server_addr", serverAddr, "temporal_hostport", hostPort,
				"namespace", namespace, "task_queue", taskQueue)

			c, err := temporalclient.Dial(temporalclient.Options{
				HostPort:  hostPort,
				Namespace: namespace,
				Logger:    temporallog.NewStructuredLogger(logger),
			})
			if err != nil {
				return fmt.Errorf("agent: dial temporal: %w", err)
			}
			defer c.Close()

			// EnableSessionWorker lets the orchestrating workflow pin a sequence of
			// activities (write configs, fetch binaries, run the load) to THIS
			// agent via a Temporal worker session.
			w := temporalworker.New(c, taskQueue, temporalworker.Options{EnableSessionWorker: true})
			impl := agentworker.NewActivities(agentworker.WithLogger(logger))
			workflowpb.RegisterAgentCommandServiceActivities(w, impl)

			return w.Run(temporalworker.InterruptCh())
		},
	}
	return cmd
}

// envOr returns the env value for key or def when unset/empty.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// grpcHostPort strips a URL scheme (and path) from the server address so the
// Temporal gRPC client gets a bare host:port. "http://host:8080" -> "host:8080".
func grpcHostPort(serverAddr string) string {
	if u, err := url.Parse(serverAddr); err == nil && u.Host != "" {
		return u.Host
	}
	return serverAddr
}

func signalCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		cancel()
	}()
	return ctx
}
