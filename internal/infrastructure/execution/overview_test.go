package execution

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domainpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestOverviewFromRun_BuildsFromSummaryAndRuntimeState is SP-E Task 4's Step
// 1 fixture-driven test: overviewFromRun must read Status/Summary/
// RuntimeState straight off models.Run.
func TestOverviewFromRun_BuildsFromSummaryAndRuntimeState(t *testing.T) {
	run := &models.Run{
		Entity: &common.Entity{Id: "run-1"},
		Status: common.Status_STATUS_RUNNING,
		Summary: &models.Run_Summary{
			DbKind: domainpb.Database_KIND_POSTGRES, ProgressPct: 40,
		},
		RuntimeState: &workflowpb.RunState{},
	}
	got := overviewFromRun("run-1", run, nil, timestamppb.Now())
	require.Equal(t, common.Status_STATUS_RUNNING, got.GetStatus())
	require.EqualValues(t, 40, got.GetProgressPct())
}

// TestTopologyFromRun_UsesTopologyFieldOnly asserts the promoted single
// branch: a run's topology snapshot (run.topology, RunTopology) is the only
// source runTopologyState/runtimeTopologyFromRun read — Run has no
// Spec/InfrastructureState/DeploymentPlan to branch on (Step 0's dead-branch
// removal). The brief's illustrative fixture referenced a
// topology.Topology_STATE_PROVISIONED enum value that does not exist on
// topology.Topology_State (see topology.pb.go) and a Topology.GetNodes()
// accessor Topology does not have (it exposes GetRuntimeNodes() instead);
// this test asserts the actual, compiling equivalents: the exact enum value
// runTopologyState's recipe/topology-snapshot branch returns
// (STATE_INFRASTRUCTURE_DEPLOYED, unchanged from the pre-migration
// topologyState's recipe branch) and the machine/agent runtime nodes
// runtimeTopologyFromRun projects from that one topology node.
func TestTopologyFromRun_UsesTopologyFieldOnly(t *testing.T) {
	run := &models.Run{
		Topology: &models.RunTopology{Provider: "docker", Nodes: []*models.RunTopology_MachineNode{
			{NodeId: "db-0", Group: "db"},
		}},
	}
	got := topologyFromRunWithRunState(run, nil)
	require.Equal(t, topologypb.Topology_STATE_INFRASTRUCTURE_DEPLOYED, got.GetState())
	require.NotNil(t, runtimeNodeByID(got, "control-plane"))
	require.NotNil(t, runtimeNodeByID(got, "machine/db-0"))
	require.NotNil(t, runtimeNodeByID(got, "agent/db-0"))
}

// TestWorkerMachineIDs_SourcesFromRunTopologyNotDeploymentPlan is SP-E Task
// 4's discovered-gap fix: workerMachineIDsFromRun must read
// run.topology.nodes, not the always-nil InfrastructureState/DeploymentPlan
// the pre-migration workerMachineIDs read off TestRunRecord (structurally
// empty for every live recipe run — see runtime_topology.go's
// runtimeTopologyFromRun doc).
func TestWorkerMachineIDs_SourcesFromRunTopologyNotDeploymentPlan(t *testing.T) {
	run := &models.Run{Topology: &models.RunTopology{Nodes: []*models.RunTopology_MachineNode{
		{NodeId: "db-0"}, {NodeId: "runner-0"},
	}}}
	ids := workerMachineIDsFromRun(run)
	require.ElementsMatch(t, []string{"db-0", "runner-0"}, ids, "must read Run.topology.nodes, not the always-nil InfrastructureState/DeploymentPlan")
}

