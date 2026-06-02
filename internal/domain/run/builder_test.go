package run

import (
	"testing"

	infrastructurebuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/infrastructure"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

func TestBuildTestRun(t *testing.T) {
	overrides := &deployment.RenderOverrideSet{
		Labels: map[string]string{"source": "wizard"},
	}
	run, err := BuildTestRun(BuildOptions{
		ID:              "run-1",
		Database:        postgresDatabase(),
		Workload:        workload(),
		Provider:        deployment.Provider_PROVIDER_DOCKER,
		RenderOverrides: overrides,
		Infrastructure: infrastructurebuilder.BuildOptions{
			DefaultSizing: infrastructurebuilder.MachineSizing{CPUCores: 1, MemoryMB: 1024, DiskGB: 20},
		},
	})
	if err != nil {
		t.Fatalf("build test run: %v", err)
	}

	if err := run.Validate(); err != nil {
		t.Fatalf("run is invalid: %v", err)
	}
	if got, want := len(run.GetTopologySpec().GetNodes()), 2; got != want {
		t.Fatalf("topology nodes = %d, want %d", got, want)
	}
	if got, want := len(run.GetInfrastructurePlan().GetMachines()), 2; got != want {
		t.Fatalf("machines = %d, want %d", got, want)
	}
	if got := run.GetRenderOverrides().GetLabels()["source"]; got != "wizard" {
		t.Fatalf("render override label = %q, want wizard", got)
	}
}

func postgresDatabase() *domain.Database {
	return &domain.Database{
		Kind: domain.Database_KIND_POSTGRES,
		Source: &domain.Database_Params{
			Params: &domain.DatabaseParams{
				Version: "16",
				Engine: &domain.DatabaseParams_Postgres{
					Postgres: &domain.PostgresParams{Replicas: 1},
				},
			},
		},
	}
}

func workload() *domain.Workload {
	return &domain.Workload{
		Script:   "tpcc/tx",
		Protocol: domain.Workload_PROTOCOL_PG,
		Execution: &domain.Workload_Execution{
			Vus: 1,
			Limit: &domain.Workload_Execution_Duration{
				Duration: "1m",
			},
		},
	}
}
