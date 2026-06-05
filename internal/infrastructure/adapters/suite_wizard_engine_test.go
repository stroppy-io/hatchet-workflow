package adapters

import (
	"context"
	"testing"

	"github.com/stroppy-io/schemapb/schemapb"
	runbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	workloadbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/workload"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domain "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"google.golang.org/protobuf/proto"
)

func TestSuiteWizardEnginePreservesCellMachineOverridesInPreviewAndBake(t *testing.T) {
	engine := NewSuiteWizardEngine(nil, &recordingSuiteBaker{})
	cellSpec := adaptersSuiteCell(t)
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

func TestSuiteWizardEngineBakesSeededDraftBackIntoExistingSuite(t *testing.T) {
	baker := &recordingSuiteBaker{}
	engine := NewSuiteWizardEngine(nil, baker)
	cellSpec := adaptersSuiteCell(t)
	draft := &models.SuiteWizardDraftRecord{
		Entity:   &common.Entity{Id: "draft-1", TenantId: "tenant-1", Name: "suite", AuthorId: "account-1"},
		Provider: deployment.Provider_PROVIDER_DOCKER,
		SuiteId:  "suite-existing",
		Ready:    true,
		Cells: []*models.SuiteWizardDraftRecord_Cell{
			{
				Spec:       cellSpec,
				Database:   adaptersDatabase(),
				Workload:   adaptersWorkload(),
				Compatible: true,
				Ready:      true,
			},
		},
	}

	suite, _, _, err := engine.Bake(context.Background(), draft, &api.FinishSuiteWizardRequest{})
	if err != nil {
		t.Fatalf("bake seeded suite draft: %v", err)
	}
	if baker.replaceSuiteID != "suite-existing" {
		t.Fatalf("replaceSuiteID = %q, want suite-existing", baker.replaceSuiteID)
	}
	if got := suite.GetEntity().GetId(); got != "suite-existing" {
		t.Fatalf("suite entity id = %q, want suite-existing", got)
	}
	if got := suite.GetSpec().GetId(); got != "suite-existing" {
		t.Fatalf("suite spec id = %q, want suite-existing", got)
	}
}

func TestSuiteWizardEngineRequiresCellMachineOverrides(t *testing.T) {
	engine := NewSuiteWizardEngine(nil, &recordingSuiteBaker{})
	cellSpec := adaptersSuiteCell(t)
	cellSpec.MachineOverrides = nil
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
	if ready {
		t.Fatal("suite draft is ready without explicit machine overrides")
	}
	if !hasAdaptersError(cells[0].GetErrors(), "machine_overrides") {
		t.Fatalf("cell errors = %v, want machine_overrides", cells[0].GetErrors())
	}
	if !hasAdaptersError(errs, "cells") {
		t.Fatalf("draft errors = %v, want cells", errs)
	}
}

type recordingSuiteBaker struct {
	children       []*BakedSuiteChild
	replaceSuiteID string
}

func (b *recordingSuiteBaker) SaveSuite(_ context.Context, suite *models.SuiteRecord, replaceSuiteID string) (*models.SuiteRecord, error) {
	b.replaceSuiteID = replaceSuiteID
	return suite, nil
}

func (b *recordingSuiteBaker) StartSuiteRun(_ context.Context, _ *models.SuiteRecord, children []*BakedSuiteChild, _ common.Trigger, _ uint32) (*models.SuiteRunRecord, func(context.Context) error, error) {
	b.children = children
	return &models.SuiteRunRecord{}, func(context.Context) error { return nil }, nil
}

func adaptersSuiteCell(t *testing.T) *domain.SuiteCell {
	t.Helper()
	db := adaptersDatabase()
	wl := adaptersWorkload()
	return &domain.SuiteCell{
		Id:      "cell-1",
		Name:    "cell 1",
		Enabled: true,
		Source: &domain.SuiteCell_InlineTest{InlineTest: &domain.Test{
			Database: db,
			Workload: wl,
		}},
		MachineOverrides: completeAdaptersDockerMachineOverrides(t, db, wl, workloadbuilder.RunnerNodeID),
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

func completeAdaptersDockerMachineOverrides(t *testing.T, db *domain.Database, wl *domain.Workload, customNodeID string) []*deployment.MachinePlan {
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
			out = append(out, adaptersDockerMachineOverride(customNodeID))
			continue
		}
		out = append(out, proto.Clone(machine).(*deployment.MachinePlan))
	}
	return out
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

func hasAdaptersError(errs []*schemapb.FieldError, field string) bool {
	for _, err := range errs {
		if err.GetField() == field {
			return true
		}
	}
	return false
}
