package execution

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	monitorpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	runtimeControlPlaneID = "control-plane"

	runtimeLabelSource       = "source"
	runtimeLabelRelation     = "relation"
	runtimeLabelRuntimeClass = "runtime_class"
)

type runtimeTopologyBuilder struct {
	nodes map[string]*topology.RuntimeNode
	edges map[string]*topology.RuntimeConnection
}

// runtimeTopologyFromRun projects a run's runtime topology. Per SP-E Task
// 4's Step 0 (dead classic-branch removal): the pre-migration version of
// this function branched on rec.GetSpec().GetTopologySpec() (a classic
// domain.TestRun's spec-driven topology, complete with a separate
// InfrastructureState/DeploymentPlan-derived node/edge set) before falling
// back to the recipe-topology-snapshot path below. models.Run never carries
// a Spec/InfrastructureState/DeploymentPlan at all (see Run's own doc
// comment) — that classic branch, and the ~10 helpers it alone called
// (addComponents/addLogicalConnections/addComponentDependencies/
// addMonitoringRuntime/addDatabaseMonitoring and their status/lookup
// helpers), were already unreachable dead code for the live RunRecipeWorkflow
// path (spec §1: domainTestWorkflow is unregistered) and are deleted rather
// than ported forward. The topology-snapshot path (run.topology, filled once
// right after ProvisionActivity) is promoted to the only path; a run with no
// topology snapshot yet renders just the control-plane skeleton plus
// whatever RunState stages have reported a machine_id so far.
func runtimeTopologyFromRun(run *models.Run, rs *workflowpb.RunState) ([]*topology.RuntimeNode, []*topology.RuntimeConnection) {
	if run == nil {
		return nil, nil
	}

	if snap := run.GetTopology(); snap != nil {
		return runtimeTopologyFromRunSnapshot(run, snap, rs)
	}

	b := &runtimeTopologyBuilder{
		nodes: make(map[string]*topology.RuntimeNode),
		edges: make(map[string]*topology.RuntimeConnection),
	}
	b.addControlPlane(run, "")
	for _, machineID := range runtimeMachineIDsFromStages(rs) {
		b.addMachineAndAgent(run, machineID, nil, "", "")
	}
	// rs.GetStages() never carries a machine_id for RunRecipeWorkflow's own
	// root stages (compile/infra/execute/teardown are workflow-level, not
	// per-machine — see runrecipe.go's newRunRecipeWorkflow) until a
	// per-machine stage actually lands; this call is what picks those up
	// once they do.
	b.addStages(rs, nil, nil)
	return b.sortedNodes(), b.sortedEdges()
}

// runtimeMachineIDsFromStages collects the distinct machine ids RunState's
// own stages have reported so far — the only source of machine ids before a
// run's topology snapshot exists (models.Run has no InfrastructureState/
// DeploymentPlan/TopologySpec to read them from otherwise).
func runtimeMachineIDsFromStages(rs *workflowpb.RunState) []string {
	ids := make(map[string]struct{})
	for _, stage := range rs.GetStages() {
		if stage.GetMachineId() != "" {
			ids[stage.GetMachineId()] = struct{}{}
		}
	}
	return sortedSetKeys(ids)
}

// runtimeTopologyFromRunSnapshot projects a run's persisted models.RunTopology
// into topology.RuntimeNode/RuntimeConnection shape: a control-plane node,
// one machine+agent node pair per provisioned machine (mirroring
// addMachineAndAgent's control-plane heartbeat/binary-cache edges), and one
// component node per service placed on that machine's group, connected to
// the machine by a placement edge. Recipe runs carry no server_addr today
// (models.Run has no label carrier equivalent to the classic spec/plan/state
// label sources the pre-Task-4 topologyRuntimeServerAddr scanned — deleted
// alongside the classic branch above), so the binary-cache edge and
// control-plane server_addr label are simply omitted rather than guessed.
func runtimeTopologyFromRunSnapshot(run *models.Run, snap *models.RunTopology, rs *workflowpb.RunState) ([]*topology.RuntimeNode, []*topology.RuntimeConnection) {
	b := &runtimeTopologyBuilder{
		nodes: make(map[string]*topology.RuntimeNode),
		edges: make(map[string]*topology.RuntimeConnection),
	}

	b.addControlPlane(run, "")
	for _, node := range snap.GetNodes() {
		b.addRecipeMachine(node)
	}
	// rs.GetStages() never carries a machine_id for RunRecipeWorkflow's own
	// root stages (compile/infra/execute/teardown are workflow-level, not
	// per-machine — see runrecipe.go's newRunRecipeWorkflow), so addStages
	// is a documented no-op today; kept for forward compatibility if a
	// future per-machine stage ever gets added to RunState here.
	b.addStages(rs, nil, nil)

	return b.sortedNodes(), b.sortedEdges()
}

