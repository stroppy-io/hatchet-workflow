package execution

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPersistRunStateUpdatesRecordAndSuiteAggregate(t *testing.T) {
	started := timestamppb.New(time.Unix(10, 0))
	finished := timestamppb.New(time.Unix(20, 0))
	store := &fakeRunPersistenceStore{
		runs: map[string]*models.TestRunRecord{
			"run-1": {
				Entity:     &common.Entity{Id: "run-1"},
				Status:     common.Status_STATUS_PENDING,
				SuiteRunId: "suite-run-1",
			},
			"run-2": {
				Entity: &common.Entity{Id: "run-2"},
				Status: common.Status_STATUS_PENDING,
			},
		},
		suites: map[string]*models.SuiteRunRecord{
			"suite-run-1": {
				Entity: &common.Entity{Id: "suite-run-1"},
				Status: common.Status_STATUS_PENDING,
				Children: []*models.SuiteRunRecord_ChildRun{
					{TestRunId: "run-1"},
					{TestRunId: "run-2"},
				},
			},
		},
	}
	activities := NewRunPersistenceActivities(store)

	err := activities.PersistRunState(
		context.Background(),
		"run-1",
		&workflowpb.RunState{
			Status: common.Status_STATUS_RUNNING,
			Stages: []*workflowpb.Stage{
				{Name: "infrastructure", Status: common.Status_STATUS_COMPLETED, StartedAt: started, FinishedAt: finished},
				{Name: "workload", Status: common.Status_STATUS_RUNNING, StartedAt: finished},
			},
		},
		&deploymentpb.InfrastructureState{Provider: deploymentpb.Provider_PROVIDER_DOCKER},
		&deploymentpb.DeploymentPlan{Labels: map[string]string{"source": "test"}},
	)
	if err != nil {
		t.Fatalf("persist run state: %v", err)
	}

	run := store.runs["run-1"]
	if got := run.GetStatus(); got != common.Status_STATUS_RUNNING {
		t.Fatalf("run status = %s, want %s", got, common.Status_STATUS_RUNNING)
	}
	if got := run.GetSummary().GetProgressPct(); got != 50 {
		t.Fatalf("run progress = %d, want 50", got)
	}
	if run.GetSummary().GetStartedAt() == nil {
		t.Fatal("run started_at was not persisted")
	}
	if run.GetInfrastructureState().GetProvider() != deploymentpb.Provider_PROVIDER_DOCKER {
		t.Fatal("infrastructure state was not persisted")
	}
	if run.GetDeploymentPlan().GetLabels()["source"] != "test" {
		t.Fatal("deployment plan was not persisted")
	}
	if got := len(run.GetRuntimeState().GetStages()); got != 2 {
		t.Fatalf("runtime_state stages = %d, want 2", got)
	}
	if run.GetEntity().GetTimings().GetUpdatedAt() == nil {
		t.Fatal("run updated_at was not touched")
	}

	suite := store.suites["suite-run-1"]
	if got := suite.GetStatus(); got != common.Status_STATUS_RUNNING {
		t.Fatalf("suite status = %s, want %s", got, common.Status_STATUS_RUNNING)
	}
	if got := suite.GetSummary().GetRunning(); got != 1 {
		t.Fatalf("suite running = %d, want 1", got)
	}
	if got := suite.GetSummary().GetPending(); got != 1 {
		t.Fatalf("suite pending = %d, want 1", got)
	}
	if got := suite.GetSummary().GetProgressPct(); got != 0 {
		t.Fatalf("suite progress = %d, want 0", got)
	}
	if got := suite.GetChildren()[0].GetStatus(); got != common.Status_STATUS_RUNNING {
		t.Fatalf("suite child status = %s, want %s", got, common.Status_STATUS_RUNNING)
	}
}

