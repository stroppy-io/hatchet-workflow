package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

// heartbeat keeps a long, single-blocking-call activity (terraform apply/teardown,
// recipe build) alive against its HeartbeatTimeout. terraform doesn't expose
// progress, so we beat on a fixed interval; the goroutine stops via the returned
// func. This lets Temporal detect a dead worker within the HeartbeatTimeout while
// NOT retrying an activity that is merely slow (which would collide on the
// terraform state lock held by the still-running first attempt).
func heartbeat(ctx context.Context) (stop func()) {
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(20 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				activity.RecordHeartbeat(ctx)
			}
		}
	}()
	return func() { close(done) }
}

// ServerActivities holds the dependencies for the side-effectful, server-side
// activities (deploy / teardown / recipe build). They run on the shared server
// task queue where the docker/terraform SDKs live. Registered by value via
// RegisterAll; the workflow references them by func, so a nil receiver is fine
// at ExecuteActivity-build time.
type ServerActivities struct {
	Deployer *agent.DockerDeployer
	// Settings is a static fallback used when SettingsFunc is unset.
	Settings *types.ServerSettings
	// SettingsFunc resolves a run's ServerSettings from its tenant. The worker
	// serves all tenants, so cloud creds (settings.Cloud.Yandex) MUST be resolved
	// per run from the run's tenant — a static Settings can't carry them.
	SettingsFunc    func(tenantID string) *types.ServerSettings
	JWTIssuer       *auth.JWTIssuer
	ServerAddr      string
	MonitoringURL   string
	MonitoringToken string
	AccountID       int32
	TenantID        string
	Logger          *zap.Logger
}

func (sa *ServerActivities) logger() *zap.Logger {
	if sa.Logger != nil {
		return sa.Logger
	}
	return zap.NewNop()
}

// settingsFor resolves the run tenant's ServerSettings (cloud creds, etc),
// falling back to the static Settings when no resolver/tenant is available.
func (sa *ServerActivities) settingsFor(tenantID string) *types.ServerSettings {
	if sa.SettingsFunc != nil && tenantID != "" {
		if s := sa.SettingsFunc(tenantID); s != nil {
			return s
		}
	}
	return sa.Settings
}

func (sa *ServerActivities) deps(state *run.State, settings *types.ServerSettings) run.Deps {
	return run.Deps{
		Deployer:        sa.Deployer,
		State:           state,
		ServerAddr:      sa.ServerAddr,
		Settings:        settings,
		MonitoringURL:   sa.MonitoringURL,
		MonitoringToken: sa.MonitoringToken,
		AccountID:       sa.AccountID,
		JWTIssuer:       sa.JWTIssuer,
		TenantID:        sa.TenantID,
	}
}

// DeployMachinesActivity provisions the network + machines (containers / VMs) and
// returns the resulting targets + endpoint + teardown handles as a Deployment.
func (sa *ServerActivities) DeployMachinesActivity(ctx context.Context, rc *workflowpb.RunConfig) (*workflowpb.Deployment, error) {
	cfg, err := decodeRunConfig(rc)
	if err != nil {
		return nil, err
	}
	defer heartbeat(ctx)()
	state := run.NewState()
	nc := run.NewNodeContext(ctx, sa.logger())
	if err := run.Deploy(nc, cfg, sa.deps(state, sa.settingsFor(cfg.Monitor.TenantID))); err != nil {
		return nil, err
	}
	return runStateToDeployment(state.ExportRunState()), nil
}

// TeardownActivity destroys the run's infrastructure from the deployment handles.
// Idempotent; runs even on failure.
func (sa *ServerActivities) TeardownActivity(ctx context.Context, req *TeardownRequest) error {
	cfg, err := decodeRunConfig(req.Config)
	if err != nil {
		return err
	}
	state := run.NewState()
	defer heartbeat(ctx)()
	state.ImportRunState(deploymentToRunState(req.Deployment))
	nc := run.NewNodeContext(ctx, sa.logger())
	return run.Teardown(nc, cfg, sa.deps(state, sa.settingsFor(cfg.Monitor.TenantID)))
}

// BuildRecipeActivity builds the per-machine command plans by driving the run
// recipe in collect mode against the deployed targets.
func (sa *ServerActivities) BuildRecipeActivity(ctx context.Context, req *BuildRequest) ([]InstancePlan, error) {
	cfg, err := decodeRunConfig(req.Config)
	if err != nil {
		return nil, err
	}
	// Prefer the per-run monitoring tenant stamped into the config at launch
	// (cfg.Monitor) over the worker's static default, so agents write metrics/logs
	// to the SAME account the UI reads from. TenantID!="" marks it resolved.
	acct, tenant := sa.AccountID, sa.TenantID
	if cfg.Monitor.TenantID != "" {
		acct, tenant = cfg.Monitor.AccountID, cfg.Monitor.TenantID
	}
	// Cloud VMs reach the control plane ONLY through the public gateway, so both
	// the binary/artifact downloads (ServerAddr → /api/binaries, /artifacts) and
	// the metrics/log ingest (MetricsURL → /insert/*, relayed by the gateway to
	// vmauth) must use the PUBLIC cloud.server_addr. The docker-internal
	// AGENT_SERVER_ADDR ("http://server:8080") / MONITORING_URL ("http://vmauth:8427")
	// can't be resolved from a YC VM. Docker agents keep the in-cluster addresses.
	serverAddr, metricsURL := sa.ServerAddr, sa.MonitoringURL
	if cfg.Provider == types.ProviderYandex {
		if s := sa.settingsFor(cfg.Monitor.TenantID); s != nil && s.Cloud.ServerAddr != "" {
			serverAddr = s.Cloud.ServerAddr
			metricsURL = s.Cloud.ServerAddr
		}
	}
	return BuildPlans(ctx, sa.logger(), cfg, req.Deployment, MonitoringRefs{
		ServerAddr: serverAddr,
		MetricsURL: metricsURL,
		Token:      sa.MonitoringToken,
		AccountID:  acct,
		TenantID:   tenant,
	})
}

// BuildRequest is the input to BuildRecipeActivity.
type BuildRequest struct {
	Config     *workflowpb.RunConfig
	Deployment *workflowpb.Deployment
}

// TeardownRequest carries the deployment handles + run config (provider) needed
// to tear the stand down.
type TeardownRequest struct {
	Config     *workflowpb.RunConfig
	Deployment *workflowpb.Deployment
}

// decodeRunConfig unmarshals the opaque config_json into the run engine's config.
func decodeRunConfig(rc *workflowpb.RunConfig) (types.RunConfig, error) {
	var cfg types.RunConfig
	if err := json.Unmarshal(rc.GetConfigJson(), &cfg); err != nil {
		return cfg, fmt.Errorf("workflows: decode run config: %w", err)
	}
	return cfg, nil
}

// runStateToDeployment exports the run.RunState into the proto Deployment the
// workflow threads to BuildRecipe + Teardown.
func runStateToDeployment(rs *run.RunState) *workflowpb.Deployment {
	dep := &workflowpb.Deployment{
		DbHost:        rs.DBHost,
		DbPort:        int32(rs.DBPort),
		ContainerIds:  rs.ContainerIDs,
		NetworkId:     rs.NetworkID,
		TerraformWdId: rs.TerraformWdId,
	}
	for _, t := range rs.Targets {
		dep.Targets = append(dep.Targets, &workflowpb.Target{
			Id:           t.ID,
			Host:         t.Host,
			InternalHost: t.InternalHost,
			AgentPort:    int32(t.AgentPort),
			Zone:         t.Zone,
			Role:         t.Role,
		})
	}
	return dep
}