func (b *runtimeTopologyBuilder) addControlPlane(run *models.Run, serverAddr string) {
	labels := map[string]string{
		runtimeLabelSource:       "control_plane",
		runtimeLabelRuntimeClass: "stroppy_server",
	}
	if serverAddr != "" {
		labels["server_addr"] = serverAddr
	}
	b.addNode(&topology.RuntimeNode{
		Id:           runtimeControlPlaneID,
		Kind:         topology.RuntimeNode_KIND_CONTROL_PLANE,
		Label:        "stroppy server",
		Engine:       "stroppy",
		Role:         "control-plane",
		Status:       controlPlaneRuntimeStatus(run),
		StatusReason: "control_plane_external_to_run",
		Address:      serverAddr,
		Labels:       labels,
	})
}

func (b *runtimeTopologyBuilder) addMachineAndAgent(run *models.Run, machineID string, machine *deploymentpb.MachineState, currentExecutionID string, serverAddr string) {
	if machineID == "" {
		return
	}

	machineStatus := pendingIfUnspecified(machine.GetStatus())
	if machine == nil {
		machineStatus = commonpb.Status_STATUS_PENDING
	}
	machineLabels := mergeRuntimeLabels(machine.GetLabels(), map[string]string{
		runtimeLabelSource:       "infrastructure_state",
		runtimeLabelRuntimeClass: "machine",
	})
	b.addNode(&topology.RuntimeNode{
		Id:           runtimeMachineNodeID(machineID),
		Kind:         topology.RuntimeNode_KIND_MACHINE,
		Label:        machineID,
		Engine:       strings.ToLower(machine.GetProviderResourceId()),
		Role:         "machine",
		MachineId:    machineID,
		Status:       machineStatus,
		StatusReason: machineRuntimeStatusReason(machine),
		Address:      machineHost(machine),
		Labels:       machineLabels,
		Tags:         machine.GetTags(),
	})

	agentStatus := agentRuntimeStatus(run, machine, currentExecutionID)
	agentID := runtimeAgentNodeID(machineID)
	b.addNode(&topology.RuntimeNode{
		Id:              agentID,
		Kind:            topology.RuntimeNode_KIND_AGENT,
		Label:           "agent/" + machineID,
		Engine:          "stroppy",
		Role:            "agent",
		MachineId:       machineID,
		NodeExecutionId: currentExecutionID,
		Status:          agentStatus,
		StatusReason:    agentRuntimeStatusReason(run, machine, currentExecutionID),
		Address:         machineHost(machine),
		Labels: map[string]string{
			runtimeLabelSource:       "agent_registry_or_plan",
			runtimeLabelRuntimeClass: "agent",
		},
	})
	b.addEdge(&topology.RuntimeConnection{
		Id:           runtimeEdgeID("agent", machineID, "control-plane", "heartbeat"),
		FromNodeId:   agentID,
		ToNodeId:     runtimeControlPlaneID,
		Kind:         topology.Connection_KIND_SUPPORT,
		Protocol:     topology.Connection_PROTOCOL_CONTROL,
		Mode:         topology.Connection_MODE_HEARTBEAT,
		EndpointName: "agent_control",
		Status:       agentStatus,
		StatusReason: "agent_presence_and_control_channel",
		Labels: map[string]string{
			runtimeLabelRelation: "agent_control",
		},
	})
	if serverAddr != "" {
		b.addEdge(&topology.RuntimeConnection{
			Id:           runtimeEdgeID("agent", machineID, "control-plane", "binary-cache"),
			FromNodeId:   agentID,
			ToNodeId:     runtimeControlPlaneID,
			Kind:         topology.Connection_KIND_SUPPORT,
			Protocol:     topology.Connection_PROTOCOL_HTTP,
			Mode:         topology.Connection_MODE_REQUEST,
			EndpointName: "/api/binaries",
			Status:       agentStatus,
			StatusReason: "agent_downloads_binaries_through_control_plane",
			Labels: map[string]string{
				runtimeLabelRelation: "binary_cache",
				"server_addr":        serverAddr,
			},
		})
	}
}

