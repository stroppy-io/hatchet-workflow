package workload

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

const (
	Engine       = "stroppy"
	RunnerRole   = "runner"
	RunnerNodeID = "workload-runner-1"
)

func ExtendTopologySpec(spec *topologypb.TopologySpec, input *domain.Workload) (*topologypb.TopologySpec, error) {
	if spec == nil {
		return nil, fmt.Errorf("topology spec is required")
	}
	if input == nil {
		return nil, fmt.Errorf("workload is required")
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}
	if hasComponent(spec, RunnerNodeID) || hasNode(spec, RunnerNodeID) {
		return spec, nil
	}

	targets := workloadTargets(spec)
	if len(targets) == 0 {
		return nil, fmt.Errorf("workload runner has no database entrypoint target")
	}

	spec.Components = append(spec.Components, component(RunnerNodeID, RunnerNodeID))
	spec.Nodes = append(spec.Nodes, node(RunnerNodeID, []string{RunnerNodeID}))
	for _, target := range targets {
		endpoint, port, protocol := workloadConnection(input, target)
		spec.Connections = append(spec.Connections, &topologypb.Connection{
			FromComponentId: RunnerNodeID,
			ToComponentId:   target.GetId(),
			Kind:            topologypb.Connection_KIND_FLOW,
			Protocol:        protocol,
			Mode:            topologypb.Connection_MODE_STREAM,
			EndpointName:    endpoint,
			Port:            &port,
			Tags:            tags(Engine, "workload"),
		})
	}
	if spec.Labels == nil {
		spec.Labels = map[string]string{}
	}
	spec.Labels["workload_engine"] = Engine
	spec.Labels["workload_runner"] = RunnerNodeID
	spec.Labels["workload_vus"] = strconv.FormatUint(uint64(input.GetExecution().GetVus()), 10)
	return spec, nil
}

func workloadTargets(spec *topologypb.TopologySpec) []*topologypb.Component {
	components := componentsByID(spec)
	incomingProxy := map[string]struct{}{}
	for _, connection := range spec.GetConnections() {
		from := components[connection.GetFromComponentId()]
		to := components[connection.GetToComponentId()]
		if from.GetKind() == topologypb.Component_KIND_PROXY && to.GetKind() == topologypb.Component_KIND_PROXY {
			incomingProxy[to.GetId()] = struct{}{}
		}
	}

	var proxies []*topologypb.Component
	for _, candidate := range spec.GetComponents() {
		if candidate.GetKind() != topologypb.Component_KIND_PROXY {
			continue
		}
		if _, ok := incomingProxy[candidate.GetId()]; ok {
			continue
		}
		proxies = append(proxies, candidate)
	}
	if len(proxies) > 0 {
		sortComponents(proxies)
		return proxies
	}

	var databases []*topologypb.Component
	for _, candidate := range spec.GetComponents() {
		if candidate.GetKind() == topologypb.Component_KIND_DATABASE || candidate.GetKind() == topologypb.Component_KIND_EXTERNAL {
			databases = append(databases, candidate)
		}
	}
	sortComponents(databases)
	return databases
}

func workloadConnection(input *domain.Workload, target *topologypb.Component) (string, uint32, topologypb.Connection_Protocol) {
	protocol := input.GetProtocol()
	meta := workloadProtocols[protocol]

	switch protocol {
	case domain.Workload_PROTOCOL_MYSQL:
		if target.GetRole() == "proxysql" {
			return "mysql", 6033, topologypb.Connection_PROTOCOL_TCP
		}
		return "mysql", meta.port, topologypb.Connection_PROTOCOL_TCP
	case domain.Workload_PROTOCOL_PICODATA:
		return "pgproto", meta.port, topologypb.Connection_PROTOCOL_TCP
	case domain.Workload_PROTOCOL_YDB_GRPC, domain.Workload_PROTOCOL_YDB_GRPCS:
		return "grpc", meta.port, topologypb.Connection_PROTOCOL_GRPC
	case domain.Workload_PROTOCOL_COCKROACH:
		return "sql", meta.port, topologypb.Connection_PROTOCOL_TCP
	default:
		if target.GetRole() == "pgbouncer" {
			return "pgbouncer", 6432, topologypb.Connection_PROTOCOL_POOL
		}
		return "postgres", workloadProtocols[domain.Workload_PROTOCOL_PG].port, topologypb.Connection_PROTOCOL_TCP
	}
}

func component(id, nodeID string) *topologypb.Component {
	return &topologypb.Component{
		Id:     id,
		Kind:   topologypb.Component_KIND_WORKLOAD,
		Engine: Engine,
		Role:   RunnerRole,
		Labels: map[string]string{
			"engine":  Engine,
			"role":    RunnerRole,
			"node_id": nodeID,
		},
		Tags: tags(Engine, RunnerRole),
	}
}

func node(id string, componentIDs []string) *topologypb.Node {
	return &topologypb.Node{
		Id:           id,
		ComponentIds: componentIDs,
		Labels: map[string]string{
			"engine":  Engine,
			"role":    RunnerRole,
			"ordinal": "1",
		},
		Tags: tags(Engine, RunnerRole),
	}
}

func componentsByID(spec *topologypb.TopologySpec) map[string]*topologypb.Component {
	components := make(map[string]*topologypb.Component, len(spec.GetComponents()))
	for _, component := range spec.GetComponents() {
		components[component.GetId()] = component
	}
	return components
}

func hasComponent(spec *topologypb.TopologySpec, id string) bool {
	for _, component := range spec.GetComponents() {
		if component.GetId() == id {
			return true
		}
	}
	return false
}

func hasNode(spec *topologypb.TopologySpec, id string) bool {
	for _, node := range spec.GetNodes() {
		if node.GetId() == id {
			return true
		}
	}
	return false
}

func sortComponents(components []*topologypb.Component) {
	sort.SliceStable(components, func(i, j int) bool {
		return components[i].GetId() < components[j].GetId()
	})
}

func tags(values ...string) *common.Tags {
	return &common.Tags{Tags: values}
}
