package run

import (
	"testing"

	"google.golang.org/protobuf/proto"

	infrastructurebuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/infrastructure"
	workloadbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/workload"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
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
	if got, want := len(run.GetTopologySpec().GetNodes()), 3; got != want {
		t.Fatalf("topology nodes = %d, want %d", got, want)
	}
	components := componentsByID(run.GetTopologySpec().GetComponents())
	if got, want := components[workloadbuilder.RunnerNodeID].GetKind(), topology.Component_KIND_WORKLOAD; got != want {
		t.Fatalf("runner component kind = %s, want %s", got, want)
	}
	if got, want := len(run.GetInfrastructurePlan().GetMachines()), 3; got != want {
		t.Fatalf("machines = %d, want %d", got, want)
	}
	runner := machineByNodeID(run.GetInfrastructurePlan().GetMachines(), workloadbuilder.RunnerNodeID)
	if runner == nil {
		t.Fatal("workload runner machine is missing")
	}
	if got, want := runner.GetDocker().GetResources().GetMemoryMb(), uint64(8192); got != want {
		t.Fatalf("runner memory = %d, want %d", got, want)
	}
	if got := run.GetRenderOverrides().GetLabels()["source"]; got != "wizard" {
		t.Fatalf("render override label = %q, want wizard", got)
	}
}

func TestBuildTestRunUsesMaxRunnerSizingFromThousandVUs(t *testing.T) {
	wl := workload()
	wl.Segments[0].Execution.Vus = proto.Uint32(1000)
	run, err := BuildTestRun(BuildOptions{
		ID:       "run-1",
		Database: postgresDatabase(),
		Workload: wl,
		Provider: deployment.Provider_PROVIDER_DOCKER,
		Infrastructure: infrastructurebuilder.BuildOptions{
			DefaultSizing: infrastructurebuilder.MachineSizing{CPUCores: 1, MemoryMB: 1024, DiskGB: 20},
		},
	})
	if err != nil {
		t.Fatalf("build test run: %v", err)
	}

	runner := machineByNodeID(run.GetInfrastructurePlan().GetMachines(), workloadbuilder.RunnerNodeID)
	if runner == nil {
		t.Fatal("workload runner machine is missing")
	}
	if got, want := runner.GetDocker().GetResources().GetCpuCores(), float64(16); got != want {
		t.Fatalf("runner cpu = %v, want %v", got, want)
	}
	if got, want := runner.GetDocker().GetResources().GetMemoryMb(), uint64(65536); got != want {
		t.Fatalf("runner memory = %d, want %d", got, want)
	}
}

func TestBuildTestRunKeepsUserRunnerSizingOverride(t *testing.T) {
	run, err := BuildTestRun(BuildOptions{
		ID:       "run-1",
		Database: postgresDatabase(),
		Workload: workload(),
		Provider: deployment.Provider_PROVIDER_DOCKER,
		Infrastructure: infrastructurebuilder.BuildOptions{
			DefaultSizing: infrastructurebuilder.MachineSizing{CPUCores: 1, MemoryMB: 1024, DiskGB: 20},
			MachineSizing: map[string]infrastructurebuilder.MachineSizing{
				workloadbuilder.RunnerNodeID: {CPUCores: 6, MemoryMB: 12288, DiskGB: 75},
			},
		},
	})
	if err != nil {
		t.Fatalf("build test run: %v", err)
	}

	runner := machineByNodeID(run.GetInfrastructurePlan().GetMachines(), workloadbuilder.RunnerNodeID)
	if runner == nil {
		t.Fatal("workload runner machine is missing")
	}
	if got, want := runner.GetDocker().GetResources().GetCpuCores(), float64(6); got != want {
		t.Fatalf("runner cpu = %v, want %v", got, want)
	}
	if got, want := runner.GetDocker().GetResources().GetMemoryMb(), uint64(12288); got != want {
		t.Fatalf("runner memory = %d, want %d", got, want)
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
		Protocol: domain.Workload_PROTOCOL_PG,
		Segments: []*domain.Workload_Segment{{
			Name:   "workload",
			Script: "tpcc/tx",
			Execution: &domain.Workload_Execution{
				Vus: proto.Uint32(1),
				Limit: &domain.Workload_Execution_Duration{
					Duration: "1m",
				},
			},
		}},
	}
}

func componentsByID(components []*topology.Component) map[string]*topology.Component {
	result := make(map[string]*topology.Component, len(components))
	for _, component := range components {
		result[component.GetId()] = component
	}
	return result
}

func machineByNodeID(machines []*deployment.MachinePlan, nodeID string) *deployment.MachinePlan {
	for _, machine := range machines {
		if machine.GetNodeId() == nodeID {
			return machine
		}
	}
	return nil
}
