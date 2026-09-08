package dbparams

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestMariaDB(t *testing.T) {
	schematest.Run(t, MariaDB(), schematest.Cases{
		Valid: []map[string]any{
			{"version": "11.4"},
			{
				"version":      "11.8",
				"replicas":     int64(0),
				"replication":  "galera",
				"galera_nodes": int64(5),
				"maxscale":     true,
				"init_sql":     "SET GLOBAL max_connections = 2000;",
			},
			{"version": "10.11", "replicas": int64(2), "replication": "semi_sync", "proxysql": int64(1)},
		},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"proxysql": int64(5)}, Code: "LTE_VIOLATED", Path: "proxysql"},
			{Value: map[string]any{"galera": true}, Code: "UNKNOWN_FIELD", Path: "galera"},
			// Galera sizes itself with galera_nodes, replicas must stay 0
			{
				Value: map[string]any{"replication": "galera", "replicas": int64(2)},
				Code:  "RULE_VIOLATED", Path: "galera-no-replicas",
			},
			// two routers at once
			{
				Value: map[string]any{"maxscale": true, "proxysql": int64(1)},
				Code:  "RULE_VIOLATED", Path: "one-router",
			},
			// even Galera cluster
			{
				Value: map[string]any{"replication": "galera", "galera_nodes": int64(4)},
				Code:  "CHOICE_NOT_ALLOWED", Path: "galera_nodes",
			},
			{Value: map[string]any{"version": "10.6"}, Code: "CHOICE_NOT_ALLOWED", Path: "version"},
		},
	})
}
