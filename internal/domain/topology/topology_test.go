package topology

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

func TestNewIndexValidatesAndIndexesSpec(t *testing.T) {
	spec := &topologypb.TopologySpec{
		Nodes: []*topologypb.Node{
			{Id: "node-1", ComponentIds: []string{"postgres-master"}},
		},
		Components: []*topologypb.Component{
			component("postgres-master", topologypb.Component_KIND_DATABASE, "master"),
		},
		Tags: &common.Tags{Tags: []string{"test"}},
	}

	idx, err := NewIndex(spec)
	if err != nil {
		t.Fatalf("new index: %v", err)
	}

	node, ok := idx.NodeForComponentID("postgres-master")
	if !ok {
		t.Fatal("component node is missing")
	}
	if node.GetId() != "node-1" {
		t.Fatalf("node id = %q, want node-1", node.GetId())
	}

	components, err := idx.ComponentsOnNode("node-1")
	if err != nil {
		t.Fatalf("components on node: %v", err)
	}
	if got, want := len(components), 1; got != want {
		t.Fatalf("components on node = %d, want %d", got, want)
	}
}

func TestNewIndexRejectsUnknownNodeComponent(t *testing.T) {
	_, err := NewIndex(&topologypb.TopologySpec{
		Nodes: []*topologypb.Node{
			{Id: "node-1", ComponentIds: []string{"missing"}},
		},
		Components: []*topologypb.Component{
			component("postgres-master", topologypb.Component_KIND_DATABASE, "master"),
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewIndexRejectsUnknownConnectionEndpoint(t *testing.T) {
	_, err := NewIndex(&topologypb.TopologySpec{
		Nodes: []*topologypb.Node{
			{Id: "node-1", ComponentIds: []string{"postgres-master"}},
		},
		Components: []*topologypb.Component{
			component("postgres-master", topologypb.Component_KIND_DATABASE, "master"),
		},
		Connections: []*topologypb.Connection{
			{
				FromComponentId: "postgres-master",
				ToComponentId:   "missing",
				Kind:            topologypb.Connection_KIND_PROXY,
				Protocol:        topologypb.Connection_PROTOCOL_TCP,
				Mode:            topologypb.Connection_MODE_REQUEST,
			},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func component(id string, kind topologypb.Component_Kind, role string) *topologypb.Component {
	return &topologypb.Component{
		Id:     id,
		Kind:   kind,
		Engine: "postgres",
		Role:   role,
	}
}
