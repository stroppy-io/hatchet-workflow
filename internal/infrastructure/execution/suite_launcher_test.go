package execution

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

func TestSuiteRunLauncherPersistsParentBeforeWorkflowStart(t *testing.T) {
	var events []string
	children := &recordingChildPersister{events: &events}
	suites := &recordingSuitePersister{events: &events}
	client := &recordingSuiteWorkflowClient{events: &events}
	launcher := &SuiteRunLauncher{
		tc:       client,
		resolver: staticCellResolver{},
		children: children,
		suites:   suites,
	}
	run := &models.SuiteRunRecord{
		Entity: &common.Entity{Id: "suite-run-1", TenantId: "tenant-1", AuthorId: "account-1"},
	}

	starter, err := launcher.Launch(context.Background(), run, &domain.Suite{
		Id:       "suite-1",
		Provider: deployment.Provider_PROVIDER_DOCKER,
		Cells: []*domain.SuiteCell{
			{Id: "cell-1", Name: "cell 1", Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("launch suite run: %v", err)
	}

	if got, want := fmt.Sprint(events), "[child suite]"; got != want {
		t.Fatalf("events = %s, want %s", got, want)
	}
	if suites.run == nil {
		t.Fatal("suite run was not persisted")
	}
	if len(suites.run.GetChildren()) != 1 {
		t.Fatalf("persisted suite children = %d, want 1", len(suites.run.GetChildren()))
	}
	if starter == nil {
		t.Fatal("starter was nil")
	}
	if err := starter(context.Background()); err != nil {
		t.Fatalf("start suite workflow: %v", err)
	}
	if got, want := fmt.Sprint(events), "[child suite start]"; got != want {
		t.Fatalf("events after start = %s, want %s", got, want)
	}
	if len(client.req.GetRuns()) != 1 {
		t.Fatalf("workflow request runs = %d, want 1", len(client.req.GetRuns()))
	}
}

func TestSuiteRunLauncherMarksRecordsFailedWhenWorkflowStartFails(t *testing.T) {
	var events []string
	children := &recordingChildPersister{events: &events}
	suites := &recordingSuitePersister{events: &events}
	client := &recordingSuiteWorkflowClient{events: &events, err: errors.New("temporal unavailable")}
	launcher := &SuiteRunLauncher{
		tc:       client,
		resolver: staticCellResolver{},
		children: children,
		suites:   suites,
	}
	run := &models.SuiteRunRecord{
		Entity: &common.Entity{Id: "suite-run-1", TenantId: "tenant-1", AuthorId: "account-1"},
	}

	starter, err := launcher.Launch(context.Background(), run, &domain.Suite{
		Id:       "suite-1",
		Provider: deployment.Provider_PROVIDER_DOCKER,
		Cells: []*domain.SuiteCell{
			{Id: "cell-1", Name: "cell 1", Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("launch suite run: %v", err)
	}
	if err := starter(context.Background()); err == nil {
		t.Fatal("expected workflow start error")
	}

	if got := suites.run.GetStatus(); got != common.Status_STATUS_FAILED {
		t.Fatalf("suite status = %s, want %s", got, common.Status_STATUS_FAILED)
	}
	if got := suites.run.GetChildren()[0].GetStatus(); got != common.Status_STATUS_FAILED {
		t.Fatalf("suite child status = %s, want %s", got, common.Status_STATUS_FAILED)
	}
	if children.updated == nil {
		t.Fatal("child run was not updated")
	}
	if got := children.updated.GetStatus(); got != common.Status_STATUS_FAILED {
		t.Fatalf("child run status = %s, want %s", got, common.Status_STATUS_FAILED)
	}
}

type staticCellResolver struct{}

func (staticCellResolver) ResolveCell(context.Context, string, *domain.SuiteCell) (*domain.Test, error) {
	return &domain.Test{
		Database: &domain.Database{
			Kind: domain.Database_KIND_POSTGRES,
			Source: &domain.Database_Params{
				Params: &domain.DatabaseParams{
					Version: "16",
					Engine: &domain.DatabaseParams_Postgres{
						Postgres: &domain.PostgresParams{Replicas: 1},
					},
				},
			},
		},
		Workload: &domain.Workload{
			Script:   "tpcc/tx",
			Protocol: domain.Workload_PROTOCOL_PG,
			Execution: &domain.Workload_Execution{
				Vus: 1,
				Limit: &domain.Workload_Execution_Duration{
					Duration: "1m",
				},
			},
		},
	}, nil
}

type recordingChildPersister struct {
	events  *[]string
	updated *models.TestRunRecord
}

func (p *recordingChildPersister) CreateChildRun(context.Context, *models.TestRunRecord) error {
	*p.events = append(*p.events, "child")
	return nil
}

func (p *recordingChildPersister) UpdateChildRun(_ context.Context, run *models.TestRunRecord) error {
	*p.events = append(*p.events, "child-update")
	p.updated = run
	return nil
}

type recordingSuitePersister struct {
	events *[]string
	run    *models.SuiteRunRecord
}

func (p *recordingSuitePersister) SaveSuiteRun(_ context.Context, run *models.SuiteRunRecord) error {
	*p.events = append(*p.events, "suite")
	p.run = run
	return nil
}

type recordingSuiteWorkflowClient struct {
	events *[]string
	req    *workflowpb.SuiteWorkflowRequest
	err    error
}

func (c *recordingSuiteWorkflowClient) SuiteWorkflow(context.Context, *workflowpb.SuiteWorkflowRequest, ...*workflowpb.SuiteWorkflowOptions) (*workflowpb.SuiteWorkflowResponse, error) {
	return &workflowpb.SuiteWorkflowResponse{}, nil
}

func (c *recordingSuiteWorkflowClient) SuiteWorkflowAsync(_ context.Context, req *workflowpb.SuiteWorkflowRequest, _ ...*workflowpb.SuiteWorkflowOptions) (workflowpb.SuiteWorkflowRun, error) {
	*c.events = append(*c.events, "start")
	c.req = req
	if c.err != nil {
		return nil, c.err
	}
	return nil, nil
}

func (*recordingSuiteWorkflowClient) GetSuiteWorkflow(context.Context, string, string) workflowpb.SuiteWorkflowRun {
	return nil
}

func (*recordingSuiteWorkflowClient) CancelWorkflow(context.Context, string, string) error {
	return nil
}

func (*recordingSuiteWorkflowClient) TerminateWorkflow(context.Context, string, string, string, ...interface{}) error {
	return nil
}
