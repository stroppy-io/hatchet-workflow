package mysql

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

// TestSchemaBuilds verifies the descriptor is well-formed (MySQLInputSchema
// panics via MustBuild otherwise).
func TestSchemaBuilds(t *testing.T) {
	s := MySQLInputSchema()
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

// TestSingleNodeValid: a single-node form leaves replica/proxy/GR gates inactive.
func TestSingleNodeValid(t *testing.T) {
	s := MySQLInputSchema()
	for _, e := range s.ValidateStruct(values(t, `{"topology":"single","version":"8.4"}`)) {
		if e.GetSeverity().String() == "ERROR" {
			t.Errorf("unexpected error: %s: %s", e.GetField(), e.GetMessage())
		}
	}
}

// TestGRMinNodesRuleFires: the DB-internal quorum rule fires standalone
// (root = the mysql form) when group_replication has too few members.
func TestGRMinNodesRuleFires(t *testing.T) {
	s := MySQLInputSchema()
	v := values(t, `{"topology":"group_replication","version":"8.4","replica_count":1}`)
	found := false
	for _, e := range s.ValidateStruct(v) {
		if e.GetRuleId() == "mysql_gr_min_nodes" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected mysql_gr_min_nodes to fire standalone")
	}
}

// TestGroupReplicationValid: a logical InnoDB Group Replication config (no
// machines/provider here) validates clean — those concerns live at the cluster layer.
func TestGroupReplicationValid(t *testing.T) {
	s := MySQLInputSchema()
	v := values(t, `{
		"topology":"group_replication","version":"8.4","replica_count":2,
		"replication":{
			"mode":"group_replication","gtid_mode":"ON","enforce_gtid_consistency":true,
			"binlog_format":"ROW","log_bin":true,"log_replica_updates":true,
			"group_replication":{
				"group_name":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				"group_mode":"single_primary","consistency":"BEFORE_ON_PRIMARY_FAILOVER",
				"local_address_port":33061
			}
		},
		"tuning":{"innodb_autoinc_lock_mode":2,"innodb_flush_log_at_trx_commit":1,"sync_binlog":1},
		"auth":{"default_authentication_plugin":"caching_sha2_password"}
	}`)
	for _, e := range s.ValidateStruct(v) {
		if e.GetSeverity().String() == "ERROR" {
			t.Errorf("unexpected error: %s: %s", e.GetField(), e.GetMessage())
		}
	}
}

// TestSemiSyncReplicasValid: primary + semi-sync replicas with ProxySQL.
func TestSemiSyncReplicasValid(t *testing.T) {
	s := MySQLInputSchema()
	v := values(t, `{
		"topology":"primary_replicas","version":"8.0","replica_count":2,
		"replication":{"mode":"semi_sync","semi_sync_timeout":10000,"semi_sync_wait_for_replica_count":1},
		"proxy":{"use_proxy":true,"proxysql":{"max_connections":2048,
			"hostgroups":[{"id":10,"role":"writer"},{"id":20,"role":"reader"}]}},
		"routing":{"read_write_split":true,"workload_connect_target":"proxy"}
	}`)
	for _, e := range s.ValidateStruct(v) {
		if e.GetSeverity().String() == "ERROR" {
			t.Errorf("unexpected error: %s: %s", e.GetField(), e.GetMessage())
		}
	}
}
