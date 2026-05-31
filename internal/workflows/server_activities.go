package workflows

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

// ServerActivities holds the dependencies for the side-effectful, server-side
// activities (deploy / teardown / recipe build). They run on the shared server
// task queue where the docker/terraform SDKs live. Registered by value via
// RegisterAll; the workflow references them by func, so a nil receiver is fine
// at ExecuteActivity-build time.
type ServerActivities struct {
	Deployer        *agent.DockerDeployer
	Settings        *types.ServerSettings
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

func (sa *ServerActivities) deps(state *run.State) run.Deps {
	return run.Deps{
		Deployer:        sa.Deployer,
		State:           state,
		ServerAddr:      sa.ServerAddr,
		Settings:        sa.Settings,
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
	state := run.NewState()
	nc := run.NewNodeContext(ctx, sa.logger())
	if err := run.Deploy(nc, cfg, sa.deps(state)); err != nil {
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
	state.ImportRunState(deploymentToRunState(req.Deployment))
	nc := run.NewNodeContext(ctx, sa.logger())
	return run.Teardown(nc, cfg, sa.deps(state))
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
	return BuildPlans(ctx, sa.logger(), cfg, req.Deployment, MonitoringRefs{
		ServerAddr: sa.ServerAddr,
		MetricsURL: sa.MonitoringURL,
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
