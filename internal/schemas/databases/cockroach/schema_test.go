package cockroach

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

// TestSchemaBuilds verifies the descriptor is well-formed (CockroachInputSchema
// panics via MustBuild otherwise).
func TestSchemaBuilds(t *testing.T) {
	s := CockroachInputSchema()
	if s == nil || s.GetId().GetName() != schemaName {
		t.Fatalf("unexpected schema id: %v", s.GetId())
	}
}

func values(t *testing.T, jsonStr string) *structpb.Struct {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	st, err := structpb.NewStruct(m)
	if err != nil {
		t.Fatalf("struct: %v", err)
	}
	return st
}

// assertNoErrors fails on any ERROR-severity FieldError.
func assertNoErrors(t *testing.T, st *structpb.Struct) {
	t.Helper()
	for _, e := range CockroachInputSchema().ValidateStruct(st) {
		if e.GetSeverity().String() == "ERROR" {
			t.Errorf("unexpected error: %s: %s", e.GetField(), e.GetMessage())
		}
	}
}

// TestSingleNodeValid: a single-node form leaves the cluster-only gates
// (replication_factor, gossip_join) inactive and validates clean.
func TestSingleNodeValid(t *testing.T) {
	assertNoErrors(t, values(t, `{
		"topology":"single","version":"24.2","node_count":1,
		"cluster":{"cache":".25","max_sql_memory":".25","security":"insecure"}
	}`))
}

// TestClusterValid: a logical 3-node cluster (no machines/provider here)
// validates clean — those concerns live at the cluster layer.
func TestClusterValid(t *testing.T) {
	assertNoErrors(t, values(t, `{
		"topology":"cluster","version":"24.2","node_count":3,
		"cluster":{
			"replication_factor":3,"cache":".25","max_sql_memory":".25",
			"gossip_join":true,"security":"insecure","max_offset":500,
			"cluster_settings":[{"key":"kv.snapshot_rebalance.max_rate","value":"64MiB"}]
		}
	}`))
}

// assertRuleFires fails unless the named rule emits a FieldError standalone.
func assertRuleFires(t *testing.T, ruleID string, st *structpb.Struct) {
	t.Helper()
	for _, e := range CockroachInputSchema().ValidateStruct(st) {
		if e.GetRuleId() == ruleID {
			return
		}
	}
	t.Fatalf("expected %s to fire standalone", ruleID)
}

// TestClusterMinNodesRuleFires: cluster + node_count=1 trips the min-nodes rule.
func TestClusterMinNodesRuleFires(t *testing.T) {
	assertRuleFires(t, "cockroach_cluster_min_nodes", values(t, `{
		"topology":"cluster","version":"24.2","node_count":1
	}`))
}

// TestRFLeNodesRuleFires: cluster + node_count=3 + replication_factor=5 trips
// the RF <= node_count rule.
func TestRFLeNodesRuleFires(t *testing.T) {
	assertRuleFires(t, "cockroach_rf_le_nodes", values(t, `{
		"topology":"cluster","version":"24.2","node_count":3,
		"cluster":{"replication_factor":5}
	}`))
}