func TestPersistSuiteRunDerivesTerminalAggregate(t *testing.T) {
	store := &fakeRunPersistenceStore{
		runs: map[string]*models.TestRunRecord{
			"run-1": {Entity: &common.Entity{Id: "run-1"}, Status: common.Status_STATUS_COMPLETED},
			"run-2": {Entity: &common.Entity{Id: "run-2"}, Status: common.Status_STATUS_FAILED},
		},
		suites: map[string]*models.SuiteRunRecord{
			"suite-run-1": {
				Entity: &common.Entity{Id: "suite-run-1"},
				Status: common.Status_STATUS_RUNNING,
				Children: []*models.SuiteRunRecord_ChildRun{
					{TestRunId: "run-1"},
					{TestRunId: "run-2"},
				},
			},
		},
	}
	activities := NewRunPersistenceActivities(store)

	if err := activities.PersistSuiteRun(context.Background(), "suite-run-1", common.Status_STATUS_UNSPECIFIED); err != nil {
		t.Fatalf("persist suite run: %v", err)
	}

	suite := store.suites["suite-run-1"]
	if got := suite.GetStatus(); got != common.Status_STATUS_FAILED {
		t.Fatalf("suite status = %s, want %s", got, common.Status_STATUS_FAILED)
	}
	if got := suite.GetSummary().GetCompleted(); got != 1 {
		t.Fatalf("suite completed = %d, want 1", got)
	}
	if got := suite.GetSummary().GetFailed(); got != 1 {
		t.Fatalf("suite failed = %d, want 1", got)
	}
	if got := suite.GetSummary().GetProgressPct(); got != 100 {
		t.Fatalf("suite progress = %d, want 100", got)
	}
	if suite.GetSummary().GetFinishedAt() == nil {
		t.Fatal("suite finished_at was not persisted")
	}
}

func TestPersistSuiteRunDoesNotOverwriteTerminalStatus(t *testing.T) {
	store := &fakeRunPersistenceStore{
		runs: map[string]*models.TestRunRecord{
			"run-1": {Entity: &common.Entity{Id: "run-1"}, Status: common.Status_STATUS_COMPLETED},
		},
		suites: map[string]*models.SuiteRunRecord{
			"suite-run-1": {
				Entity: &common.Entity{Id: "suite-run-1"},
				Status: common.Status_STATUS_CANCELLED,
				Children: []*models.SuiteRunRecord_ChildRun{
					{TestRunId: "run-1"},
				},
			},
		},
	}
	activities := NewRunPersistenceActivities(store)

	if err := activities.PersistSuiteRun(context.Background(), "suite-run-1", common.Status_STATUS_UNSPECIFIED); err != nil {
		t.Fatalf("persist suite run: %v", err)
	}

	if got := store.suites["suite-run-1"].GetStatus(); got != common.Status_STATUS_CANCELLED {
		t.Fatalf("suite status = %s, want %s", got, common.Status_STATUS_CANCELLED)
	}
}

func TestPersistSuiteRunUpdatesParentSuiteSummary(t *testing.T) {
	started := timestamppb.New(time.Unix(100, 0))
	store := &fakeRunPersistenceStore{
		runs: map[string]*models.TestRunRecord{
			"run-1": {Entity: &common.Entity{Id: "run-1"}, Status: common.Status_STATUS_COMPLETED},
		},
		suites: map[string]*models.SuiteRunRecord{
			"suite-run-1": {
				Entity:  &common.Entity{Id: "suite-run-1", TenantId: "tenant-1", Timings: &common.Timings{CreatedAt: started}},
				SuiteId: "suite-1",
				Status:  common.Status_STATUS_RUNNING,
				Summary: &models.SuiteRunRecord_Summary{StartedAt: started},
				Children: []*models.SuiteRunRecord_ChildRun{
					{TestRunId: "run-1"},
				},
			},
		},
		suiteDefinitions: map[string]*models.SuiteRecord{
			"suite-1": {
				Entity:  &common.Entity{Id: "suite-1", TenantId: "tenant-1"},
				Summary: &models.SuiteRecord_Summary{RunCount: 1, LastRunAt: started, LastRunStatus: common.Status_STATUS_PENDING},
			},
		},
	}
	activities := NewRunPersistenceActivities(store)

	if err := activities.PersistSuiteRun(context.Background(), "suite-run-1", common.Status_STATUS_UNSPECIFIED); err != nil {
		t.Fatalf("persist suite run: %v", err)
	}

	parent := store.suiteDefinitions["suite-1"]
	if got := parent.GetSummary().GetLastRunStatus(); got != common.Status_STATUS_COMPLETED {
		t.Fatalf("parent last_run_status = %s, want completed", got)
	}
	if parent.GetSummary().GetLastRunAt() != started {
		t.Fatal("parent last_run_at changed unexpectedly")
	}
}

