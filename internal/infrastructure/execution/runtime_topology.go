package execution

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
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

func runtimeTopologyFromRecord(rec *models.TestRunRecord, rs *workflowpb.RunState) ([]*topology.RuntimeNode, []*topology.RuntimeConnection) {
	if rec == nil {
		return nil, nil
	}

	// A recipe run has no domain.TestRun spec at all (rec.GetSpec() is nil),
	// so it never has a TopologySpec to project from — instead it persists
	// a models.RecipeTopologySnapshot (see that field's doc on
	// TestRunRecord) right after ProvisionActivity succeeds. Project from
	// that snapshot instead of the classic spec/infrastructure-state/
	// deployment-plan trio below.
	if rec.GetSpec().GetTopologySpec() == nil {
		if snap := rec.GetRecipeTopology(); snap != nil {
			return runtimeTopologyFromRecipeSnapshot(rec, snap, rs)
		}
	}

	b := &runtimeTopologyBuilder{
		nodes: make(map[string]*topology.RuntimeNode),
		edges: make(map[string]*topology.RuntimeConnection),
	}

	spec := rec.GetSpec().GetTopologySpec()
	serverAddr := topologyRuntimeServerAddr(rec)
	machines := machineStatesByID(rec.GetInfrastructureState())
	components := topologyComponentsByID(spec)
	componentPlans := componentDeploymentsByID(rec.GetDeploymentPlan())
	componentMachines := componentMachineIDs(spec, rec.GetDeploymentPlan())
	machineIDs := runtimeMachineIDs(spec, rec.GetInfrastructureState(), rec.GetDeploymentPlan(), rs)
	componentIDsByMachine := runtimeComponentIDsByMachine(componentMachines)
	currentByMachine := currentExecutionByNode(rec.GetDeploymentPlan())

	b.addControlPlane(rec, serverAddr)
	for _, machineID := range machineIDs {
		b.addMachineAndAgent(rec, machineID, machines[machineID], currentByMachine[machineID], serverAddr)
	}
	b.addComponents(rec, components, componentPlans, componentMachines)
	b.addLogicalConnections(rec, spec, components, componentPlans)
	b.addComponentDependencies(rec.GetDeploymentPlan())
	b.addMonitoringRuntime(rec, serverAddr, machines, components, componentIDsByMachine)
	b.addStages(rs, components, machines)

	return b.sortedNodes(), b.sortedEdges()
}

// runtimeTopologyFromRecipeSnapshot projects a recipe run's persisted
// models.RecipeTopologySnapshot into the same topology.RuntimeNode/
// RuntimeConnection shape runtimeTopologyFromRecord builds for a classic run:
// a control-plane node, one machine+agent node pair per provisioned machine
// (mirroring addMachineAndAgent's control-plane heartbeat/binary-cache
// edges), and one component node per service placed on that machine's group,
// connected to the machine by a placement edge (mirroring addComponents'
// placement edge for a classic run's topology.Component). Recipe runs carry
// no server_addr label source today (see topologyRuntimeServerAddr's label
// sources, none of which a recipe run ever fills), so the binary-cache edge
// and control-plane server_addr label are simply omitted rather than guessed.
func runtimeTopologyFromRecipeSnapshot(rec *models.TestRunRecord, snap *models.RecipeTopologySnapshot, rs *workflowpb.RunState) ([]*topology.RuntimeNode, []*topology.RuntimeConnection) {
	b := &runtimeTopologyBuilder{
		nodes: make(map[string]*topology.RuntimeNode),
		edges: make(map[string]*topology.RuntimeConnection),
	}

	b.addControlPlane(rec, "")
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

func topologyRuntimeServerAddr(rec *models.TestRunRecord) string {
	for _, labels := range []map[string]string{
		rec.GetSpec().GetTopologySpec().GetLabels(),
		rec.GetDeploymentPlan().GetLabels(),
		rec.GetSpec().GetInfrastructurePlan().GetLabels(),
		rec.GetInfrastructureState().GetLabels(),
	} {
		if addr := strings.TrimRight(labels[deploymentbuilder.LabelServerAddr], "/"); addr != "" {
			return addr
		}
	}
	return ""
}

func (b *runtimeTopologyBuilder) addControlPlane(rec *models.TestRunRecord, serverAddr string) {
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
		Status:       controlPlaneRuntimeStatus(rec),
		StatusReason: "control_plane_external_to_run",
		Address:      serverAddr,
		Labels:       labels,
	})
}

