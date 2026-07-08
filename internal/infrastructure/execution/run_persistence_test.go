package execution

import (
	"context"
	"fmt"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domainpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

func TestPersistRunStateUpdatesRecord(t *testing.T) {
	started := timestamppb.New(time.Unix(10, 0))
	finished := timestamppb.New(time.Unix(20, 0))
	store := &fakeRunPersistenceStore{
		runs: map[string]*models.Run{
			"run-1": {
				Entity: &commonpb.Entity{Id: "run-1"},
				Status: commonpb.Status_STATUS_PENDING,
			},
		},
	}
	activities := NewRunPersistenceActivities(store)

	// infrastructureState/deploymentPlan are accepted for RuntimeActivities
	// interface stability but are documented no-ops on a models.Run-backed
	// store (Run has no such fields — see PersistRunState's own doc
	// comment), so this test only asserts the fields Run does carry.
	err := activities.PersistRunState(
		context.Background(),
		"run-1",
		&workflowpb.RunState{
			Status: commonpb.Status_STATUS_RUNNING,
			Stages: []*workflowpb.Stage{
				{Name: "infrastructure", Status: commonpb.Status_STATUS_COMPLETED, StartedAt: started, FinishedAt: finished},
				{Name: "workload", Status: commonpb.Status_STATUS_RUNNING, StartedAt: finished},
			},
		},
		&deploymentpb.InfrastructureState{Provider: deploymentpb.Provider_PROVIDER_DOCKER},
		&deploymentpb.DeploymentPlan{Labels: map[string]string{"source": "test"}},
	)
	if err != nil {
		t.Fatalf("persist run state: %v", err)
	}

	run := store.runs["run-1"]
	if got := run.GetStatus(); got != commonpb.Status_STATUS_RUNNING {
		t.Fatalf("run status = %s, want %s", got, commonpb.Status_STATUS_RUNNING)
	}
	if got := run.GetSummary().GetProgressPct(); got != 50 {
		t.Fatalf("run progress = %d, want 50", got)
	}
	if run.GetSummary().GetStartedAt() == nil {
		t.Fatal("run started_at was not persisted")
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
		runs: map[string]*models.Run{
			"run-1": {
				Entity: &commonpb.Entity{Id: "run-1"},
				Status: commonpb.Status_STATUS_RUNNING,
				Summary: &models.Run_Summary{
					StartedAt:   started,
					ProgressPct: 25,
				},
			},
		},
	}
	activities := NewRunPersistenceActivities(store)

	err := activities.PersistRunSummary(context.Background(), "run-1", &models.Run_Summary{
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

// TestPersistRunCompiledPlan_StoresOnRunRecord asserts PersistRunCompiledPlan
// (SP-E Task 3's new activity) stores the compiled plan onto the run
// record's compiled_plan field — the fix for the plan otherwise living only
// in workflow memory and never being durably persisted.
func TestPersistRunCompiledPlan_StoresOnRunRecord(t *testing.T) {
	store := &fakeRunPersistenceStore{
		runs: map[string]*models.Run{
			"run-1": {Entity: &commonpb.Entity{Id: "run-1", TenantId: "t1"}},
		},
	}
	activities := NewRunPersistenceActivities(store)

	plan := &dslpb.CompiledPlan{Provider: &dslpb.ProviderRef{Name: "docker"}}
	if err := activities.PersistRunCompiledPlan(context.Background(), "run-1", plan); err != nil {
		t.Fatalf("persist run compiled plan: %v", err)
	}

	got := store.runs["run-1"]
	if got.GetCompiledPlan().GetProvider().GetName() != "docker" {
		t.Fatalf("compiled_plan.provider.name = %q, want %q", got.GetCompiledPlan().GetProvider().GetName(), "docker")
	}
	if got.GetEntity().GetTimings().GetUpdatedAt() == nil {
		t.Fatal("run updated_at was not touched")
	}
}

// TestPersistRunCompiledPlan_UnknownRunReturnsError asserts a lookup failure
// (unknown run id) propagates rather than being swallowed — mirrors every
// other Persist* activity's error-propagation contract.
func TestPersistRunCompiledPlan_UnknownRunReturnsError(t *testing.T) {
	store := &fakeRunPersistenceStore{runs: map[string]*models.Run{}}
	activities := NewRunPersistenceActivities(store)

	err := activities.PersistRunCompiledPlan(context.Background(), "missing", &dslpb.CompiledPlan{})
	if err == nil {
		t.Fatal("expected an error for an unknown run id")
	}
}

func TestNextRunStatusDoesNotDowngradeDurableTerminalState(t *testing.T) {
	tests := []struct {
		name     string
		current  commonpb.Status
		incoming commonpb.Status
		want     commonpb.Status
	}{
		{
			name:     "completed ignores running",
			current:  commonpb.Status_STATUS_COMPLETED,
			incoming: commonpb.Status_STATUS_RUNNING,
			want:     commonpb.Status_STATUS_COMPLETED,
		},
		{
			name:     "failed ignores cancelled",
			current:  commonpb.Status_STATUS_FAILED,
			incoming: commonpb.Status_STATUS_CANCELLED,
			want:     commonpb.Status_STATUS_FAILED,
		},
		{
			name:     "cancelling ignores running",
			current:  commonpb.Status_STATUS_CANCELLING,
			incoming: commonpb.Status_STATUS_RUNNING,
			want:     commonpb.Status_STATUS_CANCELLING,
		},
		{
			name:     "terminal can move from cancelling",
			current:  commonpb.Status_STATUS_CANCELLING,
			incoming: commonpb.Status_STATUS_CANCELLED,
			want:     commonpb.Status_STATUS_CANCELLED,
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
	runs map[string]*models.Run
}

func (f *fakeRunPersistenceStore) RunRecord(_ context.Context, runID string) (*models.Run, error) {
	rec, ok := f.runs[runID]
	if !ok {
		return nil, fmt.Errorf("run %q not found", runID)
	}
	return rec, nil
}

func (f *fakeRunPersistenceStore) SaveRunRecord(_ context.Context, run *models.Run) error {
	f.runs[run.GetEntity().GetId()] = run
	return nil
}
