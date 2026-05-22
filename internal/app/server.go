package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"connectrpc.com/connect"
	"github.com/go-chi/chi/v5"
	"github.com/gopherex/pgtx"
	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/gopherex/xlog"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	apiadmin "github.com/stroppy-io/stroppy-cloud/internal/api/admin"
	apiagent "github.com/stroppy-io/stroppy-cloud/internal/api/agent"
	apiui "github.com/stroppy-io/stroppy-cloud/internal/api/ui"
	dagdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/dagstore"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	infs3 "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/s3"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/valkey"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/victoria"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/webhooksender"
	adminconnect "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/admin/adminconnect"
	agentconnect "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/agent/agentconnect"
	uiconnect "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui/uiconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/runtime"
	"github.com/stroppy-io/stroppy-cloud/internal/services/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/services/agentqueue"
	"github.com/stroppy-io/stroppy-cloud/internal/services/apitoken"
	"github.com/stroppy-io/stroppy-cloud/internal/services/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/services/authz"
	"github.com/stroppy-io/stroppy-cloud/internal/services/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/services/deploy"
	"github.com/stroppy-io/stroppy-cloud/internal/services/inventory"
	"github.com/stroppy-io/stroppy-cloud/internal/services/logs"
	"github.com/stroppy-io/stroppy-cloud/internal/services/metrics"
	"github.com/stroppy-io/stroppy-cloud/internal/services/netinventory"
	"github.com/stroppy-io/stroppy-cloud/internal/services/packages"
	"github.com/stroppy-io/stroppy-cloud/internal/services/platform"
	"github.com/stroppy-io/stroppy-cloud/internal/services/run"
	"github.com/stroppy-io/stroppy-cloud/internal/services/settings"
	"github.com/stroppy-io/stroppy-cloud/internal/services/share"
	"github.com/stroppy-io/stroppy-cloud/internal/services/suite"
	"github.com/stroppy-io/stroppy-cloud/internal/services/tasks"
	"github.com/stroppy-io/stroppy-cloud/internal/services/tenancy"
	"github.com/stroppy-io/stroppy-cloud/internal/services/webhook"
	"github.com/stroppy-io/stroppy-cloud/internal/services/yandexcloud"
)

// Server is the assembled control plane: a Connect HTTP API (which also speaks gRPC
// and gRPC-Web over h2c, so the CLI/agent gRPC clients keep working) plus the
// background dag processor. It also serves the web SPA and reverse-proxies Grafana
// + VictoriaMetrics + VictoriaLogs. Run blocks serving; Close releases resources.
type Server struct {
	httpSrv   *http.Server
	processor *runtime.DagProcessor
	pool      *pgxpool.Pool
	log       *xlog.Logger
}

