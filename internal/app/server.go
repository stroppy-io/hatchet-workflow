package app

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/gopherex/pgtx"
	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/gopherex/xlog"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"

	apiadmin "github.com/stroppy-io/stroppy-cloud/internal/api/admin"
	apiagent "github.com/stroppy-io/stroppy-cloud/internal/api/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/api/middleware"
	apiui "github.com/stroppy-io/stroppy-cloud/internal/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/planner"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/dagstore"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	infs3 "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/s3"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/valkey"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/victoria"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/webhooksender"
	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/admin"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/agent"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
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
	"github.com/stroppy-io/stroppy-cloud/internal/services/run"
	"github.com/stroppy-io/stroppy-cloud/internal/services/settings"
	"github.com/stroppy-io/stroppy-cloud/internal/services/share"
	"github.com/stroppy-io/stroppy-cloud/internal/services/suite"
	"github.com/stroppy-io/stroppy-cloud/internal/services/tasks"
	"github.com/stroppy-io/stroppy-cloud/internal/services/tenancy"
	"github.com/stroppy-io/stroppy-cloud/internal/services/webhook"
	"github.com/stroppy-io/stroppy-cloud/internal/services/yandexcloud"
)

// Server is the assembled control plane: a grpc API plus the background dag
// processor. Run blocks serving both; Close releases resources.
type Server struct {
	grpc      *grpc.Server
	processor *runtime.DagProcessor
	pool      *pgxpool.Pool
	addr      string
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
	provider := deploy.New(logger, executor, netInv, cfg)
	plannerImpl := planner.New()

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

	// ── grpc server ──────────────────────────────────────────────────────────
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			middleware.NewAuthInterceptor(authSvc),
			adminGuardInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			authStreamInterceptor(authSvc),
		),
	)

	uipb.RegisterAuthServiceServer(srv, apiui.NewAuthService(logger, authSvc))
	uipb.RegisterRunServiceServer(srv, apiui.NewRunService(logger, runSvc))
	uipb.RegisterSuiteServiceServer(srv, apiui.NewSuiteService(logger, suiteSvc))
	uipb.RegisterPresetServiceServer(srv, apiui.NewPresetService(logger, presetSvc))
	uipb.RegisterTenantServiceServer(srv, apiui.NewTenantService(logger, tenantSvc))
	uipb.RegisterPackageServiceServer(srv, apiui.NewPackageService(logger, packageSvc))
	uipb.RegisterSettingsServiceServer(srv, apiui.NewSettingsService(logger, settingsSvc))
	uipb.RegisterCloudInventoryServiceServer(srv, apiui.NewCloudInventoryService(logger, inventorySvc))
	uipb.RegisterApiTokenServiceServer(srv, apiui.NewApiTokenService(logger, apiTokenSvc))
	uipb.RegisterWebhookServiceServer(srv, apiui.NewWebhookService(logger, webhookSvc))
	adminpb.RegisterAccountAdminServiceServer(srv, apiadmin.NewAccountAdminService(logger, accountAdminSvc))
	adminpb.RegisterTenantAdminServiceServer(srv, apiadmin.NewTenantAdminService(logger, tenantAdminSvc))
	agentpb.RegisterAgentServiceServer(srv, apiagent.NewAgentService(logger, agentSvc))

	return &Server{grpc: srv, processor: processor, pool: pool, addr: cfg.GRPCAddr, log: logger}, nil
}

// Run starts the dag processor and serves the grpc API until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	if err := s.processor.Start(ctx); err != nil {
		return fmt.Errorf("start processor: %w", err)
	}
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.addr, err)
	}
	go func() {
		<-ctx.Done()
		s.grpc.GracefulStop()
	}()
	s.log.Info("stroppy-cloud serving", xlog.String("addr", s.addr))
	return s.grpc.Serve(lis)
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
