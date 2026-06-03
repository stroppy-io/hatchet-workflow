package run

import (
	"context"
	"strings"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

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

	// Stamp the non-secret monitoring context onto the topology-spec labels so
	// the deployment renderer can build the agent-side metrics/logs collector
	// phase. Per-agent bearer tokens travel separately in AgentBootstrap.
	stampMonitorLabels(testRun.GetTopologySpec(), testRun.GetId(), bootstrap)

	cfg := &workflowpb.RunConfig{
		TenantId:           tenantID,
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

// stampMonitorLabels writes the server address and run id onto topology labels.
// Per-node bearer tokens stay in AgentBootstrap.AgentTokens so they are not
// exposed as topology/runtime metadata. A nil/empty bootstrap leaves labels
// untouched, so the monitor phase is skipped.
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
}