// BuildServer wires every infrastructure client, service, api handler, the dag
// processor and the agent queue into a ready-to-serve control plane.
func BuildServer(ctx context.Context, cfg *Config, logger *xlog.Logger) (*Server, error) {
	// ── infrastructure ──────────────────────────────────────────────────────
	pool, err := pgxpool.New(ctx, pgDSN(&cfg.Postgres))
	if err != nil {
		return nil, fmt.Errorf("postgres connect: %w", err)
	}
	if err := applyMigrations(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}
	executor := pgtx.NewTxDB(pool)
	txm, err := pgtx.NewTxManager(pool, tx.ReadCommitted())
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("tx manager: %w", err)
	}

	vk, _, err := valkey.NewValkey(&cfg.Valkey, logger)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("valkey: %w", err)
	}

	vl := victoria.NewLogsClient(cfg.VictoriaLogsURL, cfg.VictoriaToken)
	vm := victoria.NewClient(cfg.VictoriaMetricsURL, cfg.VictoriaToken)
	s3Client, _ := infs3.NewClient(&cfg.S3, logger)
	uploader := infs3.NewUploader(s3Client, cfg.S3.Bucket)
	tfRunner := terraform.New(logger)
	sender := webhooksender.New()

	// ── stores + domain ─────────────────────────────────────────────────────
	dagStore := dagstore.New(logger, executor)
	netInv := netinventory.New(logger, executor, 24*time.Hour)
	yc := yandexcloud.New(logger, executor, netInv)
	platformSvc := platform.New(logger, executor, txm)
	provider := deploy.New(logger, executor, netInv, platformSvc, cfg)
	plannerImpl := dagdomain.New()

	// ── core services ───────────────────────────────────────────────────────
	az := authz.New(logger, executor)
	authSvc := auth.New(logger, executor, vk, cfg)
	logsSvc := logs.New(logger, vl)
	metricsSvc := metrics.New(logger, vm)
	shareSvc := share.New(logger, vk, metricsSvc, cfg)

	runSvc := run.New(logger, executor, txm, az, plannerImpl, provider, dagStore, logsSvc, metricsSvc, shareSvc)
	suiteSvc := suite.New(logger, executor, txm, az, plannerImpl, provider, dagStore)
	presetSvc := catalog.NewPresetService(logger, executor, txm, az)
	tenantSvc := tenancy.NewTenantService(logger, executor, txm, az)
	packageSvc := packages.NewPackageService(logger, executor, txm, az, uploader)
	settingsSvc := settings.NewSettingsService(logger, executor, txm, az)
	inventorySvc := inventory.NewCloudInventoryService(logger, executor, az, yc)
	apiTokenSvc := apitoken.NewApiTokenService(logger, executor, txm, az)
	webhookSvc := webhook.NewWebhookService(logger, executor, txm, az, sender)
	accountAdminSvc := tenancy.NewAccountAdminService(logger, executor, txm)
	tenantAdminSvc := tenancy.NewTenantAdminService(logger, executor, txm)

	queue := agentqueue.New(logger, dagStore)
	agentSvc := agent.New(logger, executor, txm, az, queue, logsSvc)

	// ── dag processor (orchestration) ────────────────────────────────────────
	taskReg := tasks.NewRegistry(logger, tfRunner)
	processor := runtime.NewDagProcessor(
		dagStore, taskReg,
		runtime.WithProcessorPredicates(runtime.DefaultPredicates()),
		runtime.WithProcessorInterval(time.Second),
		runtime.WithTerminalHook(run.WebhookTerminalHook(webhookSvc)),
	)

	// ── connect API (also serves grpc + grpc-web over h2c) ────────────────────
	opts := connect.WithInterceptors(newConnectAuthInterceptor(authSvc))
	mux := chi.NewRouter()
	mount := func(path string, h http.Handler) { mux.Handle(path+"*", h) }

	mount(uiconnect.NewAuthServiceHandler(apiui.NewAuthService(logger, authSvc), opts))
	mount(uiconnect.NewRunServiceHandler(apiui.NewRunService(logger, runSvc), opts))
	mount(uiconnect.NewSuiteServiceHandler(apiui.NewSuiteService(logger, suiteSvc), opts))
	mount(uiconnect.NewPresetServiceHandler(apiui.NewPresetService(logger, presetSvc), opts))
	mount(uiconnect.NewTenantServiceHandler(apiui.NewTenantService(logger, tenantSvc), opts))
	mount(uiconnect.NewPackageServiceHandler(apiui.NewPackageService(logger, packageSvc), opts))
	mount(uiconnect.NewSettingsServiceHandler(apiui.NewSettingsService(logger, settingsSvc), opts))
	mount(uiconnect.NewCloudInventoryServiceHandler(apiui.NewCloudInventoryService(logger, inventorySvc), opts))
	mount(uiconnect.NewApiTokenServiceHandler(apiui.NewApiTokenService(logger, apiTokenSvc), opts))
	mount(uiconnect.NewWebhookServiceHandler(apiui.NewWebhookService(logger, webhookSvc), opts))
	mount(adminconnect.NewAccountAdminServiceHandler(apiadmin.NewAccountAdminService(logger, accountAdminSvc), opts))
	mount(adminconnect.NewTenantAdminServiceHandler(apiadmin.NewTenantAdminService(logger, tenantAdminSvc), opts))
	mount(adminconnect.NewPlatformAdminServiceHandler(apiadmin.NewPlatformAdminService(logger, platformSvc), opts))
	mount(agentconnect.NewAgentServiceHandler(apiagent.NewAgentService(logger, agentSvc), opts))

	// ── reverse proxies (embedded Grafana + Victoria metrics/logs queries) ────
	if cfg.GrafanaURL != "" {
		mux.Handle("/grafana/*", reverseProxy(cfg.GrafanaURL, "/grafana"))
	}
	if cfg.VictoriaMetricsURL != "" {
		mux.Handle("/vm/*", reverseProxy(cfg.VictoriaMetricsURL, "/vm"))
	}
	if cfg.VictoriaLogsURL != "" {
		mux.Handle("/vl/*", reverseProxy(cfg.VictoriaLogsURL, "/vl"))
	}

	mux.Get("/health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.Get("/agent/binary", agentBinaryHandler())
	mountSPA(mux) // catch-all; no-op when the SPA is not embedded

	handler := h2c.NewHandler(mux, &http2.Server{})
	httpSrv := &http.Server{Addr: cfg.GRPCAddr, Handler: handler}

	return &Server{httpSrv: httpSrv, processor: processor, pool: pool, log: logger}, nil
}

// reverseProxy proxies <prefix>/* to target, stripping the prefix.
func reverseProxy(target, prefix string) http.Handler {
	u, err := url.Parse(target)
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "bad proxy target", http.StatusInternalServerError)
		})
	}
	return http.StripPrefix(prefix, httputil.NewSingleHostReverseProxy(u))
}

// agentBinaryHandler serves the agent binary that cloud-init downloads from
// <server_addr>/agent/binary. The file path is configured via STROPPY_AGENT_BINARY_PATH.
//
// TODO(agent-binary): building/embedding the agent binary into the server image is
// out of scope; until a path is configured this returns 501.
func agentBinaryHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := os.Getenv("STROPPY_AGENT_BINARY_PATH")
		if path == "" {
			http.Error(w, "agent binary not configured (set STROPPY_AGENT_BINARY_PATH)", http.StatusNotImplemented)
			return
		}
		http.ServeFile(w, r, path)
	}
}

// Run starts the dag processor and serves the API until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	if err := s.processor.Start(ctx); err != nil {
		return fmt.Errorf("start processor: %w", err)
	}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.httpSrv.Shutdown(shutCtx)
	}()
	s.log.Info("stroppy-cloud serving", xlog.String("addr", s.httpSrv.Addr))
	if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Close releases the processor + db pool.
func (s *Server) Close() {
	s.processor.Stop()
	s.pool.Close()
}

func pgDSN(c *postgres.Config) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		c.Username, c.Password, c.Host, c.Port, c.Database)
}
