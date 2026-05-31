// Package app is the demo backbone: it wires the GORM-backed repositories
// (auto-migrated postgres), a Temporal client + server worker, the
// wizard/run/overview services and a real gRPC server into a single App.
//
// Construction (New) does NOT touch postgres, Temporal or Docker: it only builds
// the static auth, the wizard engine and the docker deployer handle, so
// `go build`/`go vet` and the e2e can construct an App before any backing
// service exists. The postgres connection + migration, the Temporal connection,
// the worker, the repo-backed services and the gRPC server are all built in
// Start(), which is where Temporal already connects (keeping all live-backend IO
// in one place).
//
// The three services are still exposed as App fields so the in-process e2e can
// drive them directly without a gRPC round-trip; the gRPC server is additive.
package app

import (
	"fmt"
	"net"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/stroppy-io/stroppy-cloud/internal/deploy/dockerprov"
	"github.com/stroppy-io/stroppy-cloud/internal/gateway"
	"github.com/stroppy-io/stroppy-cloud/internal/overview"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"github.com/stroppy-io/stroppy-cloud/internal/services/system_settings"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_run"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_run_overview"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_wizard"
	"github.com/stroppy-io/stroppy-cloud/internal/store/gormstore"
	"github.com/stroppy-io/stroppy-cloud/internal/wizard/testengine"
	"github.com/stroppy-io/stroppy-cloud/internal/workflows"
)

// App is the wired demo backbone. The three services are exposed as fields so an
// in-process caller (the e2e) can drive them directly without a gRPC round-trip.
// They are nil until Start() has connected the backends and constructed them.
type App struct {
	cfg Config

	// Pure deps, built in New (no IO).
	authn  staticAuthn
	sum    summarizer
	engine *testengine.Engine
	trm    noopTrm

	// Docker deployer handle (no daemon contact until an activity runs).
	deployer *dockerprov.Deployer

	// Live backends, built in Start.
	db      *gorm.DB
	client  client.Client
	worker  worker.Worker
	grpc    *grpc.Server
	gateway *gateway.Gateway

	// Services (built in Start). Exposed for in-process callers.
	TestWizard *test_wizard.TestWizardService
	TestRun    *test_run.TestRunService
	Overview   *test_run_overview.TestRunOverviewService
}

// New builds the App without contacting postgres, Temporal or Docker. The
// backends, repositories, services and gRPC server are wired in Start().
func New(cfg Config) (*App, error) {
	deployer, err := dockerprov.New(cfg.AgentImage)
	if err != nil {
		return nil, fmt.Errorf("app: create docker deployer: %w", err)
	}
	// Agents are told ONLY the server address; the deployer injects it as
	// STROPPY_SERVER_ADDR into every container.
	deployer.ServerAddr = cfg.AgentServerAddr
	return &App{
		cfg:      cfg,
		engine:   testengine.New(),
		deployer: deployer,
	}, nil
}

// stroppyArtifactPath is the gateway path agents fetch stroppy from, or empty
// when no upstream is configured (the recipe then has no concrete artifact). The
// full URL is composed at deploy time from the resolved server address.
func (a *App) stroppyArtifactPath() string {
	if a.cfg.StroppyUpstream == "" {
		return ""
	}
	return "/artifacts/stroppy"
}