// addRecipeMachine adds the machine+agent node pair and service (component)
// nodes for one models.RunTopology_MachineNode entry — a run's snapshot
// already carries each machine's group/service placement directly (no
// separate topology.Component/deployment.ComponentDeployment lookup is
// needed). Status comes straight from the snapshot, not derived from
// run/rs: a run's topology snapshot is a point-in-time fact, not a
// live-updated one.
func (b *runtimeTopologyBuilder) addRecipeMachine(node *models.RunTopology_MachineNode) {
	if node == nil || node.GetNodeId() == "" {
		return
	}
	machineID := node.GetNodeId()
	status := pendingIfUnspecified(node.GetStatus())
	address := node.GetIp()

	machineLabels := mergeRuntimeLabels(node.GetLabels(), map[string]string{
		runtimeLabelSource:       "recipe_topology_snapshot",
		runtimeLabelRuntimeClass: "machine",
		"group":                  node.GetGroup(),
	})
	b.addNode(&topology.RuntimeNode{
		Id:           runtimeMachineNodeID(machineID),
		Kind:         topology.RuntimeNode_KIND_MACHINE,
		Label:        machineID,
		Role:         node.GetGroup(),
		MachineId:    machineID,
		Status:       status,
		StatusReason: "recipe_topology_snapshot",
		Address:      address,
		Labels:       machineLabels,
	})

	agentID := runtimeAgentNodeID(machineID)
	b.addNode(&topology.RuntimeNode{
		Id:           agentID,
		Kind:         topology.RuntimeNode_KIND_AGENT,
		Label:        "agent/" + machineID,
		Engine:       "stroppy",
		Role:         "agent",
		MachineId:    machineID,
		Status:       status,
		StatusReason: "recipe_topology_snapshot",
		Address:      address,
		Labels: map[string]string{
			runtimeLabelSource:       "recipe_topology_snapshot",
			runtimeLabelRuntimeClass: "agent",
		},
	})
	b.addEdge(&topology.RuntimeConnection{
		Id:           runtimeEdgeID("agent", machineID, "control-plane", "heartbeat"),
		FromNodeId:   agentID,
		ToNodeId:     runtimeControlPlaneID,
		Kind:         topology.Connection_KIND_SUPPORT,
		Protocol:     topology.Connection_PROTOCOL_CONTROL,
		Mode:         topology.Connection_MODE_HEARTBEAT,
		EndpointName: "agent_control",
		Status:       status,
		StatusReason: "agent_presence_and_control_channel",
		Labels: map[string]string{
			runtimeLabelRelation: "agent_control",
		},
	})

	for _, svc := range node.GetServices() {
		b.addRecipeService(machineID, status, svc)
	}
}

// addRecipeService adds one service placed on a run's machine as a
// component-kind runtime node, plus its placement edge from the machine.
func (b *runtimeTopologyBuilder) addRecipeService(machineID string, status commonpb.Status, svc *models.RunTopology_ServiceNode) {
	if svc == nil || svc.GetName() == "" {
		return
	}
	serviceID := svc.GetName()
	nodeID := runtimeComponentNodeID(serviceID)
	b.addNode(&topology.RuntimeNode{
		Id:           nodeID,
		Kind:         topology.RuntimeNode_KIND_COMPONENT,
		Label:        serviceID,
		Engine:       svc.GetImage(),
		Role:         "service",
		ComponentId:  serviceID,
		MachineId:    machineID,
		Status:       status,
		StatusReason: "recipe_topology_snapshot",
		Labels: map[string]string{
			runtimeLabelSource:       "recipe_topology_snapshot",
			runtimeLabelRuntimeClass: "service",
		},
	})
	b.addEdge(&topology.RuntimeConnection{
		Id:           runtimeEdgeID("placement", machineID, serviceID),
		FromNodeId:   runtimeMachineNodeID(machineID),
		ToNodeId:     nodeID,
		Kind:         topology.Connection_KIND_SUPPORT,
		Protocol:     topology.Connection_PROTOCOL_CONTROL,
		Mode:         topology.Connection_MODE_REQUEST,
		EndpointName: "placement",
		Status:       status,
		StatusReason: "service_placed_on_machine",
		Labels: map[string]string{
			runtimeLabelRelation: "placement",
		},
	})
}

func (b *runtimeTopologyBuilder) addStages(rs *workflowpb.RunState, components map[string]*topology.Component, machines map[string]*deploymentpb.MachineState) {
	for _, stage := range rs.GetStages() {
		if stage == nil {
			continue
		}
		machineID := stage.GetMachineId()
		componentID := stage.GetComponentId()
		if machineID != "" {
			b.addMachineAndAgent(nil, machineID, machines[machineID], stage.GetNodeExecutionId(), "")
		}
		if componentID != "" && stage.GetOperation() == nil {
			component := components[componentID]
			if component != nil {
				b.addNode(&topology.RuntimeNode{
					Id:              runtimeComponentNodeID(componentID),
					Kind:            runtimeComponentKind(component),
					Label:           componentLabel(component),
					Engine:          component.GetEngine(),
					Role:            component.GetRole(),
					ComponentId:     componentID,
					MachineId:       machineID,
					NodeExecutionId: stage.GetNodeExecutionId(),
					Status:          pendingIfUnspecified(stage.GetStatus()),
					StatusReason:    stageStatusReason(stage),
					StartedAt:       stage.GetStartedAt(),
					FinishedAt:      stage.GetFinishedAt(),
					Labels: map[string]string{
						runtimeLabelSource:       "temporal_run_state",
						runtimeLabelRuntimeClass: "component",
					},
				})
			}
		}
		b.addStageAction(stage)
	}
}

