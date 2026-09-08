package dbparams

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestMySQL(t *testing.T) {
	schematest.Run(t, MySQL(), schematest.Cases{
		Valid: []map[string]any{
			{"version": "8.4"},
			{
				"version":        "8.0",
				"replicas":       int64(2),
				"replication":    "group",
				"single_primary": false,
				"proxysql":       int64(2),
				"init_sql":       "SET GLOBAL innodb_flush_log_at_trx_commit = 2;",
			},
			{
				"version":                        "8.4",
				"replicas":                       int64(2),
				"replication":                    "semi_sync",
				"semi_sync_wait_for_slave_count": int64(2),
			},
		},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"replicas": int64(-1)}, Code: "GTE_VIOLATED", Path: "replicas"},
			{Value: map[string]any{"group_replication": true}, Code: "UNKNOWN_FIELD", Path: "group_replication"},
			// Group Replication needs 3..9 members
			{
				Value: map[string]any{"replication": "group", "replicas": int64(1)},
				Code:  "RULE_VIOLATED", Path: "group-members",
			},
			// semi-sync with no replica at all
			{
				Value: map[string]any{"replication": "semi_sync", "replicas": int64(0)},
				Code:  "RULE_VIOLATED", Path: "replication-needs-replica",
			},
			// more acks than replicas
			{
				Value: map[string]any{"replication": "semi_sync", "replicas": int64(1), "semi_sync_wait_for_slave_count": int64(3)},
				Code:  "RULE_VIOLATED", Path: "semi-sync-acks-le-replicas",
			},
			{Value: map[string]any{"version": "5.7"}, Code: "CHOICE_NOT_ALLOWED", Path: "version"},
		},
	})
}
