package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	otelconnect "connectrpc.com/otelconnect"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/stroppy-io/stroppy-cloud/internal/core/configurator"
	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/logger"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/stroppy"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	mockhandler "github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker/handlers"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/recovery"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/scheduler"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/migrations"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/s3"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/stroppybin"
	valkey "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/valkey"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	transportconnect "github.com/stroppy-io/stroppy-cloud/internal/transport/connect"
	"github.com/stroppy-io/stroppy-cloud/internal/transport/middleware"
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

	if cfg.Workers.RecoveryOnStart {
		if err := recovery.Run(ctx, pool, zlog); err != nil {
			return fmt.Errorf("recovery: %w", err)
		}
	}

	nodeReg := nodeworker.NewRegistry()
	nodeReg.Register(mockhandler.NewMockHandler())

	worker := nodeworker.New(pool, systemSvc, nodeReg, nodeworker.Config{
		Workers: cfg.Workers.NodeWorkers,
		Tick:    500 * time.Millisecond,
	}, zlog)
	sched := scheduler.New(pool, systemSvc, scheduler.Config{
		Tick:          cfg.Workers.SchedulerTick,
		LeaseDuration: 30 * time.Second,
	}, zlog)

	workerCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()
	go worker.Run(workerCtx)
	go sched.Run(workerCtx)

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
	mux.Handle("/", transportconnect.Mount(transportconnect.Deps{
		IAMHandler:     transportconnect.NewIAMHandler(iamSvc),
		CatalogHandler: transportconnect.NewCatalogHandler(catalogSvc),
		StroppyHandler: transportconnect.NewStroppyHandler(stroppySvc),
		Interceptors:   interceptors,
	}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	srv := &http.Server{
		Addr:    cfg.Server.HTTPAddr,
		Handler: h2c.NewHandler(mux, &http2.Server{}),
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
	log.Info("bootstrapped initial admin", zap.String("user_id", user.GetId().GetValue()))
	return nil
}