func TestOverviewGetFallsBackToPersistedTerminalRecord(t *testing.T) {
	started := timestamppb.New(time.Unix(10, 0))
	finished := timestamppb.New(time.Unix(70, 0))
	tc := &countingRunStateQuerier{err: errors.New("must not query terminal workflow")}
	reader := &OverviewReader{
		tc: tc,
		store: fakeSnapshotStore{
			run: &models.Run{
				Entity: &common.Entity{Id: "run-1"},
				Status: common.Status_STATUS_COMPLETED,
				Summary: &models.Run_Summary{
					StartedAt:   started,
					FinishedAt:  finished,
					Duration:    durationpb.New(time.Minute),
					ProgressPct: 100,
				},
			},
		},
	}

	snap, err := reader.Get(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("get overview: %v", err)
	}

	if got := snap.GetOverview().GetStatus(); got != common.Status_STATUS_COMPLETED {
		t.Fatalf("overview status = %s, want %s", got, common.Status_STATUS_COMPLETED)
	}
	if got := snap.GetOverview().GetProgressPct(); got != 100 {
		t.Fatalf("overview progress = %d, want 100", got)
	}
	if got := snap.GetOverview().GetStartedAt(); got != started {
		t.Fatalf("overview started_at = %v, want persisted value", got)
	}
	if got := snap.GetRun().GetStatus(); got != common.Status_STATUS_COMPLETED {
		t.Fatalf("run status = %s, want %s", got, common.Status_STATUS_COMPLETED)
	}
	roots := snap.GetOverview().GetPipeline().GetRoots()
	if got, want := len(roots), 5; got != want {
		t.Fatalf("pipeline roots = %d, want %d", got, want)
	}
	for i, want := range []string{
		stageInfrastructureNodeName,
		stageRenderDeploymentPlanNodeName,
		executeDeploymentPlanNodeName,
		stageWorkloadNodeName,
		stageTeardownNodeName,
	} {
		if got := roots[i].GetName(); got != want {
			t.Fatalf("root[%d] name = %q, want %q", i, got, want)
		}
		if got := roots[i].GetOrder(); got != uint32(i+1) {
			t.Fatalf("root[%d] order = %d, want %d", i, got, i+1)
		}
	}
	if got := snap.GetOverview().GetSource(); got != monitor.ObservationSource_OBSERVATION_SOURCE_PERSISTED_RECORD {
		t.Fatalf("overview source = %s, want persisted record", got)
	}
	if tc.calls != 0 {
		t.Fatalf("temporal queries = %d, want 0 for terminal persisted record", tc.calls)
	}
	if got := snap.GetOverview().GetDegradedReasons(); len(got) != 0 {
		t.Fatalf("overview degraded reasons = %v, want none for intentional persisted terminal path", got)
	}
}

