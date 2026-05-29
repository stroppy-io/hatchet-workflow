package mariadb

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

// TestSchemaBuilds verifies the descriptor is well-formed (MariaDBInputSchema
// panics via MustBuild otherwise).
func TestSchemaBuilds(t *testing.T) {
	s := MariaDBInputSchema()
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

// TestSingleNodeValid: a single-node form leaves replica/proxy/galera gates inactive.
func TestSingleNodeValid(t *testing.T) {
	s := MariaDBInputSchema()
	for _, e := range s.ValidateStruct(values(t, `{"topology":"single","version":"11.4"}`)) {
		if e.GetSeverity().String() == "ERROR" {
			t.Errorf("unexpected error: %s: %s", e.GetField(), e.GetMessage())
		}
	}
}

// TestGaleraMinNodesRuleFires: the DB-internal quorum rule fires standalone
// (root = the mariadb form) when galera has too few nodes.
func TestGaleraMinNodesRuleFires(t *testing.T) {
	s := MariaDBInputSchema()
	v := values(t, `{"topology":"galera","version":"11.4","replica_count":1}`)
	found := false
	for _, e := range s.ValidateStruct(v) {
		if e.GetRuleId() == "mariadb_galera_min_nodes" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected mariadb_galera_min_nodes to fire standalone")
	}
}

// TestGaleraValid: a logical Galera cluster config (no machines/provider here)
// validates clean — those concerns live at the cluster layer.
func TestGaleraValid(t *testing.T) {
	s := MariaDBInputSchema()
	v := values(t, `{
		"topology":"galera","version":"11.4","replica_count":2,
		"replication":{
			"mode":"galera","gtid_strict_mode":true,
			"binlog_format":"ROW","log_bin":true,
			"galera":{
				"wsrep_cluster_name":"stroppy_galera","wsrep_sst_method":"mariabackup",
				"wsrep_slave_threads":4,"local_address_port":4567,"gcache_size":"1G"
			}
		},
		"tuning":{"innodb_autoinc_lock_mode":2,"innodb_flush_log_at_trx_commit":1,
			"innodb_doublewrite":true,"sync_binlog":1},
		"auth":{"default_authentication_plugin":"mysql_native_password"}
	}`)
	for _, e := range s.ValidateStruct(v) {
		if e.GetSeverity().String() == "ERROR" {
			t.Errorf("unexpected error: %s: %s", e.GetField(), e.GetMessage())
		}
	}
}

// TestSemiSyncReplicasValid: primary + semi-sync replicas with ProxySQL.
func TestSemiSyncReplicasValid(t *testing.T) {
	s := MariaDBInputSchema()
	v := values(t, `{
		"topology":"primary_replicas","version":"10.11","replica_count":2,
		"replication":{"mode":"semi_sync","rpl_semi_sync_master_timeout":10000,
			"rpl_semi_sync_master_wait_no_slave":true},
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
