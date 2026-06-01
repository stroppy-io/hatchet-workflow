package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	temporalclient "go.temporal.io/sdk/client"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

// App is the top-level application facade. Run execution now lives in Temporal:
// App is a thin launcher over a Temporal client (it starts RunWorkflow and reads
// the live RunState via a query). The DAG executor + durable scheduler are gone.
type App struct {
	pool   *pgxpool.Pool
	logger *zap.Logger

	temporal temporalclient.Client
	launcher workflowpb.RunWorkflowServiceClient

	// Set by Server after construction.
	settingsFunc    func(tenantID string) *types.ServerSettings
	monitoringURL   string
	monitoringToken string
	accountIDFunc   func(tenantID string) int32
	jwtIssuer       *auth.JWTIssuer
}

// Config holds application-level settings.
type Config struct {
	Pool   *pgxpool.Pool
	Logger *zap.Logger
}

// New creates a new App. SetTemporal must be called before launching runs.
func New(cfg Config) *App {
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}
	return &App{pool: cfg.Pool, logger: cfg.Logger}
}

// SetTemporal wires the Temporal client used to launch + query RunWorkflows.
func (a *App) SetTemporal(c temporalclient.Client) {
	a.temporal = c
	a.launcher = workflowpb.NewRunWorkflowServiceClient(c)
}

// SettingsFunc returns the per-tenant ServerSettings resolver (set by Server).
// The Temporal server worker uses it to resolve a run's cloud creds from the
// run's tenant.
func (a *App) SettingsFunc() func(tenantID string) *types.ServerSettings {
	return a.settingsFunc
}

// JWTIssuer returns the issuer used to mint agent tokens for provisioned VMs.
func (a *App) JWTIssuer() *auth.JWTIssuer { return a.jwtIssuer }

// runWorkflowID is the deterministic workflow id for a run.
func runWorkflowID(runID string) string { return "run/" + runID }

// Start validates the config and launches its RunWorkflow on Temporal (async).
func (a *App) Start(ctx context.Context, tenantID string, cfg types.RunConfig) error {
	if err := run.ValidateConfig(cfg); err != nil {
		return fmt.Errorf("api: validate: %w", err)
	}
	run.FillMachinesFromTopology(&cfg)

	// Stamp the run tenant's monitoring account into the config so the agents'
	// vmagent/vector write to the SAME VictoriaMetrics/VictoriaLogs tenant the UI
	// reads from (accountIDFromRunID → tenant account_id). Without this the
	// ServerActivities default of account 0 mismatches the reader and the UI
	// shows empty metrics/logs.
	cfg.Monitor.TenantID = tenantID
	if a.accountIDFunc != nil {
		cfg.Monitor.AccountID = a.accountIDFunc(tenantID)
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("api: marshal run config: %w", err)
	}
	rc := &workflowpb.RunConfig{
		Id:         cfg.ID,
		Provider:   string(cfg.Provider),
		ExternalDb: cfg.ExternalDB != nil,
		ConfigJson: data,
	}
	if a.launcher == nil {
		return fmt.Errorf("api: temporal not wired")
	}
	opts := workflowpb.NewRunWorkflowOptions().WithID(runWorkflowID(cfg.ID))
	_, err = a.launcher.RunWorkflowAsync(ctx, rc, opts)
	return err
}

// Status returns the live RunState (status + per-phase breakdown) for a run via
// the RunWorkflow GetRunWorkflowState query.
func (a *App) Status(ctx context.Context, runID string) (*workflowpb.RunState, error) {
	if a.launcher == nil {
		return nil, fmt.Errorf("api: temporal not wired")
	}
	return a.launcher.GetRunWorkflowState(ctx, runWorkflowID(runID), "")
}

// Cancel requests cancellation of a run's RunWorkflow.
func (a *App) Cancel(ctx context.Context, runID string) error {
	if a.temporal == nil {
		return fmt.Errorf("api: temporal not wired")
	}
	return a.temporal.CancelWorkflow(ctx, runWorkflowID(runID), "")
}

// Validate checks that a RunConfig is well-formed (no graph build needed).
func (a *App) Validate(cfg types.RunConfig) error {
	if err := run.ValidateConfig(cfg); err != nil {
		return err
	}
	run.FillMachinesFromTopology(&cfg)
	return nil
}

// DryRun returns the resolved RunConfig (and an empty graph placeholder — the
// run now executes as a Temporal workflow, not a static DAG). Kept so the review
// step's resolved-config preview still works.
func (a *App) DryRun(cfg types.RunConfig) ([]byte, *types.RunConfig, error) {
	run.BakeMachineOverrideIntoTopology(&cfg)
	run.AdjustYDBStorageDisk(&cfg)
	run.FillMachinesFromTopology(&cfg)
	return []byte(`{"nodes":[]}`), &cfg, nil
}

// LoadConfig reads a RunConfig from a JSON file.
func LoadConfig(path string) (types.RunConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return types.RunConfig{}, err
	}
	var cfg types.RunConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return types.RunConfig{}, err
	}
	return cfg, nil
}
