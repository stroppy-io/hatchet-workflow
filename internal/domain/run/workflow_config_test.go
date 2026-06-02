package run

import (
	"context"
	"testing"

	infrastructurebuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/infrastructure"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

func TestBuildRunConfigUsesSettingsSource(t *testing.T) {
	cfg, err := BuildRunConfig(context.Background(), "tenant-1", fakeSettingsSource{}, BuildOptions{
		ID:       "run-1",
		Database: postgresDatabase(),
		Workload: workload(),
		Provider: deployment.Provider_PROVIDER_DOCKER,
		Infrastructure: infrastructurebuilder.BuildOptions{
			DefaultSizing: infrastructurebuilder.MachineSizing{CPUCores: 1, MemoryMB: 1024, DiskGB: 20},
		},
	})
	if err != nil {
		t.Fatalf("build run config: %v", err)
	}

	if cfg.GetInfrastructurePlan().GetSettings().GetDocker() == nil {
		t.Fatal("docker provider settings were not injected")
	}
	if got, want := cfg.GetAgentBootstrap().GetServerAddr(), "https://control.example"; got != want {
		t.Fatalf("server addr = %q, want %q", got, want)
	}
}

type fakeSettingsSource struct{}

func (fakeSettingsSource) ProviderSettings(context.Context, string, deployment.Provider) (*deployment.ProviderSettings, error) {
	return &deployment.ProviderSettings{
		Settings: &deployment.ProviderSettings_Docker{Docker: &deployment.Docker_Settings{}},
	}, nil
}

func (fakeSettingsSource) AgentBootstrap(context.Context) (*workflowpb.AgentBootstrap, error) {
	return &workflowpb.AgentBootstrap{ServerAddr: "https://control.example"}, nil
}
