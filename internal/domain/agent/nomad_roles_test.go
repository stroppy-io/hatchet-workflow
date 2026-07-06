package agent

import (
	"testing"
)

func TestAssignNomadRoles_GatewayInList(t *testing.T) {
	machineIDs := []string{"a", "b", "c"}
	got := AssignNomadRoles(machineIDs, "a", "10.0.0.5")

	want := map[string]NomadAssignment{
		"a": {Role: NomadRoleServer, ServerAddr: ""},
		"b": {Role: NomadRoleClient, ServerAddr: "10.0.0.5:4647"},
		"c": {Role: NomadRoleClient, ServerAddr: "10.0.0.5:4647"},
	}

	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d (got=%+v)", len(got), len(want), got)
	}
	for id, wantAssignment := range want {
		gotAssignment, ok := got[id]
		if !ok {
			t.Fatalf("missing assignment for machine %q", id)
		}
		if gotAssignment != wantAssignment {
			t.Errorf("assignment[%q] = %+v, want %+v", id, gotAssignment, wantAssignment)
		}
	}
}

func TestAssignNomadRoles_GatewayNotInList(t *testing.T) {
	machineIDs := []string{"b", "c"}
	got := AssignNomadRoles(machineIDs, "a", "10.0.0.5")

	want := map[string]NomadAssignment{
		"b": {Role: NomadRoleClient, ServerAddr: "10.0.0.5:4647"},
		"c": {Role: NomadRoleClient, ServerAddr: "10.0.0.5:4647"},
	}

	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d (got=%+v)", len(got), len(want), got)
	}
	for id, wantAssignment := range want {
		gotAssignment, ok := got[id]
		if !ok {
			t.Fatalf("missing assignment for machine %q", id)
		}
		if gotAssignment != wantAssignment {
			t.Errorf("assignment[%q] = %+v, want %+v", id, gotAssignment, wantAssignment)
		}
	}
	if _, ok := got["a"]; ok {
		t.Errorf("unexpected assignment for gateway id %q that was not in machineIDs", "a")
	}
}

func TestAssignNomadRoles_Deterministic(t *testing.T) {
	machineIDs := []string{"a", "b", "c"}
	first := AssignNomadRoles(machineIDs, "a", "10.0.0.5")
	second := AssignNomadRoles(machineIDs, "a", "10.0.0.5")

	if len(first) != len(second) {
		t.Fatalf("len(first) = %d, len(second) = %d", len(first), len(second))
	}
	for id, wantAssignment := range first {
		gotAssignment, ok := second[id]
		if !ok {
			t.Fatalf("second run missing assignment for machine %q", id)
		}
		if gotAssignment != wantAssignment {
			t.Errorf("second run assignment[%q] = %+v, want %+v", id, gotAssignment, wantAssignment)
		}
	}
}

func TestAssignNomadRoles_SingleMachineIsGateway(t *testing.T) {
	got := AssignNomadRoles([]string{"a"}, "a", "10.0.0.5")

	want := map[string]NomadAssignment{
		"a": {Role: NomadRoleServer, ServerAddr: ""},
	}

	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d (got=%+v)", len(got), len(want), got)
	}
	if gotAssignment := got["a"]; gotAssignment != want["a"] {
		t.Errorf("assignment[%q] = %+v, want %+v", "a", gotAssignment, want["a"])
	}
}
