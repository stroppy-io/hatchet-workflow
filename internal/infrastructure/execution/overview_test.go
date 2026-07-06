package execution

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

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

func TestOverviewGetFallsBackToPersistedTerminalRecord(t *testing.T) {
	started := timestamppb.New(time.Unix(10, 0))
	finished := timestamppb.New(time.Unix(70, 0))
	tc := &countingRunStateQuerier{err: errors.New("must not query terminal workflow")}
	reader := &OverviewReader{
		tc: tc,
		store: fakeSnapshotStore{
			run: &models.TestRunRecord{
				Entity: &common.Entity{Id: "run-1"},
				Status: common.Status_STATUS_COMPLETED,
				InfrastructureState: &deploymentpb.InfrastructureState{
					Machines: []*deploymentpb.MachineState{
						{NodeId: "node-1", Status: common.Status_STATUS_DEPLOYED},
					},
				},
				DeploymentPlan: &deploymentpb.DeploymentPlan{
					Components: []*deploymentpb.ComponentDeployment{
						{ComponentId: "postgres-master", NodeId: "node-1", Status: common.Status_STATUS_DEPLOYED},
					},
				},
				Summary: &models.TestRunRecord_Summary{
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
			run: &models.TestRunRecord{
				Entity: &common.Entity{Id: "run-1"},
				Status: common.Status_STATUS_RUNNING,
				Summary: &models.TestRunRecord_Summary{
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
			run: &models.TestRunRecord{
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

func TestDeploymentPlanChildrenMatchWorkflowExecutionOrder(t *testing.T) {
	plan := &deploymentpb.DeploymentPlan{
		Components: []*deploymentpb.ComponentDeployment{
			{ComponentId: "node-2-fast", NodeId: "node-2", GlobalPriority: 1, NodePriority: 1},
			{ComponentId: "node-1-slow", NodeId: "node-1", GlobalPriority: 1, NodePriority: 99},
			{ComponentId: "global-0", NodeId: "node-9", GlobalPriority: 0, NodePriority: 99},
		},
	}

	nodes := deploymentPlanChildren("run-1", plan, common.Status_STATUS_RUNNING)
	got := []string{nodes[0].GetComponentId(), nodes[1].GetComponentId(), nodes[2].GetComponentId()}
	want := []string{"global-0", "node-1-slow", "node-2-fast"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("component order = %v, want %v", got, want)
		}
		if nodes[i].GetOrder() != uint32(i+1) {
			t.Fatalf("node[%d] order = %d, want %d", i, nodes[i].GetOrder(), i+1)
		}
	}
}

func TestLegacyRecordFallbackInheritsCompletedDeploymentChildren(t *testing.T) {
	rec := &models.TestRunRecord{
		Entity: &common.Entity{Id: "run-1"},
		Status: common.Status_STATUS_COMPLETED,
		Spec: &domainpb.TestRun{
			InfrastructurePlan: &deploymentpb.InfrastructurePlan{
				Provider: deploymentpb.Provider_PROVIDER_DOCKER,
				Machines: []*deploymentpb.MachinePlan{
					{
						NodeId: "node-1",
						ProviderParams: &deploymentpb.MachinePlan_Docker{Docker: &deploymentpb.Docker_Container{
							Image: "postgres:16",
						}},
						QuotaRequests: []*deploymentpb.Quota_Request{
							{
								Info: &deploymentpb.Quota_Info{
									Provider: deploymentpb.Provider_PROVIDER_DOCKER,
									Name:     "host.cpuCores",
									Units:    "cores",
								},
								Request: 2,
							},
						},
					},
				},
			},
		},
		InfrastructureState: &deploymentpb.InfrastructureState{
			Machines: []*deploymentpb.MachineState{{NodeId: "node-1", Status: common.Status_STATUS_DEPLOYED}},
		},
		DeploymentPlan: &deploymentpb.DeploymentPlan{
			Components: []*deploymentpb.ComponentDeployment{
				{
					ComponentId: "postgres-master",
					NodeId:      "node-1",
					Steps: []*deploymentpb.AgentStep{
						{
							Id:    "010_write_config",
							Order: 10,
							Action: &deploymentpb.AgentStep_WriteFile{WriteFile: &common.File{
								Info: &common.File_Info{Path: "/etc/postgresql/postgresql.conf"},
							}},
						},
					},
				},
			},
		},
	}

	overview := overviewFromRecord("run-1", rec, nil, timestamppb.Now())
	execute := overview.GetPipeline().GetRoots()[2]
	component := execute.GetChildren()[0]
	step := component.GetChildren()[0]
	if got := component.GetStatus(); got != common.Status_STATUS_COMPLETED {
		t.Fatalf("component status = %s, want completed", got)
	}
	if got := step.GetStatus(); got != common.Status_STATUS_COMPLETED {
		t.Fatalf("step status = %s, want completed", got)
	}
	infrastructure := overview.GetPipeline().GetRoots()[0]
	for _, id := range []string{"infrastructure/plan", "infrastructure/state", "quota/request/node-1/host.cpuCores"} {
		if !pipelineNodeHasOutputID(infrastructure, id) {
			t.Fatalf("infrastructure root missing output %q: %+v", id, infrastructure.GetOutputs())
		}
	}
	if got := overview.GetPipeline().GetRoots()[4].GetName(); got != stageTeardownNodeName {
		t.Fatalf("root[4] = %q, want teardown", got)
	}
}

func TestRecordFallbackUsesPersistedRuntimeState(t *testing.T) {
	rec := &models.TestRunRecord{
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
		Summary: &models.TestRunRecord_Summary{ProgressPct: 100},
	}

	overview := overviewFromRecord("run-1", rec, nil, timestamppb.Now())
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

// TestRecordFallbackProjectsRecipeRunDynamicStagesGenerically is Task 5's
// overview acceptance case: a recipe run's TestRunRecord (no Spec/
// InfrastructureState/DeploymentPlan — see recipe.Service.StartRun's doc
// comment) carries a RunState with RunRecipeWorkflow's own dynamic stage
// names (compile/infra/execute — see internal/workflows/runrecipe.go), not
// domainTestWorkflow's 5 hardcoded pipeline names. The record-fallback
// projection must render exactly those stages, not force them into
// stageInfrastructureNodeName/stageRenderDeploymentPlanNodeName/etc.
func TestRecordFallbackProjectsRecipeRunDynamicStagesGenerically(t *testing.T) {
	rec := &models.TestRunRecord{
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

	overview := overviewFromRecord("run-1", rec, nil, timestamppb.Now())
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

func TestTopologyFromRecordIncludesRuntimeControlPlaneMonitoringAndActionEdges(t *testing.T) {
	started := timestamppb.New(time.Unix(100, 0))
	installCollectors := &deploymentpb.AgentStep{
		Id:     "300_install_collectors",
		Order:  300,
		Status: common.Status_STATUS_RUNNING,
		Action: &deploymentpb.AgentStep_CallCmd{CallCmd: &common.Cmd{Spec: &common.Cmd_Spec{
			Command: &common.Cmd_Spec_Script{Script: &common.Cmd_Script{
				Text: "curl https://control.example/api/binaries/node_exporter/1 && install postgres_exporter vmagent vector",
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
	rec := &models.TestRunRecord{
		Entity: &common.Entity{Id: "run-1"},
		Spec: &domainpb.TestRun{
			Id: "run-1",
			TopologySpec: &topologypb.TopologySpec{
				Labels: map[string]string{
					deploymentbuilder.LabelServerAddr: "https://control.example",
					deploymentbuilder.LabelRunID:      "run-1",
				},
				Nodes: []*topologypb.Node{
					{Id: "node-1", ComponentIds: []string{"postgres-master"}},
				},
				Components: []*topologypb.Component{
					{
						Id:     "postgres-master",
						Kind:   topologypb.Component_KIND_DATABASE,
						Engine: "postgres",
						Role:   "master",
					},
				},
			},
		},
		Status: common.Status_STATUS_RUNNING,
		InfrastructureState: &deploymentpb.InfrastructureState{
			Machines: []*deploymentpb.MachineState{
				{
					NodeId: "node-1",
					Status: common.Status_STATUS_DEPLOYED,
					Endpoints: []*deploymentpb.Endpoint{
						{Name: "private", Address: "10.0.0.10"},
					},
				},
			},
		},
		DeploymentPlan: &deploymentpb.DeploymentPlan{
			Components: []*deploymentpb.ComponentDeployment{
				{
					ComponentId: "postgres-master",
					NodeId:      "node-1",
					Status:      common.Status_STATUS_DEPLOYMENT,
					Steps:       []*deploymentpb.AgentStep{installCollectors},
				},
			},
		},
		RuntimeState: &workflowpb.RunState{
			Status: common.Status_STATUS_RUNNING,
			Stages: []*workflowpb.Stage{
				{
					NodeExecutionId:       deploymentbuilder.StepExecutionID("postgres-master", "300_install_collectors"),
					Name:                  "install collectors",
					Status:                common.Status_STATUS_RUNNING,
					StartedAt:             started,
					Attempt:               1,
					Order:                 1,
					ParentNodeExecutionId: deploymentbuilder.ComponentExecutionID("postgres-master"),
					Phase:                 executeDeploymentPlanNodeName,
					ComponentId:           "postgres-master",
					MachineId:             "node-1",
					Operation:             deploymentbuilder.AgentStepOperation(installCollectors),
				},
				{
					NodeExecutionId:       deploymentbuilder.StepExecutionID("postgres-master", "310_write_vector_config"),
					Name:                  "write vector config",
					Status:                common.Status_STATUS_RUNNING,
					StartedAt:             started,
					Attempt:               1,
					Order:                 2,
					ParentNodeExecutionId: deploymentbuilder.ComponentExecutionID("postgres-master"),
					Phase:                 executeDeploymentPlanNodeName,
					ComponentId:           "postgres-master",
					MachineId:             "node-1",
					Operation:             deploymentbuilder.AgentStepOperation(writeVectorConfig),
				},
			},
		},
	}

	topo := topologyFromRecord(rec)
	for _, id := range []string{
		"control-plane",
		"machine/node-1",
		"agent/node-1",
		"component/postgres-master",
		"monitor/node-1/node_exporter",
		"monitor/node-1/postgres_exporter",
		"monitor/node-1/vmagent",
		"monitor/node-1/vector",
	} {
		if node := runtimeNodeByID(topo, id); node == nil {
			t.Fatalf("runtime node %q missing", id)
		}
	}
	if got, want := runtimeNodeByID(topo, "control-plane").GetAddress(), "https://control.example"; got != want {
		t.Fatalf("control-plane address = %q, want %q", got, want)
	}
	assertRuntimeEdge(t, topo, "agent/node-1", "control-plane", "/api/binaries")
	assertRuntimeEdge(t, topo, "monitor/node-1/vmagent", "control-plane", "/insert/0/prometheus/api/v1/write")
	assertRuntimeEdge(t, topo, "monitor/node-1/vector", "control-plane", "/insert/jsonline")
	assertRuntimeEdge(t, topo, "monitor/node-1/vmagent", "monitor/node-1/node_exporter", "node")
	assertRuntimeEdge(t, topo, "monitor/node-1/postgres_exporter", "component/postgres-master", "postgres_local")
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
	logs := assertRuntimeEdge(t, topo, "monitor/node-1/vector", "component/postgres-master", "logs/journald_and_files")
	if got := logs.GetKind(); got != topologypb.Connection_KIND_OBSERVATION {
		t.Fatalf("vector local logs kind = %s, want observation", got)
	}
	if got := logs.GetProtocol(); got == topologypb.Connection_PROTOCOL_CONTROL {
		t.Fatalf("vector local logs protocol = %s, want non-control observation edge", got)
	}
	if got, want := logs.GetLabels()[runtimeLabelRelation], "local_log_collection"; got != want {
		t.Fatalf("vector local logs relation = %q, want %q", got, want)
	}
}

func TestTopologyFromRecordLabelsUnspecifiedLogicalConnections(t *testing.T) {
	rec := &models.TestRunRecord{
		Entity: &common.Entity{Id: "run-1"},
		Spec: &domainpb.TestRun{
			Id: "run-1",
			TopologySpec: &topologypb.TopologySpec{
				Nodes: []*topologypb.Node{
					{Id: "node-1", ComponentIds: []string{"client", "postgres-master"}},
				},
				Components: []*topologypb.Component{
					{Id: "client", Kind: topologypb.Component_KIND_WORKLOAD, Engine: "stroppy", Role: "runner"},
					{Id: "postgres-master", Kind: topologypb.Component_KIND_DATABASE, Engine: "postgres", Role: "master"},
				},
				Connections: []*topologypb.Connection{
					{FromComponentId: "client", ToComponentId: "postgres-master"},
				},
			},
		},
		Status: common.Status_STATUS_PENDING,
	}

	topo := topologyFromRecord(rec)
	edge := assertRuntimeEdge(t, topo, "component/client", "component/postgres-master", "flow/tcp")
	if got := edge.GetKind(); got != topologypb.Connection_KIND_FLOW {
		t.Fatalf("logical edge kind = %s, want flow", got)
	}
	if got := edge.GetProtocol(); got != topologypb.Connection_PROTOCOL_TCP {
		t.Fatalf("logical edge protocol = %s, want tcp", got)
	}
	if got := edge.GetMode(); got != topologypb.Connection_MODE_REQUEST {
		t.Fatalf("logical edge mode = %s, want request", got)
	}
}

func TestTopologyFromRecordReadsServerAddrFromDeploymentPlanLabels(t *testing.T) {
	rec := &models.TestRunRecord{
		Entity: &common.Entity{Id: "run-1"},
		Spec: &domainpb.TestRun{
			Id: "run-1",
			TopologySpec: &topologypb.TopologySpec{
				Nodes: []*topologypb.Node{
					{Id: "node-1", ComponentIds: []string{"postgres-master"}},
				},
				Components: []*topologypb.Component{
					{Id: "postgres-master", Kind: topologypb.Component_KIND_DATABASE, Engine: "postgres", Role: "master"},
				},
			},
		},
		InfrastructureState: &deploymentpb.InfrastructureState{
			Machines: []*deploymentpb.MachineState{{NodeId: "node-1", Status: common.Status_STATUS_DEPLOYED}},
		},
		DeploymentPlan: &deploymentpb.DeploymentPlan{
			Labels: map[string]string{deploymentbuilder.LabelServerAddr: "https://control.example"},
			Components: []*deploymentpb.ComponentDeployment{
				{ComponentId: "postgres-master", NodeId: "node-1", Status: common.Status_STATUS_DEPLOYED},
			},
		},
	}

	topo := topologyFromRecord(rec)
	if got, want := runtimeNodeByID(topo, "control-plane").GetAddress(), "https://control.example"; got != want {
		t.Fatalf("control-plane address = %q, want %q", got, want)
	}
	if node := runtimeNodeByID(topo, "monitor/node-1/vmagent"); node == nil {
		t.Fatal("vmagent runtime node missing when deployment plan carries server addr")
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

func TestOverviewProjectsWorkersFromPersistedTopology(t *testing.T) {
	rec := &models.TestRunRecord{
		Entity: &common.Entity{Id: "run-1"},
		InfrastructureState: &deploymentpb.InfrastructureState{
			Machines: []*deploymentpb.MachineState{
				{
					NodeId: "node-1",
					Status: common.Status_STATUS_DEPLOYED,
					Endpoints: []*deploymentpb.Endpoint{
						{Name: "public", Address: "203.0.113.10"},
						{Name: "private", Address: "10.0.0.10"},
					},
				},
			},
		},
		DeploymentPlan: &deploymentpb.DeploymentPlan{
			Components: []*deploymentpb.ComponentDeployment{
				{
					ComponentId: "postgres-master",
					NodeId:      "node-1",
					Status:      common.Status_STATUS_DEPLOYMENT,
					Steps: []*deploymentpb.AgentStep{
						{
							Id:     "020_start",
							Status: common.Status_STATUS_DEPLOYMENT,
							Action: &deploymentpb.AgentStep_CallCmd{CallCmd: &common.Cmd{}},
							Labels: map[string]string{
								deploymentbuilder.LabelNodeExecutionID: deploymentbuilder.StepExecutionID("postgres-master", "020_start"),
							},
						},
					},
				},
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

	overview := mergeOverviewWithRecord(projectOverview("run-1", &workflowpb.RunState{
		Status: common.Status_STATUS_RUNNING,
		Stages: []*workflowpb.Stage{
			{
				NodeExecutionId: deploymentbuilder.StageExecutionID(executeDeploymentPlanNodeName),
				Name:            executeDeploymentPlanNodeName,
				Status:          common.Status_STATUS_RUNNING,
			},
		},
	}, timestamppb.Now()), rec, presence)

	workers := overview.GetWorkers()
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
		t.Fatalf("worker host = %q, want %q", got, want)
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
	if got, want := worker.GetStatus(), common.Status_STATUS_RUNNING; got != want {
		t.Fatalf("worker status = %s, want %s", got, want)
	}
	if got, want := worker.GetCurrentNodeExecutionId(), deploymentbuilder.StepExecutionID("postgres-master", "020_start"); got != want {
		t.Fatalf("worker current_node_execution_id = %q, want %q", got, want)
	}
}

func TestWorkersFromRecordKeepTerminalPresenceOverRegistrySample(t *testing.T) {
	lastSeen := timestamppb.New(time.Unix(100, 0))
	rec := &models.TestRunRecord{
		Entity: &common.Entity{Id: "run-1"},
		Status: common.Status_STATUS_COMPLETED,
		InfrastructureState: &deploymentpb.InfrastructureState{
			Machines: []*deploymentpb.MachineState{
				{NodeId: "node-1", Status: common.Status_STATUS_DEPLOYED},
			},
		},
		DeploymentPlan: &deploymentpb.DeploymentPlan{
			Components: []*deploymentpb.ComponentDeployment{
				{ComponentId: "postgres-master", NodeId: "node-1", Status: common.Status_STATUS_DEPLOYED},
			},
		},
	}
	workers := workersFromRecord(rec, map[string]*monitor.WorkerInfo{
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
			run: &models.TestRunRecord{
				Entity: &common.Entity{Id: "run-1"},
				Status: common.Status_STATUS_COMPLETED,
				Summary: &models.TestRunRecord_Summary{
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

type blockingRunStateQuerier struct {
	calls int
}

func (f *blockingRunStateQuerier) GetRunState(ctx context.Context, _, _ string) (*workflowpb.RunState, error) {
	f.calls++
	<-ctx.Done()
	return nil, ctx.Err()
}

type fakeSnapshotStore struct {
	run *models.TestRunRecord
}

func (f fakeSnapshotStore) RunRecord(context.Context, string) (*models.TestRunRecord, error) {
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

func pipelineNodeHasOutputID(node *monitor.PipelineNode, id string) bool {
	for _, output := range node.GetOutputs() {
		if output.GetId() == id {
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
