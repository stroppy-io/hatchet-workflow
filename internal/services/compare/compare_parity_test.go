package compare

import (
	"testing"

	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domainpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// runFixtureWithEveryField builds a models.Run with every field the spec §6
// data-parity chart cares about set to a distinguishable, non-zero value, so
// a dropped mapping shows up as an assertion failure rather than a
// false-positive zero-value match.
func runFixtureWithEveryField(t *testing.T) *models.Run {
	t.Helper()
	return &models.Run{
		Entity: &commonpb.Entity{
			Id:         "run-1",
			Name:       "run-name",
			TenantId:   "tenant-1",
			AuthorId:   "author-1",
			IsFavorite: true,
			Timings: &commonpb.Timings{
				CreatedAt: timestamppb.New(timestamppb.Now().AsTime()),
			},
		},
		Status:          commonpb.Status_STATUS_COMPLETED,
		Trigger:         commonpb.Trigger_TRIGGER_MANUAL,
		WorkflowId:      "workflow-1",
		WorkflowVersion: "3",
		InTenantRating:  true,
		InGlobalRating:  true,
		Topology: &models.RunTopology{
			Provider: "docker",
			Nodes: []*models.RunTopology_MachineNode{
				{NodeId: "db-0", Group: "db", Ip: "10.0.0.1", Status: commonpb.Status_STATUS_COMPLETED},
			},
		},
		Observability: &models.ObservabilityRefs{
			MetricsQueryKey:     "run-1",
			LogsQueryKey:        "run-1",
			GrafanaDashboardUid: "uid-1",
		},
		Summary: &models.Run_Summary{
			DbKind:           domainpb.Database_KIND_POSTGRES,
			DbPresetId:       "db-preset-1",
			DbPresetName:     "db-preset-name",
			WorkloadPresetId: "workload-preset-1",
			WorkloadName:     "workload-name",
			StroppyVersion:   "1.2.3",
			WorkloadProtocol: domainpb.Workload_PROTOCOL_PG,
			TestPresetId:     "test-preset-1",
			TestPresetName:   "test-preset-name",
			TopologyLabel:    "topology-label",
			NodeCount:        3,
			Provider:         deploymentpb.Provider_PROVIDER_DOCKER,
			ProgressPct:      100,
			StartedAt:        timestamppb.New(timestamppb.Now().AsTime()),
			FinishedAt:       timestamppb.New(timestamppb.Now().AsTime()),
			Duration:         durationpb.New(60_000_000_000),
		},
	}
}

// TestCompareParity_RunColumnFieldsFromRunFixture guards spec §6.D: every
// RunColumn field Compare.tsx renders must be sourced 1:1 from the models.Run
// fixture via the real runColumn mapping function in this package (not a
// mock-of-a-mock).
func TestCompareParity_RunColumnFieldsFromRunFixture(t *testing.T) {
	run := runFixtureWithEveryField(t)
	col := runColumn(run)

	if got, want := col.GetRunId(), run.GetEntity().GetId(); got != want {
		t.Errorf("§6.D parity: RunColumn.run_id = %q, want %q", got, want)
	}
	if got, want := col.GetName(), run.GetEntity().GetName(); got != want {
		t.Errorf("§6.D parity: RunColumn.name = %q, want %q", got, want)
	}
	if got, want := col.GetStatus(), run.GetStatus(); got != want {
		t.Errorf("§6.D parity: RunColumn.status = %v, want %v", got, want)
	}
	if got, want := col.GetDbKind(), run.GetSummary().GetDbKind(); got != want {
		t.Errorf("§6.D parity: RunColumn.db_kind = %v, want %v", got, want)
	}
	if got, want := col.GetDbName(), run.GetSummary().GetDbPresetName(); got != want {
		t.Errorf("§6.D parity: RunColumn.db_name = %q, want %q", got, want)
	}
	if got, want := col.GetWorkloadName(), run.GetSummary().GetWorkloadName(); got != want {
		t.Errorf("§6.D parity: RunColumn.workload_name = %q, want %q", got, want)
	}
	if got, want := col.GetStroppyVersion(), run.GetSummary().GetStroppyVersion(); got != want {
		t.Errorf("§6.D parity: RunColumn.stroppy_version = %q, want %q", got, want)
	}
	if got, want := col.GetProvider(), run.GetSummary().GetProvider(); got != want {
		t.Errorf("§6.D parity: RunColumn.provider = %v, want %v", got, want)
	}
	if got, want := col.GetTopologyLabel(), run.GetSummary().GetTopologyLabel(); got != want {
		t.Errorf("§6.D parity: RunColumn.topology_label = %q, want %q", got, want)
	}
	if got, want := col.GetNodeCount(), run.GetSummary().GetNodeCount(); got != want {
		t.Errorf("§6.D parity: RunColumn.node_count = %d, want %d", got, want)
	}
	if got, want := col.GetStartedAt().AsTime(), run.GetSummary().GetStartedAt().AsTime(); !got.Equal(want) {
		t.Errorf("§6.D parity: RunColumn.started_at = %v, want %v", got, want)
	}
	if got, want := col.GetDuration().AsDuration(), run.GetSummary().GetDuration().AsDuration(); got != want {
		t.Errorf("§6.D parity: RunColumn.duration = %v, want %v", got, want)
	}
}
