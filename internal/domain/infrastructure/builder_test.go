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
	if got := machine.GetDocker().GetEnv()["STROPPY_SERVER_ADDR"]; got != "" {
		t.Fatalf("docker plan contains runtime server addr %q", got)
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

func TestYandexMachineOverrideValidationAllowsProviderOwnedFieldsEmpty(t *testing.T) {
	vm := &deployment.Yandex_Vm{
		Cores:        2,
		MemoryGb:     4,
		BootDiskGb:   50,
		BootDiskType: "network-ssd",
	}
	if err := vm.Validate(); err != nil {
		t.Fatalf("validate sizing-only yandex vm override: %v", err)
	}
}

func TestBuildPlanAppliesDockerMachineOverride(t *testing.T) {
	spec := postgresSpec(t)
	nodeID := spec.GetNodes()[0].GetId()

	plan, err := BuildPlan(spec, deployment.Provider_PROVIDER_DOCKER, BuildOptions{
		DefaultSizing: MachineSizing{CPUCores: 1, MemoryMB: 1024, DiskGB: 20},
		MachineOverrides: []*deployment.MachinePlan{
			{
				NodeId: nodeID,
				ProviderParams: &deployment.MachinePlan_Docker{Docker: &deployment.Docker_Container{
					Image: "custom-postgres:16",
					Resources: &deployment.Docker_Resources{
						CpuCores: 6,
						MemoryMb: 12288,
					},
				}},
				QuotaRequests: []*deployment.Quota_Request{
					quotaRequest(deployment.Provider_PROVIDER_DOCKER, "host.disk.size", "GiB", 90),
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("build infrastructure plan: %v", err)
	}

	machine := machineByNodeID(plan.GetMachines(), nodeID)
	if machine == nil {
		t.Fatalf("machine %q is missing", nodeID)
	}
	if got, want := machine.GetDocker().GetImage(), "custom-postgres:16"; got != want {
		t.Fatalf("docker image = %q, want %q", got, want)
	}
	if got, want := machine.GetDocker().GetResources().GetCpuCores(), float64(6); got != want {
		t.Fatalf("docker cpu = %v, want %v", got, want)
	}
	if got, want := machine.GetDocker().GetResources().GetMemoryMb(), uint64(12288); got != want {
		t.Fatalf("docker memory = %d, want %d", got, want)
	}
	if got, want := quotaRequestValue(machine, "host.disk.size"), uint64(90); got != want {
		t.Fatalf("docker disk quota = %d, want %d", got, want)
	}
}

func TestBuildPlanDockerOverridePreservesRuntimeFields(t *testing.T) {
	spec := postgresSpec(t)
	nodeID := spec.GetNodes()[0].GetId()

	// A partial override that touches only sizing must not strip the agent
	// container's runtime-critical fields (privileged, init cmd, tmpfs).
	plan, err := BuildPlan(spec, deployment.Provider_PROVIDER_DOCKER, BuildOptions{
		MachineOverrides: []*deployment.MachinePlan{
			{
				NodeId: nodeID,
				ProviderParams: &deployment.MachinePlan_Docker{Docker: &deployment.Docker_Container{
					Resources: &deployment.Docker_Resources{CpuCores: 6, MemoryMb: 12288},
				}},
			},
		},
	})
	if err != nil {
		t.Fatalf("build infrastructure plan: %v", err)
	}

	container := machineByNodeID(plan.GetMachines(), nodeID).GetDocker()
	if !container.GetPrivileged() {
		t.Fatal("privileged was stripped by a partial override")
	}
	if got := container.GetCmd(); len(got) != 1 || got[0] != "/sbin/init" {
		t.Fatalf("init cmd = %v, want [/sbin/init]", got)
	}
	if _, ok := container.GetTmpfs()["/run"]; !ok {
		t.Fatalf("tmpfs mounts were stripped: %v", container.GetTmpfs())
	}
	if got, want := container.GetImage(), DefaultDockerImage; got != want {
		t.Fatalf("image = %q, want generated default %q", got, want)
	}
	if got, want := container.GetResources().GetCpuCores(), float64(6); got != want {
		t.Fatalf("override cpu = %v, want %v", got, want)
	}
}

func TestBuildPlanAppliesYandexMachineOverride(t *testing.T) {
	spec := postgresSpec(t)
	nodeID := spec.GetNodes()[0].GetId()

	plan, err := BuildPlan(spec, deployment.Provider_PROVIDER_YANDEX, BuildOptions{
		DefaultSizing: MachineSizing{CPUCores: 1, MemoryMB: 1024, DiskGB: 20},
		Yandex: YandexOptions{
			Zone:                "ru-central1-a",
			InternalIP:          "auto",
			PublicIP:            true,
			NetworkAcceleration: "software_accelerated",
		},
		MachineOverrides: []*deployment.MachinePlan{
			{
				NodeId: nodeID,
				ProviderParams: &deployment.MachinePlan_Yandex{Yandex: &deployment.Yandex_Vm{
					Cores:               8,
					MemoryGb:            32,
					BootDiskGb:          200,
					BootDiskType:        "network-hdd",
					Zone:                "ru-central1-b",
					InternalIp:          "10.0.0.42",
					PublicIp:            false,
					NetworkAcceleration: "standard",
				}},
			},
		},
	})
	if err != nil {
		t.Fatalf("build infrastructure plan: %v", err)
	}

	machine := machineByNodeID(plan.GetMachines(), nodeID)
	if machine == nil {
		t.Fatalf("machine %q is missing", nodeID)
	}
	vm := machine.GetYandex()
	if got, want := vm.GetCores(), uint32(8); got != want {
		t.Fatalf("yandex cores = %d, want %d", got, want)
	}
	if got, want := vm.GetMemoryGb(), uint64(32); got != want {
		t.Fatalf("yandex memory = %d, want %d", got, want)
	}
	if got, want := vm.GetBootDiskGb(), uint64(200); got != want {
		t.Fatalf("yandex boot disk = %d, want %d", got, want)
	}
	if got, want := vm.GetBootDiskType(), "network-hdd"; got != want {
		t.Fatalf("yandex boot disk type = %q, want %q", got, want)
	}
	if got, want := vm.GetZone(), "ru-central1-a"; got != want {
		t.Fatalf("yandex zone = %q, want provider-owned %q", got, want)
	}
	if got, want := vm.GetInternalIp(), "auto"; got != want {
		t.Fatalf("yandex internal ip = %q, want provider-owned %q", got, want)
	}
	if !vm.GetPublicIp() {
		t.Fatal("yandex public ip should stay provider-owned")
	}
	if got, want := vm.GetNetworkAcceleration(), "software_accelerated"; got != want {
		t.Fatalf("yandex network acceleration = %q, want provider-owned %q", got, want)
	}
	if got, want := quotaRequestValue(machine, "compute.hddDisks.size"), uint64(200); got != want {
		t.Fatalf("yandex disk quota = %d, want %d", got, want)
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

func machineByNodeID(machines []*deployment.MachinePlan, nodeID string) *deployment.MachinePlan {
	for _, machine := range machines {
		if machine.GetNodeId() == nodeID {
			return machine
		}
	}
	return nil
}

func quotaRequestValue(machine *deployment.MachinePlan, name string) uint64 {
	for _, req := range machine.GetQuotaRequests() {
		if req.GetInfo().GetName() == name {
			return req.GetRequest()
		}
	}
	return 0
}