func (b *runtimeTopologyBuilder) addMachineAndAgent(rec *models.TestRunRecord, machineID string, machine *deploymentpb.MachineState, currentExecutionID string, serverAddr string) {
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

	agentStatus := agentRuntimeStatus(rec, machine, currentExecutionID)
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
		StatusReason:    agentRuntimeStatusReason(rec, machine, currentExecutionID),
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
// nodes for one models.RecipeTopologySnapshot_MachineNode entry — the
// recipe-run counterpart to addMachineAndAgent+addComponents combined, since
// a recipe run's snapshot already carries each machine's group/service
// placement directly (no separate topology.Component/deployment.
// ComponentDeployment lookup is needed). Status comes straight from the
// snapshot (the deployment.MachineState.status ProvisionActivity captured —
// see that field's doc), not derived from rec/rs: a recipe run's snapshot is
// a point-in-time fact, not a live-updated one.
func (b *runtimeTopologyBuilder) addRecipeMachine(node *models.RecipeTopologySnapshot_MachineNode) {
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

// addRecipeService adds one service placed on a recipe-run machine as a
// component-kind runtime node, plus its placement edge from the machine —
// mirrors addComponents' machine->component placement edge for a classic
// run's topology.Component.
func (b *runtimeTopologyBuilder) addRecipeService(machineID string, status commonpb.Status, svc *models.RecipeTopologySnapshot_ServiceNode) {
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

func (b *runtimeTopologyBuilder) addComponents(
	rec *models.TestRunRecord,
	components map[string]*topology.Component,
	componentPlans map[string]*deploymentpb.ComponentDeployment,
	componentMachines map[string]string,
) {
	ids := sortedMapKeys(components)
	for _, componentID := range ids {
		component := components[componentID]
		deployment := componentPlans[componentID]
		machineID := componentMachines[componentID]
		status := componentRuntimeStatus(rec, component, deployment)
		nodeExecutionID := ""
		if componentID != "" {
			nodeExecutionID = deploymentbuilder.ComponentExecutionID(componentID)
		}
		labels := mergeRuntimeLabels(component.GetLabels(), map[string]string{
			runtimeLabelSource:       "topology_spec",
			runtimeLabelRuntimeClass: "component",
			"spec_kind":              component.GetKind().String(),
		})
		b.addNode(&topology.RuntimeNode{
			Id:              runtimeComponentNodeID(componentID),
			Kind:            runtimeComponentKind(component),
			Label:           componentLabel(component),
			Engine:          component.GetEngine(),
			Role:            component.GetRole(),
			ComponentId:     componentID,
			MachineId:       machineID,
			NodeExecutionId: nodeExecutionID,
			Status:          status,
			StatusReason:    componentRuntimeStatusReason(status),
			Address:         "",
			Labels:          labels,
			Tags:            component.GetTags(),
		})

		if machineID != "" {
			b.addEdge(&topology.RuntimeConnection{
				Id:           runtimeEdgeID("placement", machineID, componentID),
				FromNodeId:   runtimeMachineNodeID(machineID),
				ToNodeId:     runtimeComponentNodeID(componentID),
				Kind:         topology.Connection_KIND_SUPPORT,
				Protocol:     topology.Connection_PROTOCOL_CONTROL,
				Mode:         topology.Connection_MODE_REQUEST,
				EndpointName: "placement",
				Status:       status,
				StatusReason: "component_placed_on_machine",
				Labels: map[string]string{
					runtimeLabelRelation: "placement",
				},
			})
			b.addEdge(&topology.RuntimeConnection{
				Id:              runtimeEdgeID("agent", machineID, "execute", componentID),
				FromNodeId:      runtimeAgentNodeID(machineID),
				ToNodeId:        runtimeComponentNodeID(componentID),
				Kind:            topology.Connection_KIND_SUPPORT,
				Protocol:        topology.Connection_PROTOCOL_CONTROL,
				Mode:            topology.Connection_MODE_REQUEST,
				EndpointName:    "agent_execution",
				Phase:           executeDeploymentPlanNodeName,
				NodeExecutionId: nodeExecutionID,
				Status:          status,
				StatusReason:    "component_deployed_by_node_agent",
				Labels: map[string]string{
					runtimeLabelRelation: "agent_execution",
				},
			})
		}
	}
}

func (b *runtimeTopologyBuilder) addLogicalConnections(
	rec *models.TestRunRecord,
	spec *topology.TopologySpec,
	components map[string]*topology.Component,
	componentPlans map[string]*deploymentpb.ComponentDeployment,
) {
	for idx, conn := range spec.GetConnections() {
		fromID := conn.GetFromComponentId()
		toID := conn.GetToComponentId()
		if fromID == "" || toID == "" {
			continue
		}
		if components[fromID] == nil {
			b.addSyntheticExternalComponent(fromID)
		}
		if components[toID] == nil {
			b.addSyntheticExternalComponent(toID)
		}
		status := logicalConnectionStatus(rec, componentPlans[fromID], componentPlans[toID])
		kind := conn.GetKind()
		if kind == topology.Connection_KIND_UNSPECIFIED {
			kind = topology.Connection_KIND_FLOW
		}
		protocol := conn.GetProtocol()
		if protocol == topology.Connection_PROTOCOL_UNSPECIFIED {
			protocol = topology.Connection_PROTOCOL_TCP
		}
		mode := conn.GetMode()
		if mode == topology.Connection_MODE_UNSPECIFIED {
			mode = topology.Connection_MODE_REQUEST
		}
		endpointName := conn.GetEndpointName()
		if endpointName == "" {
			endpointName = logicalConnectionEndpointName(kind, protocol)
		}
		edge := &topology.RuntimeConnection{
			Id:           runtimeEdgeID("logical", strconv.Itoa(idx), fromID, toID, endpointName),
			FromNodeId:   runtimeComponentNodeID(fromID),
			ToNodeId:     runtimeComponentNodeID(toID),
			Kind:         kind,
			Protocol:     protocol,
			Mode:         mode,
			EndpointName: endpointName,
			Status:       status,
			StatusReason: "topology_spec_connection",
			Labels: map[string]string{
				runtimeLabelSource:   "topology_spec",
				runtimeLabelRelation: "logical_connection",
				"colocated":          strconv.FormatBool(conn.GetColocated()),
			},
			Tags: conn.GetTags(),
		}
		if conn.Port != nil {
			edge.Port = runtimePort(conn.GetPort())
		}
		b.addEdge(edge)
	}
}

func logicalConnectionEndpointName(kind topology.Connection_Kind, protocol topology.Connection_Protocol) string {
	kindName := strings.TrimPrefix(strings.ToLower(kind.String()), "kind_")
	protocolName := strings.TrimPrefix(strings.ToLower(protocol.String()), "protocol_")
	switch {
	case kindName != "" && protocolName != "":
		return kindName + "/" + protocolName
	case kindName != "":
		return kindName
	case protocolName != "":
		return protocolName
	default:
		return "logical_connection"
	}
}

func (b *runtimeTopologyBuilder) addComponentDependencies(plan *deploymentpb.DeploymentPlan) {
	for _, component := range plan.GetComponents() {
		if component == nil || component.GetComponentId() == "" {
			continue
		}
		for _, dependencyID := range component.GetDependsOnComponentIds() {
			if dependencyID == "" {
				continue
			}
			b.addEdge(&topology.RuntimeConnection{
				Id:              runtimeEdgeID("dependency", dependencyID, component.GetComponentId()),
				FromNodeId:      runtimeComponentNodeID(component.GetComponentId()),
				ToNodeId:        runtimeComponentNodeID(dependencyID),
				Kind:            topology.Connection_KIND_SUPPORT,
				Protocol:        topology.Connection_PROTOCOL_CONTROL,
				Mode:            topology.Connection_MODE_REQUEST,
				EndpointName:    "depends_on",
				Phase:           executeDeploymentPlanNodeName,
				NodeExecutionId: deploymentbuilder.ComponentExecutionID(component.GetComponentId()),
				Status:          pendingIfUnspecified(component.GetStatus()),
				StatusReason:    "deployment_plan_dependency",
				Labels: map[string]string{
					runtimeLabelSource:   "deployment_plan",
					runtimeLabelRelation: "depends_on",
				},
			})
		}
	}
}

func (b *runtimeTopologyBuilder) addMonitoringRuntime(
	rec *models.TestRunRecord,
	serverAddr string,
	machines map[string]*deploymentpb.MachineState,
	components map[string]*topology.Component,
	componentIDsByMachine map[string][]string,
) {
	if serverAddr == "" {
		return
	}

	machineIDs := sortedMapKeys(componentIDsByMachine)
	for _, machineID := range machineIDs {
		if machineID == "" {
			continue
		}
		status := monitorRuntimeStatus(rec, machines[machineID])
		b.addNodeExporter(machineID, status, nil)
		b.addVmagent(machineID, status, nil)
		b.addVector(machineID, status, nil)
		b.addVmagentBaseEdges(machineID, status, nil)
		b.addVectorBaseEdges(machineID, status, nil, componentIDsByMachine[machineID])

		for _, componentID := range componentIDsByMachine[machineID] {
			component := components[componentID]
			if component == nil {
				continue
			}
			b.addDatabaseMonitoring(machineID, component, status, nil)
		}
	}
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

func (b *runtimeTopologyBuilder) addVectorBaseEdges(machineID string, status commonpb.Status, stage *workflowpb.Stage, componentIDs []string) {
	b.addVectorControlPlaneEdge(machineID, status, stage)
	for _, componentID := range componentIDs {
		if componentID == "" {
			continue
		}
		b.addEdge(&topology.RuntimeConnection{
			Id:              runtimeEdgeID("monitor", machineID, "vector", "component-logs", componentID),
			FromNodeId:      runtimeMonitorNodeID(machineID, "vector"),
			ToNodeId:        runtimeComponentNodeID(componentID),
			Kind:            topology.Connection_KIND_OBSERVATION,
			Protocol:        topology.Connection_PROTOCOL_UNSPECIFIED,
			Mode:            topology.Connection_MODE_STREAM,
			EndpointName:    "logs/journald_and_files",
			Phase:           executeDeploymentPlanNodeName,
			NodeExecutionId: stageNodeExecutionID(stage),
			Status:          pendingIfUnspecified(status),
			StatusReason:    runtimeStageStatusReason(stage, "vector_collects_local_component_logs"),
			StartedAt:       stageStartedAt(stage),
			FinishedAt:      stageFinishedAt(stage),
			Labels: map[string]string{
				runtimeLabelRelation: "local_log_collection",
			},
		})
	}
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

func (b *runtimeTopologyBuilder) addDatabaseMonitoring(machineID string, component *topology.Component, status commonpb.Status, stage *workflowpb.Stage) {
	if component == nil {
		return
	}
	switch databaseMetricKind(component.GetEngine(), component.GetRole()) {
	case "postgres":
		b.addPostgresExporter(machineID, status, stage)
		b.addEdge(&topology.RuntimeConnection{
			Id:              runtimeEdgeID("monitor", machineID, "postgres_exporter", component.GetId(), "local-db"),
			FromNodeId:      runtimeMonitorNodeID(machineID, "postgres_exporter"),
			ToNodeId:        runtimeComponentNodeID(component.GetId()),
			Kind:            topology.Connection_KIND_OBSERVATION,
			Protocol:        topology.Connection_PROTOCOL_TCP,
			Mode:            topology.Connection_MODE_REQUEST,
			EndpointName:    "postgres_local",
			Port:            runtimePort(5432),
			Phase:           executeDeploymentPlanNodeName,
			NodeExecutionId: stageNodeExecutionID(stage),
			Status:          pendingIfUnspecified(status),
			StatusReason:    runtimeStageStatusReason(stage, "postgres_exporter_reads_local_postgres"),
			StartedAt:       stageStartedAt(stage),
			FinishedAt:      stageFinishedAt(stage),
			Labels:          map[string]string{runtimeLabelRelation: "db_exporter_local_read"},
		})
		b.addEdge(&topology.RuntimeConnection{
			Id:              runtimeEdgeID("monitor", machineID, "vmagent", "postgres_exporter"),
			FromNodeId:      runtimeMonitorNodeID(machineID, "vmagent"),
			ToNodeId:        runtimeMonitorNodeID(machineID, "postgres_exporter"),
			Kind:            topology.Connection_KIND_OBSERVATION,
			Protocol:        topology.Connection_PROTOCOL_PROMETHEUS_PULL,
			Mode:            topology.Connection_MODE_REQUEST,
			EndpointName:    "postgres",
			Port:            runtimePort(9187),
			Phase:           executeDeploymentPlanNodeName,
			NodeExecutionId: stageNodeExecutionID(stage),
			Status:          pendingIfUnspecified(status),
			StatusReason:    runtimeStageStatusReason(stage, "vmagent_scrapes_postgres_exporter"),
			StartedAt:       stageStartedAt(stage),
			FinishedAt:      stageFinishedAt(stage),
			Labels:          map[string]string{runtimeLabelRelation: "metrics_scrape"},
		})
	case "mysql":
		b.addMysqlExporter(machineID, status, stage)
		b.addEdge(&topology.RuntimeConnection{
			Id:              runtimeEdgeID("monitor", machineID, "mysqld_exporter", component.GetId(), "local-db"),
			FromNodeId:      runtimeMonitorNodeID(machineID, "mysqld_exporter"),
			ToNodeId:        runtimeComponentNodeID(component.GetId()),
			Kind:            topology.Connection_KIND_OBSERVATION,
			Protocol:        topology.Connection_PROTOCOL_TCP,
			Mode:            topology.Connection_MODE_REQUEST,
			EndpointName:    "mysql_local",
			Port:            runtimePort(3306),
			Phase:           executeDeploymentPlanNodeName,
			NodeExecutionId: stageNodeExecutionID(stage),
			Status:          pendingIfUnspecified(status),
			StatusReason:    runtimeStageStatusReason(stage, "mysqld_exporter_reads_local_mysql"),
			StartedAt:       stageStartedAt(stage),
			FinishedAt:      stageFinishedAt(stage),
			Labels:          map[string]string{runtimeLabelRelation: "db_exporter_local_read"},
		})
		b.addEdge(&topology.RuntimeConnection{
			Id:              runtimeEdgeID("monitor", machineID, "vmagent", "mysqld_exporter"),
			FromNodeId:      runtimeMonitorNodeID(machineID, "vmagent"),
			ToNodeId:        runtimeMonitorNodeID(machineID, "mysqld_exporter"),
			Kind:            topology.Connection_KIND_OBSERVATION,
			Protocol:        topology.Connection_PROTOCOL_PROMETHEUS_PULL,
			Mode:            topology.Connection_MODE_REQUEST,
			EndpointName:    "mysql",
			Port:            runtimePort(9104),
			Phase:           executeDeploymentPlanNodeName,
			NodeExecutionId: stageNodeExecutionID(stage),
			Status:          pendingIfUnspecified(status),
			StatusReason:    runtimeStageStatusReason(stage, "vmagent_scrapes_mysqld_exporter"),
			StartedAt:       stageStartedAt(stage),
			FinishedAt:      stageFinishedAt(stage),
			Labels:          map[string]string{runtimeLabelRelation: "metrics_scrape"},
		})
	case "picodata":
		b.addDirectMetricsEdge(machineID, component.GetId(), "picodata", 8081, status, stage)
	case "ydb_storage":
		b.addDirectMetricsEdge(machineID, component.GetId(), "ydb_storage", 8765, status, stage)
	case "ydb_database":
		b.addDirectMetricsEdge(machineID, component.GetId(), "ydb_database", 8766, status, stage)
	}
}

func (b *runtimeTopologyBuilder) addDirectMetricsEdge(machineID, componentID, endpoint string, port uint32, status commonpb.Status, stage *workflowpb.Stage) {
	b.addEdge(&topology.RuntimeConnection{
		Id:              runtimeEdgeID("monitor", machineID, "vmagent", componentID, endpoint),
		FromNodeId:      runtimeMonitorNodeID(machineID, "vmagent"),
		ToNodeId:        runtimeComponentNodeID(componentID),
		Kind:            topology.Connection_KIND_OBSERVATION,
		Protocol:        topology.Connection_PROTOCOL_PROMETHEUS_PULL,
		Mode:            topology.Connection_MODE_REQUEST,
		EndpointName:    endpoint,
		Port:            runtimePort(port),
		Phase:           executeDeploymentPlanNodeName,
		NodeExecutionId: stageNodeExecutionID(stage),
		Status:          pendingIfUnspecified(status),
		StatusReason:    runtimeStageStatusReason(stage, "vmagent_scrapes_component_metrics_directly"),
		StartedAt:       stageStartedAt(stage),
		FinishedAt:      stageFinishedAt(stage),
		Labels:          map[string]string{runtimeLabelRelation: "metrics_scrape"},
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

func (b *runtimeTopologyBuilder) addSyntheticExternalComponent(componentID string) {
	b.addNode(&topology.RuntimeNode{
		Id:           runtimeComponentNodeID(componentID),
		Kind:         topology.RuntimeNode_KIND_EXTERNAL,
		Label:        componentID,
		ComponentId:  componentID,
		Status:       commonpb.Status_STATUS_PENDING,
		StatusReason: "referenced_by_connection_not_present_in_spec",
		Labels: map[string]string{
			runtimeLabelSource:       "topology_connection_reference",
			runtimeLabelRuntimeClass: "external",
		},
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

func machineStatesByID(state *deploymentpb.InfrastructureState) map[string]*deploymentpb.MachineState {
	out := make(map[string]*deploymentpb.MachineState)
	for _, machine := range state.GetMachines() {
		if machine == nil || machine.GetNodeId() == "" {
			continue
		}
		out[machine.GetNodeId()] = machine
	}
	return out
}

func topologyComponentsByID(spec *topology.TopologySpec) map[string]*topology.Component {
	out := make(map[string]*topology.Component)
	for _, component := range spec.GetComponents() {
		if component != nil && component.GetId() != "" {
			out[component.GetId()] = component
		}
	}
	for _, component := range spec.GetExternalComponents() {
		if component != nil && component.GetId() != "" {
			out[component.GetId()] = component
		}
	}
	return out
}

func componentDeploymentsByID(plan *deploymentpb.DeploymentPlan) map[string]*deploymentpb.ComponentDeployment {
	out := make(map[string]*deploymentpb.ComponentDeployment)
	for _, component := range plan.GetComponents() {
		if component != nil && component.GetComponentId() != "" {
			out[component.GetComponentId()] = component
		}
	}
	return out
}

func componentMachineIDs(spec *topology.TopologySpec, plan *deploymentpb.DeploymentPlan) map[string]string {
	out := make(map[string]string)
	for _, node := range spec.GetNodes() {
		for _, componentID := range node.GetComponentIds() {
			if componentID != "" && node.GetId() != "" {
				out[componentID] = node.GetId()
			}
		}
	}
	for _, component := range plan.GetComponents() {
		if component.GetComponentId() != "" && component.GetNodeId() != "" {
			out[component.GetComponentId()] = component.GetNodeId()
		}
	}
	return out
}

func runtimeMachineIDs(spec *topology.TopologySpec, state *deploymentpb.InfrastructureState, plan *deploymentpb.DeploymentPlan, rs *workflowpb.RunState) []string {
	ids := make(map[string]struct{})
	for _, node := range spec.GetNodes() {
		if node.GetId() != "" {
			ids[node.GetId()] = struct{}{}
		}
	}
	for _, machine := range state.GetMachines() {
		if machine.GetNodeId() != "" {
			ids[machine.GetNodeId()] = struct{}{}
		}
	}
	for _, component := range plan.GetComponents() {
		if component.GetNodeId() != "" {
			ids[component.GetNodeId()] = struct{}{}
		}
	}
	for _, stage := range rs.GetStages() {
		if stage.GetMachineId() != "" {
			ids[stage.GetMachineId()] = struct{}{}
		}
	}
	return sortedSetKeys(ids)
}

func runtimeComponentIDsByMachine(componentMachines map[string]string) map[string][]string {
	out := make(map[string][]string)
	for componentID, machineID := range componentMachines {
		if componentID == "" || machineID == "" {
			continue
		}
		out[machineID] = append(out[machineID], componentID)
	}
	for machineID := range out {
		sort.Strings(out[machineID])
	}
	return out
}

func controlPlaneRuntimeStatus(rec *models.TestRunRecord) commonpb.Status {
	if rec == nil {
		return commonpb.Status_STATUS_RUNNING
	}
	switch rec.GetStatus() {
	case commonpb.Status_STATUS_COMPLETED:
		return commonpb.Status_STATUS_COMPLETED
	case commonpb.Status_STATUS_FAILED, commonpb.Status_STATUS_CANCELLED, commonpb.Status_STATUS_CANCELLING:
		return rec.GetStatus()
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

func agentRuntimeStatus(rec *models.TestRunRecord, machine *deploymentpb.MachineState, currentExecutionID string) commonpb.Status {
	if currentExecutionID != "" {
		return commonpb.Status_STATUS_RUNNING
	}
	if rec != nil {
		switch rec.GetStatus() {
		case commonpb.Status_STATUS_COMPLETED:
			return commonpb.Status_STATUS_COMPLETED
		case commonpb.Status_STATUS_FAILED, commonpb.Status_STATUS_CANCELLED, commonpb.Status_STATUS_CANCELLING:
			return rec.GetStatus()
		}
	}
	return workerStatus(machine, "")
}

func agentRuntimeStatusReason(rec *models.TestRunRecord, machine *deploymentpb.MachineState, currentExecutionID string) string {
	if currentExecutionID != "" {
		return "agent_executing_stage"
	}
	if rec != nil && isTerminalStatus(rec.GetStatus()) {
		return "run_terminal_agent_not_marked_offline_without_registry_sample"
	}
	if machine == nil {
		return "agent_waiting_for_machine"
	}
	return "agent_materialized_with_machine"
}

func componentRuntimeStatus(rec *models.TestRunRecord, component *topology.Component, deployment *deploymentpb.ComponentDeployment) commonpb.Status {
	if component.GetKind() == topology.Component_KIND_EXTERNAL {
		return commonpb.Status_STATUS_COMPLETED
	}
	if deployment == nil {
		return commonpb.Status_STATUS_PENDING
	}
	return inheritedDeploymentStatus(deployment.GetStatus(), recordExecutePlanStatus(rec))
}

func componentRuntimeStatusReason(status commonpb.Status) string {
	status = pendingIfUnspecified(status)
	if status == commonpb.Status_STATUS_PENDING {
		return "component_waiting_for_deployment"
	}
	return "component_" + strings.ToLower(status.String())
}

func logicalConnectionStatus(rec *models.TestRunRecord, from *deploymentpb.ComponentDeployment, to *deploymentpb.ComponentDeployment) commonpb.Status {
	if from == nil || to == nil {
		if rec != nil && rec.GetStatus() == commonpb.Status_STATUS_COMPLETED {
			return commonpb.Status_STATUS_COMPLETED
		}
		return commonpb.Status_STATUS_PENDING
	}
	left := inheritedDeploymentStatus(from.GetStatus(), recordExecutePlanStatus(rec))
	right := inheritedDeploymentStatus(to.GetStatus(), recordExecutePlanStatus(rec))
	if left == commonpb.Status_STATUS_FAILED || right == commonpb.Status_STATUS_FAILED {
		return commonpb.Status_STATUS_FAILED
	}
	if activeStatus(left) || activeStatus(right) {
		return commonpb.Status_STATUS_RUNNING
	}
	if left == commonpb.Status_STATUS_COMPLETED && right == commonpb.Status_STATUS_COMPLETED {
		return commonpb.Status_STATUS_COMPLETED
	}
	if left == commonpb.Status_STATUS_DEPLOYED && right == commonpb.Status_STATUS_DEPLOYED {
		return commonpb.Status_STATUS_DEPLOYED
	}
	return commonpb.Status_STATUS_PENDING
}

func monitorRuntimeStatus(rec *models.TestRunRecord, machine *deploymentpb.MachineState) commonpb.Status {
	executeStatus := recordExecutePlanStatus(rec)
	switch executeStatus {
	case commonpb.Status_STATUS_COMPLETED, commonpb.Status_STATUS_FAILED, commonpb.Status_STATUS_CANCELLED, commonpb.Status_STATUS_RUNNING:
		return executeStatus
	}
	if machine == nil {
		return commonpb.Status_STATUS_PENDING
	}
	if pendingIfUnspecified(machine.GetStatus()) == commonpb.Status_STATUS_DEPLOYED {
		return commonpb.Status_STATUS_PENDING
	}
	return pendingIfUnspecified(machine.GetStatus())
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

func databaseMetricKind(engine, role string) string {
	switch engine {
	case "postgres":
		if role == "master" || role == "replica" {
			return "postgres"
		}
	case "mysql":
		if role == "primary" || role == "replica" {
			return "mysql"
		}
	case "picodata":
		if role == "instance" {
			return "picodata"
		}
	case "ydb":
		switch role {
		case "storage":
			return "ydb_storage"
		case "database":
			return "ydb_database"
		}
	}
	return ""
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

func operationEndpointName(op *monitorpb.PipelineOperation) string {
	switch op.GetKind() {
	case monitorpb.OperationKind_OPERATION_KIND_CREATE_DIR:
		return "create_dir"
	case monitorpb.OperationKind_OPERATION_KIND_WRITE_FILE:
		if op.GetFilePath() != "" {
			return "write_file:" + op.GetFilePath()
		}
		return "write_file"
	case monitorpb.OperationKind_OPERATION_KIND_FETCH_FILE:
		if op.GetTarget() != "" {
			return "fetch_file:" + op.GetTarget()
		}
		return "fetch_file"
	case monitorpb.OperationKind_OPERATION_KIND_CALL_CMD:
		return "call_cmd"
	default:
		if op.GetTarget() != "" {
			return op.GetTarget()
		}
		return "agent_action"
	}
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

func sortedMapKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedSetKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
