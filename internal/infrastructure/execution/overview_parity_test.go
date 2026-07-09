package execution

import (
	"context"
	"testing"

	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domainpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// runFixtureWithEveryField builds a models.Run with every field spec §6.B/
// §6.C care about set to a distinguishable, non-zero value, so the assertions
// below fail loudly if a mapping is silently dropped rather than passing on a
// coincidental zero-value match.
func runFixtureWithEveryField(t *testing.T) *models.Run {
	t.Helper()
	return &models.Run{
		Entity: &commonpb.Entity{
			Id:       "run-1",
			Name:     "run-name",
			TenantId: "tenant-1",
			AuthorId: "author-1",
			Timings: &commonpb.Timings{
				CreatedAt: timestamppb.New(timestamppb.Now().AsTime()),
			},
		},
		Status:          commonpb.Status_STATUS_COMPLETED,
		Trigger:         commonpb.Trigger_TRIGGER_MANUAL,
		WorkflowId:      "workflow-1",
		WorkflowVersion: "3",
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

// TestOverviewParity_SidebarAndTabFieldsFromRunFixture guards spec §6.B
// (RunInfoSidebar) and §6.C (RunDetail tabs): every field the sidebar/tabs
// render must survive OverviewReader.Get's real assembly from a models.Run
// fixture with every field populated. run is COMPLETED (persisted-terminal),
// so Get takes the fully-degraded persisted-record path without needing a
// live Temporal client (tc stays nil, matching the terminal-shortcut branch
// asserted by TestOverviewGetFallsBackToPersistedTerminalRecord).
func TestOverviewParity_SidebarAndTabFieldsFromRunFixture(t *testing.T) {
	run := runFixtureWithEveryField(t)
	reader := &OverviewReader{store: fakeSnapshotStore{run: run}}

	snap, err := reader.Get(context.Background(), run.GetEntity().GetId())
	if err != nil {
		t.Fatalf("overview get: %v", err)
	}

	// §6.B sidebar fields: owner/author, created, trigger, db kind/topology
	// label/db preset/test preset, workload name/protocol/stroppy version/
	// workload preset all read straight off entity/summary.
	got := snap.GetRun()
	if got.GetEntity().GetAuthorId() != run.GetEntity().GetAuthorId() {
		t.Errorf("§6.B parity: sidebar owner/author = %q, want %q", got.GetEntity().GetAuthorId(), run.GetEntity().GetAuthorId())
	}
	if got.GetEntity().GetTimings().GetCreatedAt().AsTime() != run.GetEntity().GetTimings().GetCreatedAt().AsTime() {
		t.Errorf("§6.B parity: sidebar created = %v, want %v", got.GetEntity().GetTimings().GetCreatedAt(), run.GetEntity().GetTimings().GetCreatedAt())
	}
	if got.GetTrigger() != run.GetTrigger() {
		t.Errorf("§6.B parity: sidebar trigger = %v, want %v", got.GetTrigger(), run.GetTrigger())
	}
	if got.GetSummary().GetDbKind() != run.GetSummary().GetDbKind() {
		t.Errorf("§6.B parity: sidebar db kind = %v, want %v", got.GetSummary().GetDbKind(), run.GetSummary().GetDbKind())
	}
	if got.GetSummary().GetTopologyLabel() != run.GetSummary().GetTopologyLabel() {
		t.Errorf("§6.B parity: sidebar topology label = %q, want %q", got.GetSummary().GetTopologyLabel(), run.GetSummary().GetTopologyLabel())
	}
	if got.GetSummary().GetDbPresetId() != run.GetSummary().GetDbPresetId() {
		t.Errorf("§6.B parity: sidebar db preset id = %q, want %q", got.GetSummary().GetDbPresetId(), run.GetSummary().GetDbPresetId())
	}
	if got.GetSummary().GetTestPresetId() != run.GetSummary().GetTestPresetId() {
		t.Errorf("§6.B parity: sidebar test preset id = %q, want %q", got.GetSummary().GetTestPresetId(), run.GetSummary().GetTestPresetId())
	}
	if got.GetSummary().GetWorkloadName() != run.GetSummary().GetWorkloadName() {
		t.Errorf("§6.B parity: sidebar workload name = %q, want %q", got.GetSummary().GetWorkloadName(), run.GetSummary().GetWorkloadName())
	}
	if got.GetSummary().GetWorkloadProtocol() != run.GetSummary().GetWorkloadProtocol() {
		t.Errorf("§6.B parity: sidebar protocol = %v, want %v", got.GetSummary().GetWorkloadProtocol(), run.GetSummary().GetWorkloadProtocol())
	}
	if got.GetSummary().GetStroppyVersion() != run.GetSummary().GetStroppyVersion() {
		t.Errorf("§6.B parity: sidebar stroppy version = %q, want %q", got.GetSummary().GetStroppyVersion(), run.GetSummary().GetStroppyVersion())
	}
	if got.GetSummary().GetWorkloadPresetId() != run.GetSummary().GetWorkloadPresetId() {
		t.Errorf("§6.B parity: sidebar workload preset id = %q, want %q", got.GetSummary().GetWorkloadPresetId(), run.GetSummary().GetWorkloadPresetId())
	}
	// suiteRunId is intentionally absent from models.Run — dropped per §6.B/§5.

	// §6.C tabs.
	if snap.GetOverview() == nil {
		t.Error("§6.C parity: Overview tab payload is nil")
	}
	if snap.GetOverview().GetStatus() != run.GetStatus() {
		t.Errorf("§6.C parity: Overview tab status = %v, want %v", snap.GetOverview().GetStatus(), run.GetStatus())
	}
	if len(snap.GetTopology().GetRuntimeNodes()) == 0 {
		t.Error("§6.C parity: Topology tab has no runtime nodes, want nodes derived from run.topology")
	}
	if len(snap.GetOverview().GetWorkers()) == 0 {
		t.Error("§6.C parity: Agents tab (Overview.workers) is empty, want workers sourced from run.topology.nodes (not the always-empty InfrastructureState path)")
	} else if got := snap.GetOverview().GetWorkers()[0].GetMachineId(); got != "db-0" {
		t.Errorf("§6.C parity: Agents tab worker machine_id = %q, want %q (from run.topology.nodes)", got, "db-0")
	}

	// Logs/Metrics/Grafana tabs are keyed off ObservabilityRefs, unchanged
	// from entity.id-keyed VictoriaLogs/VictoriaMetrics/relay lookups (§6.C,
	// "preserved (тривиально)") — guard that the refs survive the round trip
	// on the returned run.
	if got.GetObservability().GetLogsQueryKey() != run.GetObservability().GetLogsQueryKey() {
		t.Errorf("§6.C parity: Logs tab query key = %q, want %q", got.GetObservability().GetLogsQueryKey(), run.GetObservability().GetLogsQueryKey())
	}
	if got.GetObservability().GetMetricsQueryKey() != run.GetObservability().GetMetricsQueryKey() {
		t.Errorf("§6.C parity: Metrics tab query key = %q, want %q", got.GetObservability().GetMetricsQueryKey(), run.GetObservability().GetMetricsQueryKey())
	}
	if got.GetObservability().GetGrafanaDashboardUid() != run.GetObservability().GetGrafanaDashboardUid() {
		t.Errorf("§6.C parity: Grafana tab dashboard uid = %q, want %q", got.GetObservability().GetGrafanaDashboardUid(), run.GetObservability().GetGrafanaDashboardUid())
	}

	// Config tab (§6.C "fixed" row): guarded separately by web/src/services/
	// run_parity.test.ts (or, absent a web test harness, noted as an
	// unguardable finding) — Run.baked/Run.compiled_plan are the new source;
	// this Go-side test only asserts the fields survive on the returned run.
	if got.GetWorkflowId() != run.GetWorkflowId() {
		t.Errorf("§6.A parity (recipeId->workflow_id rename): workflow_id = %q, want %q", got.GetWorkflowId(), run.GetWorkflowId())
	}
}
