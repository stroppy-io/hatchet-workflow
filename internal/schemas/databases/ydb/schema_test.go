package ydb

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

// TestSchemaBuilds verifies the descriptor is well-formed (YDBInputSchema panics
// via MustBuild otherwise).
func TestSchemaBuilds(t *testing.T) {
	s := YDBInputSchema()
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

// TestCombinedValid: a combined-mode config (storage + database co-located)
// leaves the split-only gates inactive and validates clean.
func TestCombinedValid(t *testing.T) {
	s := YDBInputSchema()
	v := values(t, `{
		"topology":"combined","storage_node_count":1,
		"database_path":"/Root/testdb","tenant":"/Root/testdb",
		"storage":{"fault_tolerance":"none","pool_media_kind":"ssd","storage_groups":1},
		"protocols":{"grpc_port":2136,"enable_pgwire":false,"mon_port_storage":8765},
		"config":{"memory_hard_limit":"85%","shared_cache_max_percent":30}
	}`)
	for _, e := range s.ValidateStruct(v) {
		if e.GetSeverity().String() == "ERROR" {
			t.Errorf("unexpected error: %s: %s", e.GetField(), e.GetMessage())
		}
	}
}

// TestSplitMirror3DCValid: a split-mode mirror-3-dc config (separate compute
// nodes, three-DC erasure) validates clean. The >=3 storage nodes cross-field
// rule lives at the cluster layer, not here.
func TestSplitMirror3DCValid(t *testing.T) {
	s := YDBInputSchema()
	v := values(t, `{
		"topology":"split","storage_node_count":9,"database_node_count":3,
		"database_path":"/Root/testdb","tenant":"/Root/testdb",
		"storage":{"fault_tolerance":"mirror-3-dc","failure_domain_type":"",
		           "pool_media_kind":"nvme","storage_groups":3,"state_storage_nto_select":9},
		"protocols":{"grpc_port":2136,"enable_pgwire":true,"pgwire_port":5432,
		             "interconnect_port_database":19002,"mon_port_database":8766},
		"config":{"memory_hard_limit":"32GB","query_execution_limit_percent":25,
		          "enforce_user_token_requirement":true,"actor_auto_config":true}
	}`)
	for _, e := range s.ValidateStruct(v) {
		if e.GetSeverity().String() == "ERROR" {
			t.Errorf("unexpected error: %s: %s", e.GetField(), e.GetMessage())
		}
	}
}

// TestMirror3DCMinNodesRuleFires: the DB-internal cross-field rule fires
// standalone (root = the ydb form) — mirror-3-dc with only 1 storage node.
func TestMirror3DCMinNodesRuleFires(t *testing.T) {
	s := YDBInputSchema()
	v := values(t, `{
		"topology":"split","storage_node_count":1,
		"database_path":"/Root/testdb","tenant":"/Root/testdb",
		"storage":{"fault_tolerance":"mirror-3-dc","pool_media_kind":"ssd","storage_groups":1},
		"protocols":{"grpc_port":2136},"config":{"memory_hard_limit":"85%"}
	}`)
	found := false
	for _, e := range s.ValidateStruct(v) {
		if e.GetRuleId() == "ydb_mirror3dc_min_nodes" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ydb_mirror3dc_min_nodes to fire standalone")
	}
}
