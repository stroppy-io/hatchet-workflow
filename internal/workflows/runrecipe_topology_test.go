package workflows

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
)

// TestDeriveRecipeTopologyProjectsMachinesGroupsAndServices asserts
// deriveRecipeTopology (runrecipe_topology.go) turns a compiled plan's
// services + ProvisionActivity's machines into a RecipeTopologySnapshot with
// one MachineNode per machine, stamped with its group and the services
// on_group-placed there, sorted deterministically by (group, node_id)
// regardless of map iteration order.
func TestDeriveRecipeTopologyProjectsMachinesGroupsAndServices(t *testing.T) {
	plan := &dslpb.CompiledPlan{
		Provider: &dslpb.ProviderRef{Name: "yandex"},
		MachineGroups: []*dslpb.MachineGroup{
			{Name: "db", Count: 2},
			{Name: "runner", Count: 1},
		},
		Services: []*dslpb.ServiceSpec{
			{Name: "patroni-postgres", OnGroup: "db", Image: "patroni:1.0"},
			{Name: "node-exporter", OnGroup: "db", Image: "node-exporter:1.0"},
			{Name: "stroppy", OnGroup: "runner", Image: "stroppy:1.2.3"},
		},
	}
	machines := map[string][]*deploymentpb.MachineState{
		"db": {
			{NodeId: "db-1", Status: common.Status_STATUS_DEPLOYED, Endpoints: []*deploymentpb.Endpoint{{Name: "private", Address: "10.0.0.2"}}},
			{NodeId: "db-0", Status: common.Status_STATUS_DEPLOYED, Endpoints: []*deploymentpb.Endpoint{{Name: "private", Address: "10.0.0.1"}}},
		},
		"runner": {
			{NodeId: "runner-0", Status: common.Status_STATUS_DEPLOYED, Endpoints: []*deploymentpb.Endpoint{{Name: "private", Address: "10.0.0.3"}}},
		},
	}

	snap := deriveRecipeTopology(plan, machines)

	if got, want := snap.GetProvider(), "yandex"; got != want {
		t.Errorf("provider = %q, want %q", got, want)
	}
	nodes := snap.GetNodes()
	if got, want := len(nodes), 3; got != want {
		t.Fatalf("node count = %d, want %d", got, want)
	}
	// Sorted by (group, node_id): db-0, db-1, runner-0.
	wantIDs := []string{"db-0", "db-1", "runner-0"}
	for i, want := range wantIDs {
		if got := nodes[i].GetNodeId(); got != want {
			t.Fatalf("nodes[%d].node_id = %q, want %q", i, got, want)
		}
	}
	if got, want := nodes[0].GetGroup(), "db"; got != want {
		t.Errorf("nodes[0].group = %q, want %q", got, want)
	}
	if got, want := nodes[0].GetIp(), "10.0.0.1"; got != want {
		t.Errorf("nodes[0].ip = %q, want %q", got, want)
	}
	if got, want := nodes[0].GetStatus(), common.Status_STATUS_DEPLOYED; got != want {
		t.Errorf("nodes[0].status = %s, want %s", got, want)
	}
	dbServices := nodes[0].GetServices()
	if got, want := len(dbServices), 2; got != want {
		t.Fatalf("nodes[0].services count = %d, want %d", got, want)
	}
	if got, want := dbServices[0].GetName(), "patroni-postgres"; got != want {
		t.Errorf("nodes[0].services[0].name = %q, want %q", got, want)
	}
	if got, want := dbServices[1].GetName(), "node-exporter"; got != want {
		t.Errorf("nodes[0].services[1].name = %q, want %q", got, want)
	}

	runnerNode := nodes[2]
	if got, want := runnerNode.GetGroup(), "runner"; got != want {
		t.Errorf("nodes[2].group = %q, want %q", got, want)
	}
	runnerServices := runnerNode.GetServices()
	if got, want := len(runnerServices), 1; got != want {
		t.Fatalf("nodes[2].services count = %d, want %d", got, want)
	}
	if got, want := runnerServices[0].GetName(), "stroppy"; got != want {
		t.Errorf("nodes[2].services[0].name = %q, want %q", got, want)
	}
	if got, want := runnerServices[0].GetImage(), "stroppy:1.2.3"; got != want {
		t.Errorf("nodes[2].services[0].image = %q, want %q", got, want)
	}
}

// TestDeriveRecipeTopologySkipsMachinesWithoutNodeID guards against a
// provider returning a MachineState with an empty node id (should never
// happen in practice, but deriveRecipeTopology must not persist an
// unidentifiable node).
func TestDeriveRecipeTopologySkipsMachinesWithoutNodeID(t *testing.T) {
	plan := &dslpb.CompiledPlan{Provider: &dslpb.ProviderRef{Name: "docker"}}
	machines := map[string][]*deploymentpb.MachineState{
		"app": {{NodeId: ""}},
	}

	snap := deriveRecipeTopology(plan, machines)

	if got := len(snap.GetNodes()); got != 0 {
		t.Fatalf("node count = %d, want 0 (machine with empty node_id must be skipped)", got)
	}
}