func (b *runtimeTopologyBuilder) addStageAction(stage *workflowpb.Stage) {
	if stage == nil || stage.GetMachineId() == "" || stage.GetOperation() == nil {
		return
	}
	op := stage.GetOperation()
	machineID := stage.GetMachineId()
	agentID := runtimeAgentNodeID(machineID)
	targetID := runtimeMachineNodeID(machineID)
	if stage.GetComponentId() != "" {
		targetID = runtimeComponentNodeID(stage.GetComponentId())
	}
	status := pendingIfUnspecified(stage.GetStatus())
	labels := operationLabels(op)
	labels[runtimeLabelSource] = "temporal_run_state"
	labels[runtimeLabelRelation] = "agent_action"
	b.addEdge(&topology.RuntimeConnection{
		Id:              runtimeEdgeID("agent-action", agentID, targetID),
		FromNodeId:      agentID,
		ToNodeId:        targetID,
		Kind:            topology.Connection_KIND_SUPPORT,
		Protocol:        topology.Connection_PROTOCOL_CONTROL,
		Mode:            topology.Connection_MODE_REQUEST,
		EndpointName:    "agent_action",
		Phase:           stage.GetPhase(),
		NodeExecutionId: stage.GetNodeExecutionId(),
		Status:          status,
		StatusReason:    stageStatusReason(stage),
		StartedAt:       stage.GetStartedAt(),
		FinishedAt:      stage.GetFinishedAt(),
		Labels:          labels,
	})

	facts := operationRuntimeFacts(stage)
	for _, fact := range facts {
		switch fact {
		case "node_exporter":
			b.addNodeExporter(machineID, status, stage)
			b.addAgentTouchesMonitor(machineID, "node_exporter", status, stage, op)
		case "postgres_exporter":
			b.addPostgresExporter(machineID, status, stage)
			b.addAgentTouchesMonitor(machineID, "postgres_exporter", status, stage, op)
		case "mysqld_exporter":
			b.addMysqlExporter(machineID, status, stage)
			b.addAgentTouchesMonitor(machineID, "mysqld_exporter", status, stage, op)
		case "vmagent":
			b.addVmagent(machineID, status, stage)
			b.addAgentTouchesMonitor(machineID, "vmagent", status, stage, op)
			b.addVmagentBaseEdges(machineID, status, stage)
		case "vector":
			b.addVector(machineID, status, stage)
			b.addAgentTouchesMonitor(machineID, "vector", status, stage, op)
			b.addVectorControlPlaneEdge(machineID, status, stage)
		case "binary_cache":
			b.addEdge(&topology.RuntimeConnection{
				Id:              runtimeEdgeID("agent", machineID, "control-plane", "binary-cache"),
				FromNodeId:      runtimeAgentNodeID(machineID),
				ToNodeId:        runtimeControlPlaneID,
				Kind:            topology.Connection_KIND_SUPPORT,
				Protocol:        topology.Connection_PROTOCOL_HTTP,
				Mode:            topology.Connection_MODE_REQUEST,
				EndpointName:    "/api/binaries",
				Phase:           stage.GetPhase(),
				NodeExecutionId: stage.GetNodeExecutionId(),
				Status:          status,
				StatusReason:    stageStatusReason(stage),
				StartedAt:       stage.GetStartedAt(),
				FinishedAt:      stage.GetFinishedAt(),
				Labels: map[string]string{
					runtimeLabelSource:   "temporal_run_state",
					runtimeLabelRelation: "binary_cache",
				},
			})
		}
	}
}

func (b *runtimeTopologyBuilder) addNodeExporter(machineID string, status commonpb.Status, stage *workflowpb.Stage) {
	b.addMonitorNode(machineID, "node_exporter", topology.RuntimeNode_KIND_EXPORTER, "node_exporter", "node_exporter", 9100, status, stage)
}

func (b *runtimeTopologyBuilder) addPostgresExporter(machineID string, status commonpb.Status, stage *workflowpb.Stage) {
	b.addMonitorNode(machineID, "postgres_exporter", topology.RuntimeNode_KIND_EXPORTER, "postgres_exporter", "postgres_exporter", 9187, status, stage)
}

func (b *runtimeTopologyBuilder) addMysqlExporter(machineID string, status commonpb.Status, stage *workflowpb.Stage) {
	b.addMonitorNode(machineID, "mysqld_exporter", topology.RuntimeNode_KIND_EXPORTER, "mysqld_exporter", "mysqld_exporter", 9104, status, stage)
}

func (b *runtimeTopologyBuilder) addVmagent(machineID string, status commonpb.Status, stage *workflowpb.Stage) {
	b.addMonitorNode(machineID, "vmagent", topology.RuntimeNode_KIND_MONITOR, "victoriametrics", "vmagent", 0, status, stage)
}

func (b *runtimeTopologyBuilder) addVector(machineID string, status commonpb.Status, stage *workflowpb.Stage) {
	b.addMonitorNode(machineID, "vector", topology.RuntimeNode_KIND_MONITOR, "vector", "vector", 0, status, stage)
}

