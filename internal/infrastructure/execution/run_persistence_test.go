package execution

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domainpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPersistRunStateUpdatesRecord(t *testing.T) {
	started := timestamppb.New(time.Unix(10, 0))
	finished := timestamppb.New(time.Unix(20, 0))
	store := &fakeRunPersistenceStore{
		runs: map[string]*models.TestRunRecord{
			"run-1": {
				Entity: &common.Entity{Id: "run-1"},
				Status: common.Status_STATUS_PENDING,
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
}

// TestPersistRunSummaryMergesRecipeFacetsWithoutClobberingTiming asserts
// PersistRunSummary (a) writes the recipe-derived static facets onto the
// record's Summary and (b) never touches timing/progress fields
// PersistRunState's applyRunSummary already set — the two activities must
// compose, since RunRecipeWorkflow calls both across the run's lifetime.
func TestPersistRunSummaryMergesRecipeFacetsWithoutClobberingTiming(t *testing.T) {
	started := timestamppb.New(time.Unix(10, 0))
	store := &fakeRunPersistenceStore{
		runs: map[string]*models.TestRunRecord{
			"run-1": {
				Entity: &common.Entity{Id: "run-1"},
				Status: common.Status_STATUS_RUNNING,
				Summary: &models.TestRunRecord_Summary{
					StartedAt:   started,
					ProgressPct: 25,
				},
			},
		},
	}
	activities := NewRunPersistenceActivities(store)

	err := activities.PersistRunSummary(context.Background(), "run-1", &models.TestRunRecord_Summary{
		Provider:       deploymentpb.Provider_PROVIDER_YANDEX,
		NodeCount:      4,
		TopologyLabel:  "yandex · 4 nodes",
		DbKind:         domainpb.Database_KIND_POSTGRES,
		WorkloadName:   "insert+select",
		StroppyVersion: "1.2.3",
	})
	if err != nil {
		t.Fatalf("persist run summary: %v", err)
	}

	run := store.runs["run-1"]
	sum := run.GetSummary()
	if sum.GetProvider() != deploymentpb.Provider_PROVIDER_YANDEX {
		t.Errorf("provider = %s, want %s", sum.GetProvider(), deploymentpb.Provider_PROVIDER_YANDEX)
	}
	if sum.GetNodeCount() != 4 {
		t.Errorf("node_count = %d, want 4", sum.GetNodeCount())
	}
	if sum.GetTopologyLabel() != "yandex · 4 nodes" {
		t.Errorf("topology_label = %q, want %q", sum.GetTopologyLabel(), "yandex · 4 nodes")
	}
	if sum.GetDbKind() != domainpb.Database_KIND_POSTGRES {
		t.Errorf("db_kind = %s, want %s", sum.GetDbKind(), domainpb.Database_KIND_POSTGRES)
	}
	if sum.GetWorkloadName() != "insert+select" {
		t.Errorf("workload_name = %q, want %q", sum.GetWorkloadName(), "insert+select")
	}
	if sum.GetStroppyVersion() != "1.2.3" {
		t.Errorf("stroppy_version = %q, want %q", sum.GetStroppyVersion(), "1.2.3")
	}
	// Timing/progress previously set by PersistRunState must survive.
	if sum.GetStartedAt() == nil || !sum.GetStartedAt().AsTime().Equal(started.AsTime()) {
		t.Error("started_at was clobbered by PersistRunSummary")
	}
	if sum.GetProgressPct() != 25 {
		t.Errorf("progress_pct = %d, want 25 (must not be clobbered)", sum.GetProgressPct())
	}
	if run.GetEntity().GetTimings().GetUpdatedAt() == nil {
		t.Fatal("run updated_at was not touched")
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
	runs map[string]*models.TestRunRecord
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
