package infrastructure

import (
	"testing"

	databasebuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/database"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

func TestBuildPlanDocker(t *testing.T) {
	spec := postgresSpec(t)
	plan, err := BuildPlan(spec, deployment.Provider_PROVIDER_DOCKER, BuildOptions{
		DefaultSizing: MachineSizing{CPUCores: 4, MemoryMB: 8192, DiskGB: 80},
		Docker: DockerOptions{
			Image:        "agent:test",
			PublishPorts: true,
			HostIP:       "127.0.0.1",
		},
	})
	if err != nil {
		t.Fatalf("build infrastructure plan: %v", err)
	}

	if err := plan.Validate(); err != nil {
		t.Fatalf("plan is invalid: %v", err)
	}
	if plan.GetSettings().GetDocker() == nil {
		t.Fatal("docker settings are missing")
	}
	if got, want := len(plan.GetMachines()), 2; got != want {
		t.Fatalf("machines = %d, want %d", got, want)
	}

	machine := plan.GetMachines()[0]
	if machine.GetDocker().GetImage() != "agent:test" {
		t.Fatalf("docker image = %q", machine.GetDocker().GetImage())
	}
	if got, want := machine.GetDocker().GetResources().GetMemoryMb(), uint64(8192); got != want {
		t.Fatalf("memory = %d, want %d", got, want)
	}
	if got := machine.GetDocker().GetEnv()["AGENT_MACHINE_ID"]; got != machine.GetNodeId() {
		t.Fatalf("AGENT_MACHINE_ID = %q, want node id", got)
	}
	if got, want := len(machine.GetDocker().GetPorts()), 1; got != want {
		t.Fatalf("ports = %d, want %d", got, want)
	}
}

func TestBuildPlanYandex(t *testing.T) {
	spec := postgresSpec(t)
	plan, err := BuildPlan(spec, deployment.Provider_PROVIDER_YANDEX, BuildOptions{
		DefaultSizing: MachineSizing{CPUCores: 2, MemoryMB: 3072, DiskGB: 50},
		Yandex: YandexOptions{
			Zone:     "ru-central1-a",
			PublicIP: true,
		},
	})
	if err != nil {
		t.Fatalf("build infrastructure plan: %v", err)
	}

	if err := plan.Validate(); err != nil {
		t.Fatalf("plan is invalid: %v", err)
	}

	vm := plan.GetMachines()[0].GetYandex()
	if got, want := vm.GetMemoryGb(), uint64(3); got != want {
		t.Fatalf("memory gb = %d, want %d", got, want)
	}
	if got, want := vm.GetBootDiskType(), "network-ssd"; got != want {
		t.Fatalf("boot disk type = %q, want %q", got, want)
	}
	if got, want := vm.GetInternalIp(), "auto"; got != want {
		t.Fatalf("internal ip = %q, want %q", got, want)
	}
}

func postgresSpec(t *testing.T) *topologypb.TopologySpec {
	t.Helper()

	spec, err := databasebuilder.BuildTopologySpec(&domain.Database{
		Kind: domain.Database_KIND_POSTGRES,
		Source: &domain.Database_Params{
			Params: &domain.DatabaseParams{
				Engine: &domain.DatabaseParams_Postgres{
					Postgres: &domain.PostgresParams{Replicas: 1},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("build postgres spec: %v", err)
	}
	return spec
}