func (b *runtimeTopologyBuilder) addMonitorNode(machineID, role string, kind topology.RuntimeNode_Kind, engine string, label string, port uint32, status commonpb.Status, stage *workflowpb.Stage) {
	node := &topology.RuntimeNode{
		Id:           runtimeMonitorNodeID(machineID, role),
		Kind:         kind,
		Label:        label,
		Engine:       engine,
		Role:         role,
		MachineId:    machineID,
		Status:       pendingIfUnspecified(status),
		StatusReason: "monitoring_runtime_from_deployment_plan",
		Labels: map[string]string{
			runtimeLabelSource:       "deployment_plan",
			runtimeLabelRuntimeClass: "monitoring",
		},
	}
	if port > 0 {
		node.Port = runtimePort(port)
	}
	if stage != nil {
		node.NodeExecutionId = stage.GetNodeExecutionId()
		node.StatusReason = stageStatusReason(stage)
		node.StartedAt = stage.GetStartedAt()
		node.FinishedAt = stage.GetFinishedAt()
		node.Labels[runtimeLabelSource] = "temporal_run_state"
	}
	b.addNode(node)
}

func (b *runtimeTopologyBuilder) addVmagentBaseEdges(machineID string, status commonpb.Status, stage *workflowpb.Stage) {
	b.addEdge(&topology.RuntimeConnection{
		Id:              runtimeEdgeID("monitor", machineID, "vmagent", "node_exporter"),
		FromNodeId:      runtimeMonitorNodeID(machineID, "vmagent"),
		ToNodeId:        runtimeMonitorNodeID(machineID, "node_exporter"),
		Kind:            topology.Connection_KIND_OBSERVATION,
		Protocol:        topology.Connection_PROTOCOL_PROMETHEUS_PULL,
		Mode:            topology.Connection_MODE_REQUEST,
		EndpointName:    "node",
		Port:            runtimePort(9100),
		Phase:           executeDeploymentPlanNodeName,
		NodeExecutionId: stageNodeExecutionID(stage),
		Status:          pendingIfUnspecified(status),
		StatusReason:    runtimeStageStatusReason(stage, "vmagent_scrapes_node_exporter"),
		StartedAt:       stageStartedAt(stage),
		FinishedAt:      stageFinishedAt(stage),
		Labels: map[string]string{
			runtimeLabelRelation: "metrics_scrape",
		},
	})
	b.addEdge(&topology.RuntimeConnection{
		Id:              runtimeEdgeID("monitor", machineID, "vmagent", "control-plane", "remote-write"),
		FromNodeId:      runtimeMonitorNodeID(machineID, "vmagent"),
		ToNodeId:        runtimeControlPlaneID,
		Kind:            topology.Connection_KIND_OBSERVATION,
		Protocol:        topology.Connection_PROTOCOL_PROMETHEUS_REMOTE_WRITE,
		Mode:            topology.Connection_MODE_STREAM,
		EndpointName:    "/insert/0/prometheus/api/v1/write",
		Phase:           executeDeploymentPlanNodeName,
		NodeExecutionId: stageNodeExecutionID(stage),
		Status:          pendingIfUnspecified(status),
		StatusReason:    runtimeStageStatusReason(stage, "vmagent_remote_writes_metrics_to_control_plane"),
		StartedAt:       stageStartedAt(stage),
		FinishedAt:      stageFinishedAt(stage),
		Labels: map[string]string{
			runtimeLabelRelation: "metrics_remote_write",
		},
	})
}

func (b *runtimeTopologyBuilder) addVectorControlPlaneEdge(machineID string, status commonpb.Status, stage *workflowpb.Stage) {
	b.addEdge(&topology.RuntimeConnection{
		Id:              runtimeEdgeID("monitor", machineID, "vector", "control-plane", "logs"),
		FromNodeId:      runtimeMonitorNodeID(machineID, "vector"),
		ToNodeId:        runtimeControlPlaneID,
		Kind:            topology.Connection_KIND_OBSERVATION,
		Protocol:        topology.Connection_PROTOCOL_HTTP,
		Mode:            topology.Connection_MODE_STREAM,
		EndpointName:    "/insert/jsonline",
		Phase:           executeDeploymentPlanNodeName,
		NodeExecutionId: stageNodeExecutionID(stage),
		Status:          pendingIfUnspecified(status),
		StatusReason:    runtimeStageStatusReason(stage, "vector_ships_logs_to_control_plane"),
		StartedAt:       stageStartedAt(stage),
		FinishedAt:      stageFinishedAt(stage),
		Labels: map[string]string{
			runtimeLabelRelation: "logs_stream",
		},
	})
}

