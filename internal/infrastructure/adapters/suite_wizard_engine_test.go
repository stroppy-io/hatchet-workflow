package adapters

import (
	"context"
	"testing"

	workloadbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/workload"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domain "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

func TestSuiteWizardEnginePreservesCellMachineOverridesInPreviewAndBake(t *testing.T) {
	engine := NewSuiteWizardEngine(nil, &recordingSuiteBaker{})
	cellSpec := adaptersSuiteCell()
	draft := &models.SuiteWizardDraftRecord{
		Entity:   &common.Entity{Id: "draft-1", TenantId: "tenant-1", Name: "suite", AuthorId: "account-1"},
		Provider: deployment.Provider_PROVIDER_DOCKER,
		Cells: []*models.SuiteWizardDraftRecord_Cell{
			{Spec: cellSpec},
		},
	}

	cells, errs, ready, err := engine.Recompute(context.Background(), "tenant-1", draft)
	if err != nil {
		t.Fatalf("recompute suite draft: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("suite draft errors = %v", errs)
	}
	if !ready {
		t.Fatal("suite draft is not ready")
	}
	assertAdaptersDockerOverride(t, adaptersMachineByNodeID(cells[0].GetInfrastructurePlan().GetMachines(), workloadbuilder.RunnerNodeID))

	baker := &recordingSuiteBaker{}
	engine.baker = baker
	draft.Cells = cells
	draft.Ready = true
	_, _, _, err = engine.Bake(context.Background(), draft, &api.FinishSuiteWizardRequest{Start: true})
	if err != nil {
		t.Fatalf("bake suite draft: %v", err)
	}
	if len(baker.children) != 1 {
		t.Fatalf("baked children = %d, want 1", len(baker.children))
	}
	assertAdaptersDockerOverride(t, adaptersMachineByNodeID(baker.children[0].Run.GetInfrastructurePlan().GetMachines(), workloadbuilder.RunnerNodeID))
}

type recordingSuiteBaker struct {
	children []*BakedSuiteChild
}

func (b *recordingSuiteBaker) SaveSuite(_ context.Context, suite *models.SuiteRecord) (*models.SuiteRecord, error) {
	return suite, nil
}

func (b *recordingSuiteBaker) StartSuiteRun(_ context.Context, _ *models.SuiteRecord, children []*BakedSuiteChild, _ common.Trigger, _ uint32) (*models.SuiteRunRecord, func(context.Context) error, error) {
	b.children = children
	return &models.SuiteRunRecord{}, func(context.Context) error { return nil }, nil
}

func adaptersSuiteCell() *domain.SuiteCell {
	return &domain.SuiteCell{
		Id:      "cell-1",
		Name:    "cell 1",
		Enabled: true,
		Source: &domain.SuiteCell_InlineTest{InlineTest: &domain.Test{
			Database: adaptersDatabase(),
			Workload: adaptersWorkload(),
		}},
		MachineOverrides: []*deployment.MachinePlan{
			adaptersDockerMachineOverride(workloadbuilder.RunnerNodeID),
		},
	}
}

func adaptersDatabase() *domain.Database {
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

func adaptersWorkload() *domain.Workload {
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

func adaptersDockerMachineOverride(nodeID string) *deployment.MachinePlan {
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

func assertAdaptersDockerOverride(t *testing.T, machine *deployment.MachinePlan) {
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
	if got, want := adaptersQuotaRequestValue(machine, "host.disk.size"), uint64(88); got != want {
		t.Fatalf("docker disk quota = %d, want %d", got, want)
	}
}

func adaptersMachineByNodeID(machines []*deployment.MachinePlan, nodeID string) *deployment.MachinePlan {
	for _, machine := range machines {
		if machine.GetNodeId() == nodeID {
			return machine
		}
	}
	return nil
}

func adaptersQuotaRequestValue(machine *deployment.MachinePlan, name string) uint64 {
	for _, req := range machine.GetQuotaRequests() {
		if req.GetInfo().GetName() == name {
			return req.GetRequest()
		}
	}
	return 0
}
