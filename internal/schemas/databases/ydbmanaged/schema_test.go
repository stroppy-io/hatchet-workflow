package ydbmanaged

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

// TestSchemaBuilds verifies the descriptor is well-formed (YDBManagedInputSchema
// panics via MustBuild otherwise).
func TestSchemaBuilds(t *testing.T) {
	s := YDBManagedInputSchema()
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
	s := YDBManagedInputSchema()
	for _, e := range s.ValidateStruct(st) {
		if e.GetSeverity().String() == "ERROR" {
			t.Errorf("unexpected error: %s: %s", e.GetField(), e.GetMessage())
		}
	}
}

// TestServerlessValid: a serverless form leaves the dedicated subtree inactive.
func TestServerlessValid(t *testing.T) {
	assertNoErrors(t, values(t, `{
		"type":"serverless",
		"database_path":"/Root/testdb",
		"serverless":{"throttling_rcus":0,"provisioned_rcu":0,"storage_size_limit":0}
	}`))
}

// TestDedicatedFixedScaleValid: a dedicated fixed-scale config; the auto-scale
// fields stay inactive and the autoscale range rule does not fire.
func TestDedicatedFixedScaleValid(t *testing.T) {
	assertNoErrors(t, values(t, `{
		"type":"dedicated","compute_type":"oltp",
		"database_path":"/Root/testdb",
		"dedicated":{
			"resource_preset_id":"medium",
			"scale":{"scale_type":"fixed","node_count":3},
			"storage_groups":2,"storage_type":"ssd"
		}
	}`))
}

// TestDedicatedAutoScaleValid: a dedicated auto-scale config with a valid
// min/max range and CPU target.
func TestDedicatedAutoScaleValid(t *testing.T) {
	assertNoErrors(t, values(t, `{
		"type":"dedicated","compute_type":"olap",
		"database_path":"/Root/testdb",
		"dedicated":{
			"resource_preset_id":"olap-large",
			"scale":{"scale_type":"auto","min_size":2,"max_size":6,"cpu_utilization_percent":70},
			"storage_groups":4,"storage_type":"ssd"
		}
	}`))
}
