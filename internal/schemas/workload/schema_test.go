package workload

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

// TestSchemaBuilds verifies the descriptor is well-formed (WorkloadInputSchema
// panics via MustBuild otherwise).
func TestSchemaBuilds(t *testing.T) {
	s := WorkloadInputSchema()
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

// TestDefaultDurationValid: a minimal duration-mode form (the §0.8 defaults)
// validates clean — no errors. The iterations gate is inactive in duration mode.
func TestDefaultDurationValid(t *testing.T) {
	s := WorkloadInputSchema()
	v := values(t, `{
		"stroppy_version":"5.1.1","script":"tpcc/procs",
		"duration":"60s","k6_mode":"duration",
		"vus":1,"scale_factor":1,"pool_size":100,
		"quiet":true,"default_insert_method":"native"
	}`)
	for _, e := range s.ValidateStruct(v) {
		if e.GetSeverity().String() == "ERROR" {
			t.Errorf("unexpected error: %s: %s", e.GetField(), e.GetMessage())
		}
	}
}

// TestIterationsModeValid: an iterations-mode form validates clean.
func TestIterationsModeValid(t *testing.T) {
	s := WorkloadInputSchema()
	v := values(t, `{
		"stroppy_version":"5.1.1","script":"tpcb/tx",
		"k6_mode":"iterations","iterations":10,
		"vus":4,"scale_factor":2,"pool_size":50
	}`)
	for _, e := range s.ValidateStruct(v) {
		if e.GetSeverity().String() == "ERROR" {
			t.Errorf("unexpected error: %s: %s", e.GetField(), e.GetMessage())
		}
	}
}

// TestBadDurationFires: a duration without an s/m/h suffix is rejected by the
// field Pattern (standalone, root = the workload form).
func TestBadDurationFires(t *testing.T) {
	s := WorkloadInputSchema()
	v := values(t, `{
		"stroppy_version":"5.1.1","script":"tpcc/procs",
		"duration":"60","k6_mode":"duration","vus":1
	}`)
	found := false
	for _, e := range s.ValidateStruct(v) {
		if e.GetField() == "duration" && e.GetSeverity().String() == "ERROR" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a duration pattern error for %q", "60")
	}
}
