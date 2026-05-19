package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/core/build"

	"connectrpc.com/connect"
	otelconnect "connectrpc.com/otelconnect"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/configurator"
	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/logger"
	adminsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/admin"
	agentsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/iam"
	opssvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/stroppy"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	testingsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/testing"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/testing/dagbuilder"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	handlers "github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker/handlers"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/recovery"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/scheduler"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/migrations"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/s3"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/stroppybin"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
	tfmodules "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform/modules"
	valkey "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/valkey"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/victoria"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
	transportconnect "github.com/stroppy-io/stroppy-cloud/internal/transport/connect"
	"github.com/stroppy-io/stroppy-cloud/internal/transport/httpext"
	"github.com/stroppy-io/stroppy-cloud/internal/transport/middleware"
	"github.com/stroppy-io/stroppy-cloud/web"
)

func serverCmd() *cobra.Command {
	var cfgPath string
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Run the control-plane server",
		RunE: func(c *cobra.Command, _ []string) error {
			return runServer(c.Context(), cfgPath)
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", os.Getenv("CONFIG_PATH"), "path to config.yaml")
	return cmd
}

func runServer(ctx context.Context, cfgPath string) error {
	if cfgPath == "" {
		return fmt.Errorf("--config or CONFIG_PATH required")
	}
	cfg, err := configurator.Load(cfgPath)
	if err != nil {
		return err
	}

	zlog := logger.NewFromConfig(&logger.Config{
		LogMod:   logger.ProductionMod,
		LogLevel: cfg.Log.Level,
	})
	if cfg.Log.Format == "console" {
		zlog = logger.NewFromConfig(&logger.Config{
			LogMod:   logger.DevelopmentMod,
			LogLevel: cfg.Log.Level,
		})
	}
	defer zlog.Sync() //nolint:errcheck

	pool, err := postgres.New(cfg.Postgres)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := postgres.MigrateWithLock(pool, "stroppy-cloud", migrations.Content); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	exec, txMgr, err := pgtx.NewTxFlow(pool, pgtx.ReadCommittedSettings())
	if err != nil {
		return err
	}

	rawValkey, err := valkey.NewValkey(&valkey.Config{
		Addresses: []string{cfg.Valkey.Addr},
	})
	if err != nil {
		return fmt.Errorf("valkey: %w", err)
	}
	defer rawValkey.Close()
	valkeyCli := valkey.NewClient(rawValkey)

	bus := eventing.NewInMemoryBus()

	jwtSecret := []byte(os.Getenv(cfg.Auth.JWTSecretEnv))
	if len(jwtSecret) < 32 {
		return fmt.Errorf("JWT secret env %s must hold ≥32 bytes", cfg.Auth.JWTSecretEnv)
	}

	iamSvc := iam.New(exec, txMgr, bus, cfg.Auth, jwtSecret)

	s3Client, err := s3.New(ctx, cfg.S3)
	if err != nil {
		return fmt.Errorf("s3: %w", err)
	}

	stroppyRunner := stroppybin.New(cfg.Stroppy.DefaultVersion, cfg.Stroppy.BinariesDir)

	catalogSvc := catalog.New(exec, txMgr, bus)
	catalogSvc.SetPackageStorage(catalog.NewS3PackageStorage(s3Client))

	stroppySvc := stroppy.New(stroppyRunner, valkeyCli, cfg.Stroppy.ReleasesURL, cfg.Stroppy.CommitsURL)

	systemSvc := system.New(exec, txMgr, bus)

	webhookSvc := opssvc.NewWebhookService(exec, txMgr, bus)
	quotaSvc := opssvc.NewQuotaService(exec, pool, txMgr)
	binaryCacheSvc := opssvc.NewBinaryCacheService(exec, txMgr)

	tplSvc := testingsvc.NewTemplateService(exec, txMgr, bus)
	suiteSvc := testingsvc.NewTestSuiteService(exec, txMgr, bus)
	builder := dagbuilder.New(catalogSvc)
	runSvc := testingsvc.NewTestRunService(exec, txMgr, bus, catalogSvc, systemSvc, builder).WithQuota(quotaSvc)
	suiteRunSvc := testingsvc.NewTestSuiteRunService(exec, txMgr, bus, systemSvc, builder)
	sharedTestRunSvc := testingsvc.NewSharedTestRunService(exec, txMgr, bus)
	sharedSuiteRunSvc := testingsvc.NewSharedSuiteRunService(exec, txMgr, bus)
	var metricsAdapter testingsvc.MetricsPort
	if cfg.Victoria.QueryURL != "" {
		metricsAdapter = victoria.NewMetricsAdapter(
			victoria.NewClient(cfg.Victoria.QueryURL, cfg.Victoria.Token),
		)
	}
	comparisonSvc := testingsvc.NewComparisonService(metricsAdapter)
	baselineSvc := testingsvc.NewBaselineService(exec, txMgr, bus)
	// comparisonSvc.WithTestRunLister depends on runSvc constructor below;
	// wired after runSvc + suiteRunSvc are built.
	runSvc = runSvc.WithMetrics(metricsAdapter)
	comparisonSvc.WithTestRunLister(runSvc)

	adminService := adminsvc.NewAdminService(iamSvc)
	binaryCacheAdminSvc := adminsvc.NewBinaryCacheAdminService(exec, txMgr)

	agentHub := agentsvc.NewHub()
	agentCmdRepo := agentsvc.NewCommandsRepo(exec, txMgr)
	bootstrapStore := agentsvc.NewBootstrapTokenStore(jwtSecret)
	agentService := agentsvc.New(exec, txMgr, bus, agentHub, agentCmdRepo, bootstrapStore).
		WithLogIngester(systemLogAdapter{sys: systemSvc})

	if cfg.Workers.RecoveryOnStart {
		if err := recovery.Run(ctx, systemSvc, agentService, webhookSvc, zlog); err != nil {
			return fmt.Errorf("recovery: %w", err)
		}
	}

	nodeReg := nodeworker.NewRegistry()
	nodeReg.Register(handlers.NewMockHandler()) // "" kind fallback for untyped specs
	tfActor, tfActorErr := terraform.NewActor()
	if tfActorErr != nil {
		zlog.Warn("terraform actor init failed; handler will refuse non-noop runs", zap.Error(tfActorErr))
	}
	// terraform handler: embedded modules + per-machine bootstrap-token
	// issuer (so the rendered cloud-init userdata can register agents).
	nodeReg.Register(handlers.NewTerraformHandler(tfActor, tfmodules.NewResolver(), zlog, agentService))
	nodeReg.Register(handlers.NewDockerHandler("", zlog))
	// Agent-bound handlers resolve (dag_run_id, machine_id) → agent_id from
	// agentHub at execute time.
	nodeReg.Register(handlers.NewPackageInstallHandler(agentHub, 0))
	nodeReg.Register(handlers.NewStroppyRunHandler(agentHub, zlog, 0))
	nodeReg.Register(handlers.NewOneShotHandler(agentHub, 0))
	nodeReg.Register(handlers.NewConfigApplyHandler(agentHub, 0))
	nodeReg.Register(handlers.NewWaitAgentsHandler(agentHub))
	nodeReg.Register(handlers.NewTestRunRefHandler(runSvc, systemSvc, 0, 0))

	worker := nodeworker.New(pool, systemSvc, nodeReg, nodeworker.Config{
		Workers: cfg.Workers.NodeWorkers,
		Tick:    500 * time.Millisecond,
	}, zlog)
	// Schedule hooks: every cron-triggered Schedule fire dispatches to a hook
	// here. Currently registered hooks:
	//   "launch_test_run" — payload = testing.TestRunId; calls runSvc.LaunchTestRun.
	//   "launch_test_suite" — payload = testing.TestSuiteId; calls suiteRunSvc.LaunchTestSuite.
	scheduleHooks := scheduler.NewHookRegistry()
	scheduleHooks.Register("launch_test_run", func(ctx context.Context, payload *anypb.Any) error {
		var id testingpb.TestRunId
		if err := payload.UnmarshalTo(&id); err != nil {
			return fmt.Errorf("launch_test_run: unmarshal payload: %w", err)
		}
		_, err := runSvc.LaunchTestRun(ctx, &id)
		return err
	})
	scheduleHooks.Register("launch_test_suite", func(ctx context.Context, payload *anypb.Any) error {
		var id testingpb.TestSuiteId
		if err := payload.UnmarshalTo(&id); err != nil {
			return fmt.Errorf("launch_test_suite: unmarshal payload: %w", err)
		}
		// Caller for system-triggered launches: use the schedule's owner from
		// metadata when present, otherwise nil (suite-run-service tolerates
		// nil caller for system-initiated runs).
		_, err := suiteRunSvc.LaunchTestSuite(ctx, &id, nil)
		return err
	})
	sched := scheduler.New(pool, systemSvc, scheduler.Config{
		Tick:          cfg.Workers.SchedulerTick,
		LeaseDuration: 30 * time.Second,
	}, zlog).WithHooks(scheduleHooks)
	// Wire synchronous TriggerNow → hook dispatch so the UI button observes
	// the side effect immediately. The scheduler-worker tick handles all
	// other cron-driven fires.
	systemSvc.WithHookInvoker(func(ctx context.Context, name string, payload *anypb.Any) error {
		hook, ok := scheduleHooks.Lookup(name)
		if !ok {
			return fmt.Errorf("system.TriggerNow: hook %q not registered", name)
		}
		return hook(ctx, payload)
	})

	workerCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()
	go worker.Run(workerCtx)
	go sched.Run(workerCtx)

	// Seed built-in packages whenever a tenant is created. Best-effort:
	// failures are logged, not propagated, so a partial seed doesn't roll
	// back the tenant creation.
	bus.Subscribe(eventing.TopicTenantCreated, func(_ context.Context, e eventing.Event) {
		payload, ok := e.Payload.(eventing.TenantCreated)
		if !ok {
			return
		}
		seedCtx, c := context.WithTimeout(context.Background(), 30*time.Second)
		defer c()
		if err := catalogSvc.SeedBuiltinPackages(seedCtx, &iampb.TenantId{Value: payload.TenantID}, nil); err != nil {
			zlog.Warn("seed builtin packages failed", zap.String("tenant_id", payload.TenantID), zap.Error(err))
		}
	})

	// Release one concurrent-run quota slot whenever a test run finishes.
	bus.Subscribe(eventing.TopicTestRunDone, func(_ context.Context, e eventing.Event) {
		payload, ok := e.Payload.(eventing.TestRunDone)
		if !ok {
			return
		}
		releaseCtx, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		_ = quotaSvc.Release(releaseCtx, &iampb.TenantId{Value: payload.TenantID}, "runs.concurrent", 1)
	})

	// Bootstrap initial admin (idempotent).
	if cfg.Features.InitialAdminEmail != "" {
		pwd := os.Getenv(cfg.Features.InitialAdminPasswordEnv)
		if pwd != "" {
			if err := bootstrapAdmin(ctx, iamSvc, cfg.Features.InitialAdminEmail, pwd, zlog); err != nil {
				return fmt.Errorf("bootstrap admin: %w", err)
			}
		}
	}

	otelInterceptor, err := otelconnect.NewInterceptor()
	if err != nil {
		return err
	}

	interceptors := connect.WithInterceptors(
		middleware.Recovery(zlog),
		middleware.RequestID(),
		otelInterceptor,
		middleware.Logging(zlog),
		middleware.Auth(iamSvc, transportconnect.AuthBypass()),
		middleware.Tenant(iamSvc, transportconnect.TenantBypass()),
		middleware.ProtoValidate(),
		middleware.Idempotency(valkeyCli, middleware.BuildIdempotencyRegistry(), middleware.IdempotencyConfig{
			Enabled: cfg.Idempotency.Enabled,
			TTL:     cfg.Idempotency.TTL,
		}),
		middleware.ErrorMapper(),
	)

	mux := http.NewServeMux()
	connectMux := transportconnect.Mount(transportconnect.Deps{
		IAMHandler:              transportconnect.NewIAMHandler(iamSvc, cfg.Auth.RefreshTTL, cfg.Auth.CookieSecure),
		CatalogHandler:          transportconnect.NewCatalogHandler(catalogSvc),
		StroppyHandler:          transportconnect.NewStroppyHandler(stroppySvc),
		ScheduleHandler:         transportconnect.NewScheduleHandler(systemSvc),
		TestingHandler:          transportconnect.NewTestingHandler(tplSvc, runSvc, suiteSvc),
		TestSuiteRunHandler:     transportconnect.NewTestSuiteRunHandler(suiteRunSvc),
		SharedTestRunHandler:    transportconnect.NewSharedTestRunHandler(sharedTestRunSvc),
		SharedSuiteRunHandler:   transportconnect.NewSharedSuiteRunHandler(sharedSuiteRunSvc),
		ComparisonHandler:       transportconnect.NewComparisonHandler(comparisonSvc),
		BaselineHandler:         transportconnect.NewBaselineHandler(baselineSvc),
		AgentHandler:            transportconnect.NewAgentHandler(agentService),
		WebhookHandler:          transportconnect.NewWebhookHandler(webhookSvc),
		QuotaHandler:            transportconnect.NewQuotaHandler(quotaSvc),
		BinaryCacheHandler:      transportconnect.NewBinaryCacheHandler(binaryCacheSvc),
		AdminHandler:            transportconnect.NewAdminHandler(adminService),
		BinaryCacheAdminHandler: transportconnect.NewBinaryCacheAdminHandler(binaryCacheAdminSvc),
		Interceptors:            interceptors,
	})
	// ConnectRPC paths all start with /cloud.v1.<package>.<Service>/ — route
	// them to the connect mux; /metrics + /health + /healthz are static
	// endpoints; everything else falls through to the embedded SPA file
	// server with index.html fallback for client-side routing.
	spaFS, _ := fs.Sub(web.Dist, "dist")
	spaServer := http.FileServer(http.FS(spaFS))
	processStartedAt := time.Now()
	mux.Handle("/metrics", middleware.MetricsHandler())
	httpext.NewBinariesHandler("", binaryCacheSvc, zlog).Mount(mux)
	httpext.NewPackagesHandler(catalogSvc, zlog).Mount(mux)
	httpext.NewGrafanaHandler().Mount(mux)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/cloud.v1."):
			connectMux.ServeHTTP(w, r)
		case r.URL.Path == "/healthz" || r.URL.Path == "/health":
			payload := map[string]any{
				"status":     "ok",
				"service":    build.ServiceName,
				"version":    build.Version,
				"instance":   build.GlobalInstanceId,
				"uptime_sec": int64(time.Since(processStartedAt).Seconds()),
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(payload)
		default:
			if r.URL.Path != "/" {
				f, err := spaFS.Open(r.URL.Path[1:])
				if err != nil {
					r2 := r.Clone(r.Context())
					r2.URL.Path = "/"
					spaServer.ServeHTTP(w, r2)
					return
				}
				_ = f.Close()
			}
			spaServer.ServeHTTP(w, r)
		}
	})

	srv := &http.Server{
		Addr:    cfg.Server.HTTPAddr,
		Handler: h2c.NewHandler(middleware.MetricsHTTP(mux), &http2.Server{}),
	}

	zlog.Info("serving", zap.String("addr", cfg.Server.HTTPAddr))

	stopCtx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	go func() {
		<-stopCtx.Done()
		zlog.Info("shutdown signal received")
		shutdownCtx, c := context.WithTimeout(context.Background(), 30*time.Second)
		defer c()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func bootstrapAdmin(ctx context.Context, iamSvc *iam.Service, email, password string, log *zap.Logger) error {
	existing, err := iamSvc.ListUsers(ctx)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}
	user, err := iamSvc.CreateUser(ctx, &iampb.User{Email: email, Nickname: "admin"}, password)
	if err != nil {
		return err
	}
	if _, err := iamSvc.PromoteToAdmin(ctx, user.GetId()); err != nil {
		return fmt.Errorf("promote admin: %w", err)
	}
	log.Info("bootstrapped initial admin", zap.String("user_id", user.GetId().GetValue()))
	return nil
}