func (b *runtimeTopologyBuilder) addAgentTouchesMonitor(machineID, role string, status commonpb.Status, stage *workflowpb.Stage, op *monitorpb.PipelineOperation) {
	labels := operationLabels(op)
	labels[runtimeLabelSource] = "temporal_run_state"
	labels[runtimeLabelRelation] = "agent_action"
	b.addEdge(&topology.RuntimeConnection{
		Id:              runtimeEdgeID("agent-monitor-action", machineID, role),
		FromNodeId:      runtimeAgentNodeID(machineID),
		ToNodeId:        runtimeMonitorNodeID(machineID, role),
		Kind:            topology.Connection_KIND_SUPPORT,
		Protocol:        topology.Connection_PROTOCOL_CONTROL,
		Mode:            topology.Connection_MODE_REQUEST,
		EndpointName:    "agent_action",
		Phase:           stage.GetPhase(),
		NodeExecutionId: stage.GetNodeExecutionId(),
		Status:          pendingIfUnspecified(status),
		StatusReason:    stageStatusReason(stage),
		StartedAt:       stage.GetStartedAt(),
		FinishedAt:      stage.GetFinishedAt(),
		Labels:          labels,
	})
}

func (b *runtimeTopologyBuilder) addNode(node *topology.RuntimeNode) {
	if node == nil || node.GetId() == "" {
		return
	}
	if node.Labels == nil {
		node.Labels = map[string]string{}
	}
	node.Status = pendingIfUnspecified(node.GetStatus())
	if existing := b.nodes[node.GetId()]; existing != nil {
		mergeRuntimeNode(existing, node)
		return
	}
	b.nodes[node.GetId()] = node
}

func (b *runtimeTopologyBuilder) addEdge(edge *topology.RuntimeConnection) {
	if edge == nil || edge.GetId() == "" || edge.GetFromNodeId() == "" || edge.GetToNodeId() == "" {
		return
	}
	if edge.Labels == nil {
		edge.Labels = map[string]string{}
	}
	edge.Status = pendingIfUnspecified(edge.GetStatus())
	if existing := b.edges[edge.GetId()]; existing != nil {
		mergeRuntimeEdge(existing, edge)
		return
	}
	b.edges[edge.GetId()] = edge
}

func (b *runtimeTopologyBuilder) sortedNodes() []*topology.RuntimeNode {
	nodes := make([]*topology.RuntimeNode, 0, len(b.nodes))
	for _, node := range b.nodes {
		nodes = append(nodes, node)
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		left := nodes[i]
		right := nodes[j]
		if left.GetKind() != right.GetKind() {
			return left.GetKind() < right.GetKind()
		}
		if left.GetMachineId() != right.GetMachineId() {
			return left.GetMachineId() < right.GetMachineId()
		}
		return left.GetId() < right.GetId()
	})
	return nodes
}

func (b *runtimeTopologyBuilder) sortedEdges() []*topology.RuntimeConnection {
	edges := make([]*topology.RuntimeConnection, 0, len(b.edges))
	for _, edge := range b.edges {
		edges = append(edges, edge)
	}
	sort.SliceStable(edges, func(i, j int) bool {
		left := edges[i]
		right := edges[j]
		if left.GetFromNodeId() != right.GetFromNodeId() {
			return left.GetFromNodeId() < right.GetFromNodeId()
		}
		if left.GetToNodeId() != right.GetToNodeId() {
			return left.GetToNodeId() < right.GetToNodeId()
		}
		return left.GetId() < right.GetId()
	})
	return edges
}

func mergeRuntimeNode(dst, src *topology.RuntimeNode) {
	if src.GetKind() != topology.RuntimeNode_KIND_UNSPECIFIED {
		dst.Kind = src.GetKind()
	}
	if src.GetLabel() != "" {
		dst.Label = src.GetLabel()
	}
	if src.GetEngine() != "" {
		dst.Engine = src.GetEngine()
	}
	if src.GetRole() != "" {
		dst.Role = src.GetRole()
	}
	if src.GetComponentId() != "" {
		dst.ComponentId = src.GetComponentId()
	}
	if src.GetMachineId() != "" {
		dst.MachineId = src.GetMachineId()
	}
	if src.GetNodeExecutionId() != "" {
		dst.NodeExecutionId = src.GetNodeExecutionId()
	}
	dst.Status = preferRuntimeStatus(dst.GetStatus(), src.GetStatus())
	if src.GetStatusReason() != "" {
		dst.StatusReason = src.GetStatusReason()
	}
	if src.GetAddress() != "" {
		dst.Address = src.GetAddress()
	}
	if src.Port != nil {
		dst.Port = runtimePort(src.GetPort())
	}
	if src.GetStartedAt() != nil {
		dst.StartedAt = src.GetStartedAt()
	}
	if src.GetFinishedAt() != nil {
		dst.FinishedAt = src.GetFinishedAt()
	}
	if dst.Labels == nil {
		dst.Labels = map[string]string{}
	}
	for key, value := range src.GetLabels() {
		if value != "" {
			dst.Labels[key] = value
		}
	}
	if src.GetTags() != nil {
		dst.Tags = src.GetTags()
	}
}

