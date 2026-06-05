package execution

import (
	"context"
	"testing"

	"github.com/stroppy-io/schemapb/schemapb"
	runbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	workloadbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/workload"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domain "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"google.golang.org/protobuf/proto"
)

func TestTestWizardEnginePreservesMachineOverrides(t *testing.T) {
	engine := NewTestWizardEngine()
	db := wizardDatabase()
	wl := wizardWorkload()
	draft := &models.TestWizardDraftRecord{
		Provider:         deployment.Provider_PROVIDER_DOCKER,
		Database:         db,
		Workload:         wl,
		MachineOverrides: completeDockerMachineOverrides(t, db, wl, workloadbuilder.RunnerNodeID),
	}

	if err := engine.Compute(context.Background(), "tenant-1", draft); err != nil {
		t.Fatalf("compute wizard draft: %v", err)
	}
	if !draft.GetReady() {
		t.Fatalf("draft is not ready: %v", draft.GetErrors())
	}

	runner := testMachineByNodeID(draft.GetInfrastructurePlan().GetMachines(), workloadbuilder.RunnerNodeID)
	assertDockerOverride(t, runner)

	run, err := engine.Bake(context.Background(), draft)
	if err != nil {
		t.Fatalf("bake wizard draft: %v", err)
	}
	assertDockerOverride(t, testMachineByNodeID(run.GetInfrastructurePlan().GetMachines(), workloadbuilder.RunnerNodeID))
}

func TestTestWizardEngineRequiresMachineOverrides(t *testing.T) {
	engine := NewTestWizardEngine()
	draft := &models.TestWizardDraftRecord{
		Provider: deployment.Provider_PROVIDER_DOCKER,
		Database: wizardDatabase(),
		Workload: wizardWorkload(),
	}

	if err := engine.Compute(context.Background(), "tenant-1", draft); err != nil {
		t.Fatalf("compute wizard draft: %v", err)
	}
	if draft.GetReady() {
		t.Fatal("draft is ready without explicit machine overrides")
	}
	if !hasWizardError(draft.GetErrors(), "machine_overrides") {
		t.Fatalf("draft errors = %v, want machine_overrides", draft.GetErrors())
	}
}

func wizardDatabase() *domain.Database {
	return &domain.Database{
		Kind: domain.Database_KIND_POSTGRES,
		Source: &domain.Database_Params{
			Params: &domain.DatabaseParams{
				Version: "16",
				Engine: &domain.DatabaseParams_Postgres{
					Postgres: &domain.PostgresParams{},
				},
			},
		},
	}
}

func wizardWorkload() *domain.Workload {
	return &domain.Workload{
		Script:   "tpcc/tx",
		Protocol: domain.Workload_PROTOCOL_PG,
		Execution: &domain.Workload_Execution{
			Vus: 16,
			Limit: &domain.Workload_Execution_Duration{
				Duration: "1m",
			},
		},
	}
}

func completeDockerMachineOverrides(t *testing.T, db *domain.Database, wl *domain.Workload, customNodeID string) []*deployment.MachinePlan {
	t.Helper()
	run, err := runbuilder.BuildTestRun(runbuilder.BuildOptions{
		ID:       "preview",
		Database: db,
		Workload: wl,
		Provider: deployment.Provider_PROVIDER_DOCKER,
	})
	if err != nil {
		t.Fatalf("build preview run: %v", err)
	}
	out := make([]*deployment.MachinePlan, 0, len(run.GetInfrastructurePlan().GetMachines()))
	for _, machine := range run.GetInfrastructurePlan().GetMachines() {
		if machine.GetNodeId() == customNodeID {
			out = append(out, dockerMachineOverride(customNodeID))
			continue
		}
		out = append(out, proto.Clone(machine).(*deployment.MachinePlan))
	}
	return out
}

func dockerMachineOverride(nodeID string) *deployment.MachinePlan {
	return &deployment.MachinePlan{
		NodeId: nodeID,
		ProviderParams: &deployment.MachinePlan_Docker{Docker: &deployment.Docker_Container{
			Image: "custom-runner:latest",
			Resources: &deployment.Docker_Resources{
				CpuCores: 7,
				MemoryMb: 14336,
			},
		}},
		QuotaRequests: []*deployment.Quota_Request{
			{
				Info: &deployment.Quota_Info{
					Provider: deployment.Provider_PROVIDER_DOCKER,
					Name:     "host.disk.size",
					Units:    "GiB",
				},
				Request: 88,
			},
		},
	}
}

func assertDockerOverride(t *testing.T, machine *deployment.MachinePlan) {
	t.Helper()
	if machine == nil {
		t.Fatal("overridden machine is missing")
	}
	if got, want := machine.GetDocker().GetImage(), "custom-runner:latest"; got != want {
		t.Fatalf("docker image = %q, want %q", got, want)
	}
	if got, want := machine.GetDocker().GetResources().GetCpuCores(), float64(7); got != want {
		t.Fatalf("docker cpu = %v, want %v", got, want)
	}
	if got, want := machine.GetDocker().GetResources().GetMemoryMb(), uint64(14336); got != want {
		t.Fatalf("docker memory = %d, want %d", got, want)
	}
	if got, want := testQuotaRequestValue(machine, "host.disk.size"), uint64(88); got != want {
		t.Fatalf("docker disk quota = %d, want %d", got, want)
	}
}

func testMachineByNodeID(machines []*deployment.MachinePlan, nodeID string) *deployment.MachinePlan {
	for _, machine := range machines {
		if machine.GetNodeId() == nodeID {
			return machine
		}
	}
	return nil
}

func testQuotaRequestValue(machine *deployment.MachinePlan, name string) uint64 {
	for _, req := range machine.GetQuotaRequests() {
		if req.GetInfo().GetName() == name {
			return req.GetRequest()
		}
	}
	return 0
}

func hasWizardError(errs []*schemapb.FieldError, field string) bool {
	for _, err := range errs {
		if err.GetField() == field {
			return true
		}
	}
	return false
}
