package run

import (
	"context"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
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