func mergeRuntimeEdge(dst, src *topology.RuntimeConnection) {
	if src.GetKind() != topology.Connection_KIND_UNSPECIFIED {
		dst.Kind = src.GetKind()
	}
	if src.GetProtocol() != topology.Connection_PROTOCOL_UNSPECIFIED {
		dst.Protocol = src.GetProtocol()
	}
	if src.GetMode() != topology.Connection_MODE_UNSPECIFIED {
		dst.Mode = src.GetMode()
	}
	if src.GetEndpointName() != "" {
		dst.EndpointName = src.GetEndpointName()
	}
	if src.Port != nil {
		dst.Port = runtimePort(src.GetPort())
	}
	if src.GetPhase() != "" {
		dst.Phase = src.GetPhase()
	}
	if src.GetNodeExecutionId() != "" {
		dst.NodeExecutionId = src.GetNodeExecutionId()
	}
	dst.Status = preferRuntimeStatus(dst.GetStatus(), src.GetStatus())
	if src.GetStatusReason() != "" {
		dst.StatusReason = src.GetStatusReason()
	}
	if src.GetStartedAt() != nil {
		dst.StartedAt = src.GetStartedAt()
	}
	if src.GetFinishedAt() != nil {
		dst.FinishedAt = src.GetFinishedAt()
	}
	if dst.Labels == nil {
		dst.Labels = map[string]string{}
	}
	for key, value := range src.GetLabels() {
		if value != "" {
			dst.Labels[key] = value
		}
	}
	if src.GetTags() != nil {
		dst.Tags = src.GetTags()
	}
}

func preferRuntimeStatus(oldStatus, newStatus commonpb.Status) commonpb.Status {
	oldStatus = pendingIfUnspecified(oldStatus)
	newStatus = pendingIfUnspecified(newStatus)
	if runtimeStatusRank(newStatus) >= runtimeStatusRank(oldStatus) {
		return newStatus
	}
	return oldStatus
}

func runtimeStatusRank(status commonpb.Status) int {
	switch pendingIfUnspecified(status) {
	case commonpb.Status_STATUS_FAILED, commonpb.Status_STATUS_CANCELLED:
		return 100
	case commonpb.Status_STATUS_CANCELLING, commonpb.Status_STATUS_RETRY_WAIT:
		return 90
	case commonpb.Status_STATUS_RUNNING, commonpb.Status_STATUS_DEPLOYMENT, commonpb.Status_STATUS_ALLOCATED:
		return 80
	case commonpb.Status_STATUS_COMPLETED, commonpb.Status_STATUS_DEPLOYED:
		return 70
	case commonpb.Status_STATUS_SKIPPED:
		return 60
	case commonpb.Status_STATUS_PENDING:
		return 10
	default:
		return 0
	}
}

func controlPlaneRuntimeStatus(run *models.Run) commonpb.Status {
	if run == nil {
		return commonpb.Status_STATUS_RUNNING
	}
	switch run.GetStatus() {
	case commonpb.Status_STATUS_COMPLETED:
		return commonpb.Status_STATUS_COMPLETED
	case commonpb.Status_STATUS_FAILED, commonpb.Status_STATUS_CANCELLED, commonpb.Status_STATUS_CANCELLING:
		return run.GetStatus()
	default:
		return commonpb.Status_STATUS_RUNNING
	}
}

func machineRuntimeStatusReason(machine *deploymentpb.MachineState) string {
	if machine == nil {
		return "machine_not_materialized"
	}
	status := pendingIfUnspecified(machine.GetStatus())
	if status == commonpb.Status_STATUS_PENDING {
		return "machine_pending_infrastructure_state"
	}
	return "machine_" + strings.ToLower(status.String())
}

func agentRuntimeStatus(run *models.Run, machine *deploymentpb.MachineState, currentExecutionID string) commonpb.Status {
	if currentExecutionID != "" {
		return commonpb.Status_STATUS_RUNNING
	}
	if run != nil {
		switch run.GetStatus() {
		case commonpb.Status_STATUS_COMPLETED:
			return commonpb.Status_STATUS_COMPLETED
		case commonpb.Status_STATUS_FAILED, commonpb.Status_STATUS_CANCELLED, commonpb.Status_STATUS_CANCELLING:
			return run.GetStatus()
		}
	}
	return workerStatus(machine, "")
}

func agentRuntimeStatusReason(run *models.Run, machine *deploymentpb.MachineState, currentExecutionID string) string {
	if currentExecutionID != "" {
		return "agent_executing_stage"
	}
	if run != nil && isTerminalStatus(run.GetStatus()) {
		return "run_terminal_agent_not_marked_offline_without_registry_sample"
	}
	if machine == nil {
		return "agent_waiting_for_machine"
	}
	return "agent_materialized_with_machine"
}