func TestOverviewGetBoundsSlowTemporalQuery(t *testing.T) {
	tc := &blockingRunStateQuerier{}
	reader := &OverviewReader{
		tc:                   tc,
		runStateQueryTimeout: 10 * time.Millisecond,
		store: fakeSnapshotStore{
			run: &models.Run{
				Entity: &common.Entity{Id: "run-1"},
				Status: common.Status_STATUS_RUNNING,
				Summary: &models.Run_Summary{
					ProgressPct: 25,
				},
			},
		},
	}

	started := time.Now()
	snap, err := reader.Get(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("get overview: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("get overview elapsed = %s, want bounded temporal fallback", elapsed)
	}
	if got := tc.calls; got != 1 {
		t.Fatalf("temporal queries = %d, want 1", got)
	}
	if got := snap.GetOverview().GetStatus(); got != common.Status_STATUS_RUNNING {
		t.Fatalf("overview status = %s, want persisted running", got)
	}
	if got := snap.GetOverview().GetProgressPct(); got != 25 {
		t.Fatalf("overview progress = %d, want persisted summary", got)
	}
	if got := snap.GetOverview().GetSource(); got != monitor.ObservationSource_OBSERVATION_SOURCE_PERSISTED_RECORD {
		t.Fatalf("overview source = %s, want persisted record fallback", got)
	}
	if got := snap.GetOverview().GetDegradedReasons(); !containsPrefix(got, "workflow_state_unavailable:") {
		t.Fatalf("overview degraded reasons = %v, want workflow timeout reason", got)
	}
}

func TestOverviewGetSkipsTemporalWhenPersistedRuntimeStateIsTerminal(t *testing.T) {
	tc := &countingRunStateQuerier{err: errors.New("must not query terminal runtime state")}
	reader := &OverviewReader{
		tc: tc,
		store: fakeSnapshotStore{
			run: &models.Run{
				Entity: &common.Entity{Id: "run-1"},
				Status: common.Status_STATUS_RUNNING,
				RuntimeState: &workflowpb.RunState{
					Status: common.Status_STATUS_COMPLETED,
					Stages: []*workflowpb.Stage{
						{
							NodeExecutionId: deploymentbuilder.StageExecutionID(stageInfrastructureNodeName),
							Name:            stageInfrastructureNodeName,
							Status:          common.Status_STATUS_COMPLETED,
						},
					},
				},
			},
		},
	}

	snap, err := reader.Get(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("get overview: %v", err)
	}
	if got := tc.calls; got != 0 {
		t.Fatalf("temporal queries = %d, want 0 for terminal persisted runtime_state", got)
	}
	if got := snap.GetOverview().GetStatus(); got != common.Status_STATUS_COMPLETED {
		t.Fatalf("overview status = %s, want runtime_state terminal status", got)
	}
}

// TestOverviewGetQueriesRunRecipeWorkflowIDUnconditionally covers Step 7:
// every models.Run executes under RunRecipeWorkflow (workflow_id is always
// set), so Get no longer branches between runRecipeWorkflowID and
// testWorkflowID — the classic TestWorkflow id and its branch are gone
// (Step 0). This replaces the pre-migration pair of tests that asserted the
// two different workflow ids for "recipe" vs "non-recipe" runs — there is no
// non-recipe run kind left to assert the other branch against.
func TestOverviewGetQueriesRunRecipeWorkflowIDUnconditionally(t *testing.T) {
	tc := &capturingRunStateQuerier{
		state: &workflowpb.RunState{Status: common.Status_STATUS_RUNNING},
	}
	reader := &OverviewReader{
		tc: tc,
		store: fakeSnapshotStore{
			run: &models.Run{
				Entity:     &common.Entity{Id: "run-1"},
				Status:     common.Status_STATUS_RUNNING,
				WorkflowId: "recipe-1",
			},
		},
	}

	if _, err := reader.Get(context.Background(), "run-1"); err != nil {
		t.Fatalf("get overview: %v", err)
	}
	if got, want := tc.calls, 1; got != want {
		t.Fatalf("temporal queries = %d, want %d", got, want)
	}
	if got, want := tc.gotWorkflowID, runRecipeWorkflowID("run-1"); got != want {
		t.Fatalf("queried workflow id = %q, want %q", got, want)
	}
}

func TestOverviewProjectsRunStateStageTree(t *testing.T) {
	stepOperation := deploymentbuilder.AgentStepOperation(&deploymentpb.AgentStep{
		Id:     "010_write_config",
		Order:  10,
		Status: common.Status_STATUS_DEPLOYED,
		Action: &deploymentpb.AgentStep_WriteFile{WriteFile: &common.File{
			Info:    &common.File_Info{Path: "/etc/postgresql/postgresql.conf"},
			Content: &common.File_Text{Text: "shared_buffers = 1GB\n"},
		}},
		Labels: map[string]string{
			deploymentbuilder.LabelNodeExecutionID: deploymentbuilder.StepExecutionID("postgres-master", "010_write_config"),
		},
	})
	overview := projectOverview("run-1", &workflowpb.RunState{
		Status: common.Status_STATUS_RUNNING,
		Stages: []*workflowpb.Stage{
			{
				NodeExecutionId: deploymentbuilder.StageExecutionID(executeDeploymentPlanNodeName),
				Name:            executeDeploymentPlanNodeName,
				Status:          common.Status_STATUS_RUNNING,
				Attempt:         1,
				Order:           1,
				Phase:           executeDeploymentPlanNodeName,
			},
			{
				NodeExecutionId:       deploymentbuilder.ComponentExecutionID("postgres-master"),
				Name:                  "postgres-master",
				Status:                common.Status_STATUS_RUNNING,
				Attempt:               1,
				Order:                 1,
				ParentNodeExecutionId: deploymentbuilder.StageExecutionID(executeDeploymentPlanNodeName),
				Phase:                 executeDeploymentPlanNodeName,
				ComponentId:           "postgres-master",
				MachineId:             "node-1",
			},
			{
				NodeExecutionId:       deploymentbuilder.StepExecutionID("postgres-master", "010_write_config"),
				Name:                  "write_file: /etc/postgresql/postgresql.conf",
				Status:                common.Status_STATUS_COMPLETED,
				Attempt:               1,
				Order:                 1,
				ParentNodeExecutionId: deploymentbuilder.ComponentExecutionID("postgres-master"),
				Phase:                 executeDeploymentPlanNodeName,
				ComponentId:           "postgres-master",
				MachineId:             "node-1",
				Operation:             stepOperation,
			},
		},
	}, timestamppb.Now())

	root := overview.GetPipeline().GetRoots()[0]
	if got := len(root.GetChildren()); got != 1 {
		t.Fatalf("component children = %d, want 1", got)
	}
	component := root.GetChildren()[0]
	if got, want := component.GetNodeExecutionId(), deploymentbuilder.ComponentExecutionID("postgres-master"); got != want {
		t.Fatalf("component node_execution_id = %q, want %q", got, want)
	}
	if got := len(component.GetChildren()); got != 1 {
		t.Fatalf("step children = %d, want 1", got)
	}
	step := component.GetChildren()[0]
	if got, want := step.GetNodeExecutionId(), deploymentbuilder.StepExecutionID("postgres-master", "010_write_config"); got != want {
		t.Fatalf("step node_execution_id = %q, want %q", got, want)
	}
	if got, want := step.GetLogRef().GetRunId(), "run-1"; got != want {
		t.Fatalf("step log_ref run_id = %q, want %q", got, want)
	}
	if got, want := step.GetLogRef().GetNodeExecutionId(), deploymentbuilder.StepExecutionID("postgres-master", "010_write_config"); got != want {
		t.Fatalf("step log_ref node_execution_id = %q, want %q", got, want)
	}
	if got, want := step.GetLogRef().GetComponentId(), "postgres-master"; got != want {
		t.Fatalf("step log_ref component_id = %q, want %q", got, want)
	}
	if got, want := component.GetParentNodeExecutionId(), deploymentbuilder.StageExecutionID(executeDeploymentPlanNodeName); got != want {
		t.Fatalf("component parent_node_execution_id = %q, want %q", got, want)
	}
	if got, want := component.GetMachineId(), "node-1"; got != want {
		t.Fatalf("component machine_id = %q, want %q", got, want)
	}
	if got := component.GetSource(); got != monitor.ObservationSource_OBSERVATION_SOURCE_TEMPORAL_RUN_STATE {
		t.Fatalf("component source = %s, want temporal run state", got)
	}
	if got := step.GetOperation().GetFilePath(); got != "/etc/postgresql/postgresql.conf" {
		t.Fatalf("step operation file_path = %q, want config path", got)
	}
}

func TestRunFallbackUsesPersistedRuntimeState(t *testing.T) {
	run := &models.Run{
		Entity: &common.Entity{Id: "run-1"},
		Status: common.Status_STATUS_COMPLETED,
		RuntimeState: &workflowpb.RunState{
			Status: common.Status_STATUS_COMPLETED,
			Stages: []*workflowpb.Stage{
				{
					NodeExecutionId: deploymentbuilder.StageExecutionID("custom_temporal_stage"),
					Name:            "custom_temporal_stage",
					Status:          common.Status_STATUS_COMPLETED,
					Order:           1,
					Phase:           "custom_temporal_stage",
				},
			},
		},
		Summary: &models.Run_Summary{ProgressPct: 100},
	}

	overview := overviewFromRun("run-1", run, nil, timestamppb.Now())
	roots := overview.GetPipeline().GetRoots()
	if got, want := len(roots), 1; got != want {
		t.Fatalf("roots = %d, want %d from persisted runtime_state", got, want)
	}
	if got := roots[0].GetName(); got != "custom_temporal_stage" {
		t.Fatalf("root name = %q, want persisted runtime stage", got)
	}
	if got := roots[0].GetSource(); got != monitor.ObservationSource_OBSERVATION_SOURCE_PERSISTED_RECORD {
		t.Fatalf("root source = %s, want persisted record", got)
	}
}

// TestRunFallbackProjectsRecipeRunDynamicStagesGenerically is Task 5's
// overview acceptance case: a run's RuntimeState carries RunRecipeWorkflow's
// own dynamic stage names (compile/infra/execute — see
// internal/workflows/runrecipe.go), not domainTestWorkflow's 5 hardcoded
// pipeline names. The record-fallback projection must render exactly those
// stages, not force them into
// stageInfrastructureNodeName/stageRenderDeploymentPlanNodeName/etc.
func TestRunFallbackProjectsRecipeRunDynamicStagesGenerically(t *testing.T) {
	run := &models.Run{
		Entity: &common.Entity{Id: "run-1"},
		Status: common.Status_STATUS_RUNNING,
		RuntimeState: &workflowpb.RunState{
			Status: common.Status_STATUS_RUNNING,
			Stages: []*workflowpb.Stage{
				{
					NodeExecutionId: deploymentbuilder.StageExecutionID("compile"),
					Name:            "compile",
					Status:          common.Status_STATUS_COMPLETED,
					Order:           1,
					Phase:           "compile",
				},
				{
					NodeExecutionId: deploymentbuilder.StageExecutionID("infra"),
					Name:            "infra",
					Status:          common.Status_STATUS_RUNNING,
					Order:           2,
					Phase:           "infra",
				},
				{
					NodeExecutionId: deploymentbuilder.StageExecutionID("execute"),
					Name:            "execute",
					Status:          common.Status_STATUS_PENDING,
					Order:           3,
					Phase:           "execute",
				},
			},
		},
	}

	overview := overviewFromRun("run-1", run, nil, timestamppb.Now())
	roots := overview.GetPipeline().GetRoots()
	if got, want := len(roots), 3; got != want {
		t.Fatalf("roots = %d, want %d (compile/infra/execute, not the 5 fixed test-run stages)", got, want)
	}
	wantNames := []string{"compile", "infra", "execute"}
	for i, want := range wantNames {
		if got := roots[i].GetName(); got != want {
			t.Fatalf("root[%d] name = %q, want %q", i, got, want)
		}
	}
	for _, forbidden := range []string{
		stageInfrastructureNodeName, stageRenderDeploymentPlanNodeName,
		executeDeploymentPlanNodeName, stageWorkloadNodeName, stageTeardownNodeName,
	} {
		for _, root := range roots {
			if root.GetName() == forbidden {
				t.Fatalf("root %q is one of the hardcoded test-run stage names, want generic projection", forbidden)
			}
		}
	}
}

// TestTopologyFromRunProjectsRunTopologySnapshot asserts that a run's
// persisted topology snapshot (models.RunTopology, filled once right after
// ProvisionActivity) yields a topology with the provisioned machine/agent/
// service nodes — the only projection path left after Step 0 deleted the
// classic Spec/InfrastructureState/DeploymentPlan branch (models.Run never
// had those fields to begin with).
func TestTopologyFromRunProjectsRunTopologySnapshot(t *testing.T) {
	run := &models.Run{
		Entity:     &common.Entity{Id: "run-1"},
		Status:     common.Status_STATUS_RUNNING,
		WorkflowId: "recipe-1",
		Topology: &models.RunTopology{
			Provider: "yandex",
			Nodes: []*models.RunTopology_MachineNode{
				{
					NodeId: "db-0",
					Group:  "db",
					Ip:     "10.0.0.1",
					Status: common.Status_STATUS_DEPLOYED,
					Services: []*models.RunTopology_ServiceNode{
						{Name: "patroni-postgres", Image: "spilo:16"},
					},
				},
				{
					NodeId: "db-1",
					Group:  "db",
					Ip:     "10.0.0.2",
					Status: common.Status_STATUS_DEPLOYED,
					Services: []*models.RunTopology_ServiceNode{
						{Name: "patroni-postgres", Image: "spilo:16"},
					},
				},
				{
					NodeId: "runner-0",
					Group:  "runner",
					Ip:     "10.0.0.3",
					Status: common.Status_STATUS_DEPLOYED,
					Services: []*models.RunTopology_ServiceNode{
						{Name: "stroppy", Image: "stroppy:1.2.3"},
					},
				},
			},
		},
	}

	topo := topologyFromRun(run)

	if got, want := topo.GetState(), topologypb.Topology_STATE_INFRASTRUCTURE_DEPLOYED; got != want {
		t.Fatalf("topology state = %s, want %s", got, want)
	}

	nodes := topo.GetRuntimeNodes()
	if got := len(nodes); got <= 1 {
		t.Fatalf("runtime node count = %d, want more than just the control-plane node", got)
	}

	for _, id := range []string{
		"control-plane",
		"machine/db-0",
		"machine/db-1",
		"machine/runner-0",
		"agent/db-0",
		"agent/runner-0",
		"component/patroni-postgres",
		"component/stroppy",
	} {
		if node := runtimeNodeByID(topo, id); node == nil {
			t.Fatalf("runtime node %q missing: %v", id, nodes)
		}
	}

	dbNode := runtimeNodeByID(topo, "machine/db-0")
	if got, want := dbNode.GetRole(), "db"; got != want {
		t.Errorf("machine/db-0 role = %q, want %q", got, want)
	}
	if got, want := dbNode.GetAddress(), "10.0.0.1"; got != want {
		t.Errorf("machine/db-0 address = %q, want %q", got, want)
	}
	if got, want := dbNode.GetStatus(), common.Status_STATUS_DEPLOYED; got != want {
		t.Errorf("machine/db-0 status = %s, want %s", got, want)
	}

	assertRuntimeEdge(t, topo, "machine/db-0", "component/patroni-postgres", "placement")
	assertRuntimeEdge(t, topo, "machine/runner-0", "component/stroppy", "placement")
	assertRuntimeEdge(t, topo, "agent/db-0", "control-plane", "agent_control")
}

// TestRuntimeTopologyProjectsMonitoringFactsFromStageOperations covers the
// monitoring-collector detection addStageAction/operationRuntimeFacts still
// does off RunState stage operations (unrelated to the deleted classic
// Spec-driven addMonitoringRuntime path — see runtime_topology.go's
// runtimeTopologyFromRun doc). This replaces the pre-migration test of the
// same shape that built its fixture from a classic domain.TestRun Spec +
// InfrastructureState + DeploymentPlan, none of which models.Run carries.
func TestRuntimeTopologyProjectsMonitoringFactsFromStageOperations(t *testing.T) {
	started := timestamppb.New(time.Unix(100, 0))
	installCollectors := &deploymentpb.AgentStep{
		Id:     "300_install_collectors",
		Order:  300,
		Status: common.Status_STATUS_RUNNING,
		Action: &deploymentpb.AgentStep_CallCmd{CallCmd: &common.Cmd{Spec: &common.Cmd_Spec{
			Command: &common.Cmd_Spec_Script{Script: &common.Cmd_Script{
				Text: "install node_exporter postgres_exporter vmagent vector",
			}},
		}}},
	}
	writeVectorConfig := &deploymentpb.AgentStep{
		Id:     "310_write_vector_config",
		Order:  310,
		Status: common.Status_STATUS_RUNNING,
		Action: &deploymentpb.AgentStep_WriteFile{WriteFile: &common.File{
			Info:    &common.File_Info{Path: "/etc/vector/vector.yaml"},
			Content: &common.File_Text{Text: "sources: {}\n"},
		}},
	}
	run := &models.Run{
		Entity: &common.Entity{Id: "run-1"},
		Status: common.Status_STATUS_RUNNING,
		Topology: &models.RunTopology{
			Provider: "docker",
			Nodes: []*models.RunTopology_MachineNode{
				{NodeId: "node-1", Group: "db", Status: common.Status_STATUS_DEPLOYED},
			},
		},
	}
	rs := &workflowpb.RunState{
		Status: common.Status_STATUS_RUNNING,
		Stages: []*workflowpb.Stage{
			{
				NodeExecutionId: deploymentbuilder.StepExecutionID("postgres-master", "300_install_collectors"),
				Name:            "install collectors",
				Status:          common.Status_STATUS_RUNNING,
				StartedAt:       started,
				Attempt:         1,
				Order:           1,
				Phase:           executeDeploymentPlanNodeName,
				ComponentId:     "postgres-master",
				MachineId:       "node-1",
				Operation:       deploymentbuilder.AgentStepOperation(installCollectors),
			},
			{
				NodeExecutionId: deploymentbuilder.StepExecutionID("postgres-master", "310_write_vector_config"),
				Name:            "write vector config",
				Status:          common.Status_STATUS_RUNNING,
				StartedAt:       started,
				Attempt:         1,
				Order:           2,
				Phase:           executeDeploymentPlanNodeName,
				ComponentId:     "postgres-master",
				MachineId:       "node-1",
				Operation:       deploymentbuilder.AgentStepOperation(writeVectorConfig),
			},
		},
	}

	topo := topologyFromRunWithRunState(run, rs)
	for _, id := range []string{
		"control-plane",
		"machine/node-1",
		"agent/node-1",
		"monitor/node-1/node_exporter",
		"monitor/node-1/postgres_exporter",
		"monitor/node-1/vmagent",
		"monitor/node-1/vector",
	} {
		if node := runtimeNodeByID(topo, id); node == nil {
			t.Fatalf("runtime node %q missing", id)
		}
	}
	action := assertRuntimeEdge(t, topo, "agent/node-1", "monitor/node-1/vector", "agent_action")
	if got, want := action.GetLabels()[runtimeLabelRelation], "agent_action"; got != want {
		t.Fatalf("vector action relation = %q, want %q", got, want)
	}
	if got := action.GetStartedAt(); got != started {
		t.Fatalf("vector action started_at = %v, want stage timestamp", got)
	}
	if got := countRuntimeEdges(topo, "agent/node-1", "monitor/node-1/vector", "agent_action"); got != 1 {
		t.Fatalf("vector action edges = %d, want collapsed single edge", got)
	}
}

func TestAgentStepOperationProjectsCommandDetailsAndMentions(t *testing.T) {
	step := &deploymentpb.AgentStep{
		Id:    "arbitrary_runtime_step",
		Order: 42,
		Action: &deploymentpb.AgentStep_CallCmd{CallCmd: &common.Cmd{Spec: &common.Cmd_Spec{
			Command: &common.Cmd_Spec_Script{Script: &common.Cmd_Script{
				Text: "set -e\ninstall node_exporter vmagent vector\nsystemctl restart stroppy-vector\n",
			}},
		}}},
		Labels: map[string]string{"k": "v"},
	}

	op := deploymentbuilder.AgentStepOperation(step)
	if got := op.GetKind(); got != monitor.OperationKind_OPERATION_KIND_CALL_CMD {
		t.Fatalf("operation kind = %s, want call cmd", got)
	}
	if got, want := op.GetStepId(), "arbitrary_runtime_step"; got != want {
		t.Fatalf("step id = %q, want %q", got, want)
	}
	if got, want := op.GetCommandText(), "set -e\ninstall node_exporter vmagent vector\nsystemctl restart stroppy-vector\n"; got != want {
		t.Fatalf("command_text = %q, want %q", got, want)
	}
	for _, want := range []string{"node_exporter", "vmagent", "vector", "stroppy-vector"} {
		if !containsString(op.GetMentions(), want) {
			t.Fatalf("mentions = %v, want %q", op.GetMentions(), want)
		}
	}
	if got := deploymentbuilder.AgentStepStageName(step); !strings.Contains(got, "vector") {
		t.Fatalf("agent step name = %q, want generic summary containing extracted mention", got)
	}
}

// TestWorkersFromRunProjectsRunTopologyNodes covers Step 6's redesign:
// workersFromRun builds monitor.WorkerInfo straight off run.topology.nodes
// (node_id/group/ip/status/services), replacing the old
// deploymentpb.MachineState/DeploymentPlan lookup that was structurally
// empty for every live recipe run (Task 4's discovered gap).
func TestWorkersFromRunProjectsRunTopologyNodes(t *testing.T) {
	run := &models.Run{
		Entity: &common.Entity{Id: "run-1"},
		Status: common.Status_STATUS_RUNNING,
		Topology: &models.RunTopology{
			Nodes: []*models.RunTopology_MachineNode{
				{NodeId: "node-1", Group: "db", Ip: "10.0.0.10", Status: common.Status_STATUS_DEPLOYED},
			},
		},
	}

	lastSeen := timestamppb.New(time.Unix(100, 0))
	presence := map[string]*monitor.WorkerInfo{
		"node-1": {
			MachineId:                "node-1",
			Host:                     "agent-node-1",
			Online:                   true,
			Presence:                 monitor.WorkerPresence_WORKER_PRESENCE_ONLINE,
			LastSeenAt:               lastSeen,
			HeartbeatIntervalSeconds: 15,
			AgentVersion:             "test-agent",
			RunId:                    "run-1",
			StatusReason:             "heartbeat_recent",
			Source:                   monitor.ObservationSource_OBSERVATION_SOURCE_AGENT_REGISTRY,
		},
	}

	workers := workersFromRun(run, presence)
	if got, want := len(workers), 1; got != want {
		t.Fatalf("workers = %d, want %d", got, want)
	}
	worker := workers[0]
	if got, want := worker.GetId(), "agent/node-1"; got != want {
		t.Fatalf("worker id = %q, want %q", got, want)
	}
	if got, want := worker.GetMachineId(), "node-1"; got != want {
		t.Fatalf("worker machine_id = %q, want %q", got, want)
	}
	if got, want := worker.GetHost(), "agent-node-1"; got != want {
		t.Fatalf("worker host = %q, want %q (registry sample overrides topology ip)", got, want)
	}
	if !worker.GetOnline() {
		t.Fatal("worker online = false, want true from registry presence")
	}
	if got := worker.GetPresence(); got != monitor.WorkerPresence_WORKER_PRESENCE_ONLINE {
		t.Fatalf("worker presence = %s, want online", got)
	}
	if got := worker.GetSource(); got != monitor.ObservationSource_OBSERVATION_SOURCE_AGENT_REGISTRY {
		t.Fatalf("worker source = %s, want agent registry", got)
	}
	if got := worker.GetLastSeenAt(); got != lastSeen {
		t.Fatalf("worker last_seen_at = %v, want registry timestamp", got)
	}
}

func TestWorkersFromRunKeepTerminalPresenceOverRegistrySample(t *testing.T) {
	lastSeen := timestamppb.New(time.Unix(100, 0))
	run := &models.Run{
		Entity: &common.Entity{Id: "run-1"},
		Status: common.Status_STATUS_COMPLETED,
		Topology: &models.RunTopology{
			Nodes: []*models.RunTopology_MachineNode{
				{NodeId: "node-1", Status: common.Status_STATUS_DEPLOYED},
			},
		},
	}
	workers := workersFromRun(run, map[string]*monitor.WorkerInfo{
		"node-1": {
			MachineId:                "node-1",
			Host:                     "agent-node-1",
			Online:                   true,
			Presence:                 monitor.WorkerPresence_WORKER_PRESENCE_ONLINE,
			LastSeenAt:               lastSeen,
			HeartbeatIntervalSeconds: 15,
			AgentVersion:             "test-agent",
			RunId:                    "run-1",
			StatusReason:             "heartbeat_recent",
			Source:                   monitor.ObservationSource_OBSERVATION_SOURCE_AGENT_REGISTRY,
		},
	})

	if got, want := len(workers), 1; got != want {
		t.Fatalf("workers = %d, want %d", got, want)
	}
	worker := workers[0]
	if worker.GetOnline() {
		t.Fatal("worker online = true, want false for terminal run")
	}
	if got := worker.GetPresence(); got != monitor.WorkerPresence_WORKER_PRESENCE_TERMINATED {
		t.Fatalf("worker presence = %s, want terminated", got)
	}
	if got, want := worker.GetHost(), "agent-node-1"; got != want {
		t.Fatalf("worker host = %q, want registry metadata preserved", got)
	}
	if got := worker.GetLastSeenAt(); got != lastSeen {
		t.Fatalf("worker last_seen_at = %v, want registry timestamp preserved", got)
	}
	if got, want := worker.GetStatusReason(), "run_terminal_agent_terminated"; got != want {
		t.Fatalf("worker status_reason = %q, want %q", got, want)
	}
	if got := worker.GetSource(); got != monitor.ObservationSource_OBSERVATION_SOURCE_PERSISTED_RECORD {
		t.Fatalf("worker source = %s, want persisted record terminal judgement", got)
	}
}

func TestOverviewStreamClosesAfterPersistedTerminalSnapshot(t *testing.T) {
	reader := &OverviewReader{
		tc: fakeRunStateQuerier{err: errors.New("workflow closed")},
		store: fakeSnapshotStore{
			run: &models.Run{
				Entity: &common.Entity{Id: "run-1"},
				Status: common.Status_STATUS_COMPLETED,
				Summary: &models.Run_Summary{
					ProgressPct: 100,
				},
			},
		},
	}

	ch, err := reader.Stream(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("stream overview: %v", err)
	}

	select {
	case snap, ok := <-ch:
		if !ok {
			t.Fatal("stream closed before emitting snapshot")
		}
		if got := snap.GetOverview().GetStatus(); got != common.Status_STATUS_COMPLETED {
			t.Fatalf("overview status = %s, want %s", got, common.Status_STATUS_COMPLETED)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for stream snapshot")
	}

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("stream stayed open after terminal snapshot")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for stream close")
	}
}

type fakeRunStateQuerier struct {
	state *workflowpb.RunState
	err   error
}

func (f fakeRunStateQuerier) GetRunState(context.Context, string, string) (*workflowpb.RunState, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.state, nil
}

type countingRunStateQuerier struct {
	state *workflowpb.RunState
	err   error
	calls int
}

func (f *countingRunStateQuerier) GetRunState(context.Context, string, string) (*workflowpb.RunState, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.state, nil
}

// capturingRunStateQuerier records the workflowID it was queried with, so
// tests can assert which workflow (run-recipe/<id>) the reader addressed.
type capturingRunStateQuerier struct {
	state         *workflowpb.RunState
	err           error
	gotWorkflowID string
	gotRunIDArg   string
	calls         int
}

func (f *capturingRunStateQuerier) GetRunState(_ context.Context, workflowID string, runID string) (*workflowpb.RunState, error) {
	f.calls++
	f.gotWorkflowID = workflowID
	f.gotRunIDArg = runID
	if f.err != nil {
		return nil, f.err
	}
	return f.state, nil
}

type blockingRunStateQuerier struct {
	calls int
}

func (f *blockingRunStateQuerier) GetRunState(ctx context.Context, _, _ string) (*workflowpb.RunState, error) {
	f.calls++
	<-ctx.Done()
	return nil, ctx.Err()
}

type fakeSnapshotStore struct {
	run *models.Run
}

func (f fakeSnapshotStore) RunRecord(context.Context, string) (*models.Run, error) {
	return f.run, nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsPrefix(values []string, prefix string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func runtimeNodeByID(topo *topologypb.Topology, id string) *topologypb.RuntimeNode {
	for _, node := range topo.GetRuntimeNodes() {
		if node.GetId() == id {
			return node
		}
	}
	return nil
}

func assertRuntimeEdge(t *testing.T, topo *topologypb.Topology, from, to, endpoint string) *topologypb.RuntimeConnection {
	t.Helper()
	for _, edge := range topo.GetRuntimeConnections() {
		if edge.GetFromNodeId() == from && edge.GetToNodeId() == to && edge.GetEndpointName() == endpoint {
			return edge
		}
	}
	t.Fatalf("runtime edge %s -> %s endpoint %q missing", from, to, endpoint)
	return nil
}

func countRuntimeEdges(topo *topologypb.Topology, from, to, endpoint string) int {
	var count int
	for _, edge := range topo.GetRuntimeConnections() {
		if edge.GetFromNodeId() == from && edge.GetToNodeId() == to && edge.GetEndpointName() == endpoint {
			count++
		}
	}
	return count
}
