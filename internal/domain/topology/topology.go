package topology

import (
	"errors"
	"fmt"

	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

type Index struct {
	spec *topologypb.TopologySpec

	nodesByID         map[string]*topologypb.Node
	componentsByID    map[string]*topologypb.Component
	nodeByComponentID map[string]*topologypb.Node
}

func NewIndex(spec *topologypb.TopologySpec) (*Index, error) {
	if spec == nil {
		return nil, errors.New("topology spec is required")
	}
	if err := spec.Validate(); err != nil {
		return nil, err
	}

	idx := &Index{
		spec:              spec,
		nodesByID:         make(map[string]*topologypb.Node, len(spec.GetNodes())),
		componentsByID:    make(map[string]*topologypb.Component, len(spec.GetComponents())+len(spec.GetExternalComponents())),
		nodeByComponentID: make(map[string]*topologypb.Node, len(spec.GetComponents())),
	}

	for _, component := range spec.GetComponents() {
		if _, ok := idx.componentsByID[component.GetId()]; ok {
			return nil, fmt.Errorf("duplicate component id %q", component.GetId())
		}
		idx.componentsByID[component.GetId()] = component
	}
	for _, component := range spec.GetExternalComponents() {
		if _, ok := idx.componentsByID[component.GetId()]; ok {
			return nil, fmt.Errorf("duplicate component id %q", component.GetId())
		}
		idx.componentsByID[component.GetId()] = component
	}

	for _, node := range spec.GetNodes() {
		if _, ok := idx.nodesByID[node.GetId()]; ok {
			return nil, fmt.Errorf("duplicate node id %q", node.GetId())
		}
		idx.nodesByID[node.GetId()] = node

		for _, componentID := range node.GetComponentIds() {
			if _, ok := idx.componentsByID[componentID]; !ok {
				return nil, fmt.Errorf("node %q references unknown component %q", node.GetId(), componentID)
			}
			if previous, ok := idx.nodeByComponentID[componentID]; ok {
				return nil, fmt.Errorf("component %q is assigned to both node %q and node %q", componentID, previous.GetId(), node.GetId())
			}
			idx.nodeByComponentID[componentID] = node
		}
	}

	for _, component := range spec.GetComponents() {
		if _, ok := idx.nodeByComponentID[component.GetId()]; !ok {
			return nil, fmt.Errorf("component %q is not assigned to any node", component.GetId())
		}
	}

	for _, connection := range spec.GetConnections() {
		if _, ok := idx.componentsByID[connection.GetFromComponentId()]; !ok {
			return nil, fmt.Errorf("connection references unknown from_component_id %q", connection.GetFromComponentId())
		}
		if _, ok := idx.componentsByID[connection.GetToComponentId()]; !ok {
			return nil, fmt.Errorf("connection references unknown to_component_id %q", connection.GetToComponentId())
		}
		if connection.GetKind() == topologypb.Connection_KIND_UNSPECIFIED {
			return nil, fmt.Errorf("connection %q -> %q has unspecified kind", connection.GetFromComponentId(), connection.GetToComponentId())
		}
		if connection.GetProtocol() == topologypb.Connection_PROTOCOL_UNSPECIFIED {
			return nil, fmt.Errorf("connection %q -> %q has unspecified protocol", connection.GetFromComponentId(), connection.GetToComponentId())
		}
		if connection.GetMode() == topologypb.Connection_MODE_UNSPECIFIED {
			return nil, fmt.Errorf("connection %q -> %q has unspecified mode", connection.GetFromComponentId(), connection.GetToComponentId())
		}
	}

	return idx, nil
}

func (i *Index) Spec() *topologypb.TopologySpec {
	return i.spec
}

func (i *Index) Node(id string) (*topologypb.Node, bool) {
	node, ok := i.nodesByID[id]
	return node, ok
}

func (i *Index) Component(id string) (*topologypb.Component, bool) {
	component, ok := i.componentsByID[id]
	return component, ok
}

func (i *Index) NodeForComponentID(componentID string) (*topologypb.Node, bool) {
	node, ok := i.nodeByComponentID[componentID]
	return node, ok
}

func (i *Index) ComponentsOnNode(nodeID string) ([]*topologypb.Component, error) {
	node, ok := i.Node(nodeID)
	if !ok {
		return nil, fmt.Errorf("unknown node %q", nodeID)
	}

	components := make([]*topologypb.Component, 0, len(node.GetComponentIds()))
	for _, componentID := range node.GetComponentIds() {
		component, ok := i.Component(componentID)
		if !ok {
			return nil, fmt.Errorf("node %q references unknown component %q", nodeID, componentID)
		}
		components = append(components, component)
	}

	return components, nil
}

func (i *Index) OutgoingConnections(componentID string) []*topologypb.Connection {
	connections := make([]*topologypb.Connection, 0)
	for _, connection := range i.spec.GetConnections() {
		if connection.GetFromComponentId() == componentID {
			connections = append(connections, connection)
		}
	}
	return connections
}

func (i *Index) IncomingConnections(componentID string) []*topologypb.Connection {
	connections := make([]*topologypb.Connection, 0)
	for _, connection := range i.spec.GetConnections() {
		if connection.GetToComponentId() == componentID {
			connections = append(connections, connection)
		}
	}
	return connections
}