func runtimeComponentKind(component *topology.Component) topology.RuntimeNode_Kind {
	switch component.GetKind() {
	case topology.Component_KIND_AGENT:
		return topology.RuntimeNode_KIND_AGENT
	case topology.Component_KIND_MONITOR:
		return topology.RuntimeNode_KIND_MONITOR
	case topology.Component_KIND_WORKLOAD:
		return topology.RuntimeNode_KIND_WORKLOAD
	case topology.Component_KIND_EXTERNAL:
		return topology.RuntimeNode_KIND_EXTERNAL
	default:
		return topology.RuntimeNode_KIND_COMPONENT
	}
}

func componentLabel(component *topology.Component) string {
	if component.GetId() == "" {
		return "component"
	}
	role := component.GetRole()
	if role == "" {
		return component.GetId()
	}
	return component.GetId() + " (" + role + ")"
}

func operationRuntimeFacts(stage *workflowpb.Stage) []string {
	op := stage.GetOperation()
	if op == nil {
		return nil
	}
	haystack := strings.ToLower(strings.Join(append([]string{
		stage.GetName(),
		op.GetTarget(),
		op.GetSummary(),
		op.GetCommandText(),
		op.GetFilePath(),
	}, op.GetMentions()...), "\n"))
	facts := make(map[string]struct{})
	if strings.Contains(haystack, "node_exporter") || strings.Contains(haystack, "stroppy-node-exporter") {
		facts["node_exporter"] = struct{}{}
	}
	if strings.Contains(haystack, "postgres_exporter") || strings.Contains(haystack, "stroppy-postgres-exporter") {
		facts["postgres_exporter"] = struct{}{}
	}
	if strings.Contains(haystack, "mysqld_exporter") || strings.Contains(haystack, "stroppy-mysqld-exporter") {
		facts["mysqld_exporter"] = struct{}{}
	}
	if strings.Contains(haystack, "vmagent") || strings.Contains(haystack, "/etc/vmagent") || strings.Contains(haystack, "prometheus/api/v1/write") {
		facts["vmagent"] = struct{}{}
	}
	if strings.Contains(haystack, "vector") || strings.Contains(haystack, "/etc/vector") || strings.Contains(haystack, "/insert/jsonline") {
		facts["vector"] = struct{}{}
	}
	if strings.Contains(haystack, "/api/binaries/") {
		facts["binary_cache"] = struct{}{}
	}
	return sortedSetKeys(facts)
}

func operationLabels(op *monitorpb.PipelineOperation) map[string]string {
	labels := map[string]string{
		"operation_kind": op.GetKind().String(),
	}
	if op.GetStepId() != "" {
		labels["step_id"] = op.GetStepId()
	}
	if op.GetTarget() != "" {
		labels["target"] = op.GetTarget()
	}
	if op.GetSummary() != "" {
		labels["summary"] = op.GetSummary()
	}
	if op.GetFilePath() != "" {
		labels["file_path"] = op.GetFilePath()
	}
	if len(op.GetMentions()) > 0 {
		labels["mentions"] = strings.Join(op.GetMentions(), ",")
	}
	return labels
}

func stageStatusReason(stage *workflowpb.Stage) string {
	if stage.GetStatusReason() != "" {
		return stage.GetStatusReason()
	}
	status := pendingIfUnspecified(stage.GetStatus())
	if status == commonpb.Status_STATUS_PENDING {
		return "temporal_stage_pending"
	}
	return "temporal_stage_" + strings.ToLower(status.String())
}

func runtimeStageStatusReason(stage *workflowpb.Stage, fallback string) string {
	if stage == nil {
		return fallback
	}
	return stageStatusReason(stage)
}

func stageNodeExecutionID(stage *workflowpb.Stage) string {
	if stage == nil {
		return ""
	}
	return stage.GetNodeExecutionId()
}

func stageStartedAt(stage *workflowpb.Stage) *timestamppb.Timestamp {
	if stage == nil {
		return nil
	}
	return stage.GetStartedAt()
}

func stageFinishedAt(stage *workflowpb.Stage) *timestamppb.Timestamp {
	if stage == nil {
		return nil
	}
	return stage.GetFinishedAt()
}

func runtimeMachineNodeID(machineID string) string {
	return "machine/" + machineID
}

func runtimeAgentNodeID(machineID string) string {
	return "agent/" + machineID
}

func runtimeComponentNodeID(componentID string) string {
	return "component/" + componentID
}

func runtimeMonitorNodeID(machineID, role string) string {
	return "monitor/" + machineID + "/" + role
}

func runtimeEdgeID(parts ...string) string {
	id := strings.Join(parts, "/")
	if len(id) <= 240 {
		return id
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return id[:200] + "/" + fmt.Sprintf("%08x", h.Sum32())
}

func runtimePort(port uint32) *uint32 {
	return &port
}

func mergeRuntimeLabels(maps ...map[string]string) map[string]string {
	out := make(map[string]string)
	for _, labels := range maps {
		for key, value := range labels {
			if key != "" && value != "" {
				out[key] = value
			}
		}
	}
	return out
}

func sortedSetKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
