package picodata

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

// TestSchemaBuilds verifies the descriptor is well-formed (PicodataInputSchema
// panics via MustBuild otherwise).
func TestSchemaBuilds(t *testing.T) {
	s := PicodataInputSchema()
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

func assertNoErrors(t *testing.T, st *structpb.Struct) {
	t.Helper()
	for _, e := range PicodataInputSchema().ValidateStruct(st) {
		if e.GetSeverity().String() == "ERROR" {
			t.Errorf("unexpected error: %s: %s", e.GetField(), e.GetMessage())
		}
	}
}

// TestSingleInstanceValid: a single-instance form leaves the cluster-only
// sharding/tiers gates inactive and validates clean.
func TestSingleInstanceValid(t *testing.T) {
	assertNoErrors(t, values(t, `{
		"topology":"single",
		"cluster_name":"stroppy-cluster",
		"instance_count":1,
		"replication_factor":1,
		"config":{"memtx_memory":"2GB","log_level":"info","pg_enabled":true,"pg_port":5432}
	}`))
}

// TestInstancesGeRfRuleFires: the DB-internal cross-field rule fires standalone
// (root = the picodata form) — no cluster/embedding needed.
func TestInstancesGeRfRuleFires(t *testing.T) {
	s := PicodataInputSchema()
	v := values(t, `{
		"topology":"single","cluster_name":"stroppy-cluster",
		"instance_count":1,"replication_factor":3
	}`)
	found := false
	for _, e := range s.ValidateStruct(v) {
		if e.GetRuleId() == "picodata_instances_ge_rf" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected picodata_instances_ge_rf to fire standalone")
	}
}

// TestMultiTierClusterValid: a multi-tier cluster with sharding (no machines /
// provider here — those concerns live at the cluster layer) validates clean.
func TestMultiTierClusterValid(t *testing.T) {
	assertNoErrors(t, values(t, `{
		"topology":"cluster",
		"cluster_name":"stroppy-cluster",
		"instance_count":6,
		"replication_factor":3,
		"sharding":{"enabled":true,"bucket_count":3000,"rebalancer":true},
		"tiers":[
			{"name":"default","replication_factor":3,"can_vote":true,"count":3,"sharded":true},
			{"name":"storage","replication_factor":2,"can_vote":false,"count":3,"sharded":true}
		],
		"config":{
			"memtx_memory":"25%","vinyl_memory":"128MB","net_msg_max":2048,
			"log_level":"info","pg_enabled":true,"pg_port":5432,"pg_ssl":false,
			"http_enabled":true,"http_port":8081,
			"extra_params":[{"key":"memtx.max_tuple_size","value":"2MB"}]
		}
	}`))
}