// Start opens postgres (GORM, auto-migrated), connects to Temporal, wires the
// repo-backed services, registers the TestWorkflow + server activities on the
// SERVER worker (task queue "stroppy-cloud") and starts it non-blocking, then
// stands up a gRPC server serving the three services in a background goroutine.
// After Start returns the App's service fields are populated and ready to be
// called in-process AND over gRPC.
func (a *App) Start() error {
	// Postgres + auto-migration.
	db, err := gorm.Open(postgres.Open(a.cfg.DatabaseURL), &gorm.Config{TranslateError: true})
	if err != nil {
		return fmt.Errorf("app: open postgres: %w", err)
	}
	if err := gormstore.Migrate(db); err != nil {
		return fmt.Errorf("app: migrate: %w", err)
	}
	a.db = db
	store := gormstore.New(db)
	runs := store.TestRuns()
	drafts := store.Drafts()
	presets := store.Presets()
	settings := store.Settings()

	// Temporal.
	c, err := client.Dial(client.Options{
		HostPort:  a.cfg.TemporalHostPort,
		Namespace: a.cfg.TemporalNamespace,
	})
	if err != nil {
		return fmt.Errorf("app: dial temporal: %w", err)
	}
	a.client = c

	// Server worker: TestWorkflow + server activities (deploy/build/teardown).
	// The agent-facing server address is the admin-set instance setting
	// (PlatformSettings.ServerAddr); empty falls back to the derived default.
	sa := &workflows.ServerActivities{
		Deployer:            a.deployer,
		StroppyArtifactPath: a.stroppyArtifactPath(),
		StroppyChecksum:     a.cfg.StroppyChecksum,
		Settings:            settings,
		DefaultServerAddr:   a.cfg.AgentServerAddr,
	}
	w := worker.New(c, workflow.TestServiceTaskQueue, worker.Options{})
	workflows.RegisterAll(w, sa)
	a.worker = w

	// Temporal-backed adapters.
	launcher := workflows.NewLauncher(c)
	ov := overview.New(c)

	// TestRun service.
	a.TestRun = test_run.NewTestRunService(test_run.TestRunDeps{
		Authn:      a.authn,
		Runs:       runs,
		Presets:    presets,
		Summarizer: a.sum,
		Workflows:  launcher,
		Tx:         a.trm,
	})

	// TestWizard service. Finish(start=true) routes through runStarter.
	a.TestWizard = test_wizard.NewTestWizardService(test_wizard.TestWizardDeps{
		Authn:   a.authn,
		Drafts:  drafts,
		Presets: presets,
		Engine:  a.engine,
		Runs:    newRunStarter(runs, a.sum, launcher),
		Saver:   presets,
		Tx:      a.trm,
	})

	// Overview service. The run reader is the gorm TestRunRepo (Exists).
	a.Overview = test_run_overview.NewTestRunOverviewService(test_run_overview.TestRunOverviewDeps{
		Authn:    a.authn,
		Runs:     runs,
		Overview: ov,
		Logs:     noopLogReader{},
		Metrics:  noopMetricsReader{},
		Tx:       a.trm,
	})

	// Non-blocking: the worker polls in the background.
	if err := w.Start(); err != nil {
		return fmt.Errorf("app: start worker: %w", err)
	}

	// System settings service: admins set the instance's agent-facing ServerAddr
	// here; the deploy activity reads it back when provisioning agent VMs.
	sysSettings := system_settings.NewSystemSettingsService(system_settings.SystemSettingsDeps{
		Authn:    a.authn,
		Settings: settings,
		Tx:       a.trm,
	})

	// gRPC server: additive surface over the same service instances.
	s := grpc.NewServer()
	api.RegisterTestWizardServiceServer(s, a.TestWizard)
	api.RegisterTestRunServiceServer(s, a.TestRun)
	api.RegisterTestRunOverviewServiceServer(s, a.Overview)
	api.RegisterSystemSettingsServiceServer(s, sysSettings)
	a.grpc = s

	lis, err := net.Listen("tcp", a.cfg.GRPCAddr)
	if err != nil {
		return fmt.Errorf("app: listen %s: %w", a.cfg.GRPCAddr, err)
	}
	go func() { _ = s.Serve(lis) }()

	// Agent gateway: the SINGLE address agents are told about. Temporal proxy +
	// agent binary + artifact cache + apt (forwarded to apt-cacher-ng), all
	// multiplexed on one port.
	gw, err := gateway.New(gateway.Config{
		TemporalHostPort: a.cfg.TemporalHostPort,
		AgentBinaryPath:  a.cfg.AgentBinaryPath,
		CacheDir:         a.cfg.CacheDir,
		Artifacts:        map[string]string{"stroppy": a.cfg.StroppyUpstream},
		AptBackend:       a.cfg.AptCacheBackend,
	})
	if err != nil {
		return fmt.Errorf("app: build gateway: %w", err)
	}
	a.gateway = gw
	gwLis, err := net.Listen("tcp", a.cfg.AgentGatewayAddr)
	if err != nil {
		return fmt.Errorf("app: listen gateway %s: %w", a.cfg.AgentGatewayAddr, err)
	}
	go func() { _ = gw.Serve(gwLis) }()

	return nil
}

// Close stops the gRPC server, the worker, closes the Temporal client and the
// postgres pool. Safe to call once after a successful Start (no-op for nil
// handles).
func (a *App) Close() {
	if a.gateway != nil {
		a.gateway.Close()
	}
	if a.grpc != nil {
		a.grpc.GracefulStop()
	}
	if a.worker != nil {
		a.worker.Stop()
	}
	if a.client != nil {
		a.client.Close()
	}
	if a.db != nil {
		if sqlDB, err := a.db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
}