func TestNextRunStatusDoesNotDowngradeDurableTerminalState(t *testing.T) {
	tests := []struct {
		name     string
		current  common.Status
		incoming common.Status
		want     common.Status
	}{
		{
			name:     "completed ignores running",
			current:  common.Status_STATUS_COMPLETED,
			incoming: common.Status_STATUS_RUNNING,
			want:     common.Status_STATUS_COMPLETED,
		},
		{
			name:     "failed ignores cancelled",
			current:  common.Status_STATUS_FAILED,
			incoming: common.Status_STATUS_CANCELLED,
			want:     common.Status_STATUS_FAILED,
		},
		{
			name:     "cancelling ignores running",
			current:  common.Status_STATUS_CANCELLING,
			incoming: common.Status_STATUS_RUNNING,
			want:     common.Status_STATUS_CANCELLING,
		},
		{
			name:     "terminal can move from cancelling",
			current:  common.Status_STATUS_CANCELLING,
			incoming: common.Status_STATUS_CANCELLED,
			want:     common.Status_STATUS_CANCELLED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextRunStatus(tt.current, tt.incoming); got != tt.want {
				t.Fatalf("nextRunStatus() = %s, want %s", got, tt.want)
			}
		})
	}
}

type fakeRunPersistenceStore struct {
	runs             map[string]*models.TestRunRecord
	suites           map[string]*models.SuiteRunRecord
	suiteDefinitions map[string]*models.SuiteRecord
}

func (f *fakeRunPersistenceStore) RunRecord(_ context.Context, runID string) (*models.TestRunRecord, error) {
	rec, ok := f.runs[runID]
	if !ok {
		return nil, fmt.Errorf("run %q not found", runID)
	}
	return rec, nil
}

func (f *fakeRunPersistenceStore) SaveRunRecord(_ context.Context, run *models.TestRunRecord) error {
	f.runs[run.GetEntity().GetId()] = run
	return nil
}

func (f *fakeRunPersistenceStore) SuiteRun(_ context.Context, suiteRunID string) (*models.SuiteRunRecord, error) {
	rec, ok := f.suites[suiteRunID]
	if !ok {
		return nil, fmt.Errorf("suite run %q not found", suiteRunID)
	}
	return rec, nil
}

func (f *fakeRunPersistenceStore) SaveSuiteRun(_ context.Context, suiteRun *models.SuiteRunRecord) error {
	f.suites[suiteRun.GetEntity().GetId()] = suiteRun
	return nil
}

func (f *fakeRunPersistenceStore) SuiteRecord(_ context.Context, _, suiteID string) (*models.SuiteRecord, error) {
	if f.suiteDefinitions == nil {
		return nil, fmt.Errorf("suite %q not found", suiteID)
	}
	rec, ok := f.suiteDefinitions[suiteID]
	if !ok {
		return nil, fmt.Errorf("suite %q not found", suiteID)
	}
	return rec, nil
}

func (f *fakeRunPersistenceStore) SaveSuiteRecord(_ context.Context, suite *models.SuiteRecord) error {
	if f.suiteDefinitions == nil {
		f.suiteDefinitions = make(map[string]*models.SuiteRecord)
	}
	f.suiteDefinitions[suite.GetEntity().GetId()] = suite
	return nil
}
