package postgres

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

// TestSchemaBuilds verifies the descriptor is well-formed (PostgresInputSchema
// panics via MustBuild otherwise).
func TestSchemaBuilds(t *testing.T) {
	s := PostgresInputSchema()
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

// TestSingleNodeValid: a single-node form leaves replica/HA gates inactive.
func TestSingleNodeValid(t *testing.T) {
	s := PostgresInputSchema()
	errs := s.ValidateStruct(values(t, `{"topology":"single","pg_major":16}`))
	for _, e := range errs {
		if e.GetSeverity().String() == "ERROR" {
			t.Errorf("unexpected error: %s: %s", e.GetField(), e.GetMessage())
		}
	}
}

// TestSyncCountRuleFires: the DB-internal cross-field rule fires standalone
// (root = the postgres form) — no cluster/embedding needed.
func TestSyncCountRuleFires(t *testing.T) {
	s := PostgresInputSchema()
	v := values(t, `{
		"topology":"patroni_ha","pg_major":16,"replica_count":1,
		"ha":{"dcs":{"type":"etcd","cluster_size":3},
		      "patroni":{"synchronous_mode":"on","synchronous_node_count":3}}
	}`)
	found := false
	for _, e := range s.ValidateStruct(v) {
		if e.GetRuleId() == "sync_count_le_replicas" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected sync_count_le_replicas to fire standalone")
	}
}

// TestPatroniHAValid: a logical patroni_ha config (no machines/provider here)
// validates clean — those concerns live at the cluster layer.
func TestPatroniHAValid(t *testing.T) {
	s := PostgresInputSchema()
	v := values(t, `{
		"topology":"patroni_ha","pg_major":16,"replica_count":2,
		"ha":{"dcs":{"type":"etcd","cluster_size":3},
		      "patroni":{"synchronous_mode":"on","synchronous_node_count":2}},
		"replication":{"mode":"sync","synchronous_commit":"on","max_wal_senders":10},
		"auth":{"auth_method":"scram-sha-256"}
	}`)
	for _, e := range s.ValidateStruct(v) {
		if e.GetSeverity().String() == "ERROR" {
			t.Errorf("unexpected error: %s: %s", e.GetField(), e.GetMessage())
		}
	}
}
