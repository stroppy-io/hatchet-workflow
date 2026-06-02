package run

import (
	"context"
	"strings"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

// monitorTokenEnvKey is the optional agent-bootstrap extra-env key carrying the
// bearer token the gateway/vmauth expects for the /insert/* monitoring relay.
const monitorTokenEnvKey = "STROPPY_MONITORING_TOKEN"

type SettingsSource interface {
	ProviderSettings(ctx context.Context, tenantID string, provider deployment.Provider) (*deployment.ProviderSettings, error)
	AgentBootstrap(ctx context.Context) (*workflowpb.AgentBootstrap, error)
}

func BuildRunConfig(ctx context.Context, tenantID string, settings SettingsSource, options BuildOptions) (*workflowpb.RunConfig, error) {
	var bootstrap *workflowpb.AgentBootstrap
	if settings != nil {
		providerSettings, err := settings.ProviderSettings(ctx, tenantID, options.Provider)
		if err != nil {
			return nil, err
		}
		options.Infrastructure.Settings = providerSettings

		bootstrap, err = settings.AgentBootstrap(ctx)
		if err != nil {
			return nil, err
		}
	}

	testRun, err := BuildTestRun(options)
	if err != nil {
		return nil, err
	}

	// Stamp the monitoring context onto the topology-spec labels so the
	// deployment renderer (which never receives the agent bootstrap) can build
	// the agent-side metrics/logs collector phase. Agents reach monitoring only
	// through the server/gateway address.
	stampMonitorLabels(testRun.GetTopologySpec(), testRun.GetId(), bootstrap)

	cfg := &workflowpb.RunConfig{
		Id:                 testRun.GetId(),
		Database:           testRun.GetDatabase(),
		Workload:           testRun.GetWorkload(),
		TopologySpec:       testRun.GetTopologySpec(),
		InfrastructurePlan: testRun.GetInfrastructurePlan(),
		RenderOverrides:    testRun.GetRenderOverrides(),
		AgentBootstrap:     bootstrap,
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// stampMonitorLabels writes the server address, run id, and optional monitoring
// bearer token onto the topology-spec labels. The deployment renderer reads them
// (deployment.LabelServerAddr / LabelRunID / LabelMonitorBearerToken) to build
// the collector phase. A nil/empty bootstrap leaves labels untouched, so the
// monitor phase is simply skipped (e.g. local/docker runs without a server addr).
func stampMonitorLabels(spec *topology.TopologySpec, runID string, bootstrap *workflowpb.AgentBootstrap) {
	if spec == nil || bootstrap == nil {
		return
	}
	serverAddr := strings.TrimRight(bootstrap.GetServerAddr(), "/")
	if serverAddr == "" {
		return
	}
	if spec.Labels == nil {
		spec.Labels = make(map[string]string)
	}
	spec.Labels[deploymentbuilder.LabelServerAddr] = serverAddr
	if runID != "" {
		spec.Labels[deploymentbuilder.LabelRunID] = runID
	}
	if token := bootstrap.GetExtraEnv()[monitorTokenEnvKey]; token != "" {
		spec.Labels[deploymentbuilder.LabelMonitorBearerToken] = token
	}
}
