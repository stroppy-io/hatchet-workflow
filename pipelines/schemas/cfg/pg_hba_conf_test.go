package cfg

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestPgHbaConf1(t *testing.T) {
	full := map[string]any{
		"include_cluster_defaults": "yes",
		"rules": []any{
			map[string]any{"type": "local", "database": "all", "user": "postgres", "method": "peer"},
			map[string]any{
				"type": "host", "database": "stroppy", "user": "stroppy",
				"address": "10.0.0.0/8", "method": "scram-sha-256",
			},
			map[string]any{
				"type": "host", "database": "replication", "user": "replicator",
				"address": "10.0.0.0/8", "method": "scram-sha-256",
			},
			map[string]any{
				"type": "hostssl", "database": "all", "user": "admin",
				"address": "0.0.0.0/0", "method": "cert", "options": "clientcert=verify-full",
			},
		},
	}

	schematest.Run(t, PgHbaConf1(), schematest.Cases{
		Valid: []map[string]any{{}, full},
		Invalid: []schematest.Invalid{
			{
				Value: map[string]any{"rules": []any{map[string]any{
					"type": "host", "database": "all", "user": "all", "method": "trust",
				}}},
				Code: "RULE_VIOLATED", Path: "rules[0]",
			},
			{
				Value: map[string]any{"rules": []any{map[string]any{
					"type": "host", "database": "all", "user": "all",
					"address": "10.0.0.0/8", "method": "kerberos",
				}}},
				Code: "CHOICE_NOT_ALLOWED", Path: "rules[0].method",
			},
			{
				Value: map[string]any{"include_cluster_defaults": "maybe"},
				Code:  "CHOICE_NOT_ALLOWED", Path: "include_cluster_defaults",
			},
			{
				Value: map[string]any{"include_cluster_defaults": "no"},
				Code:  "RULE_VIOLATED", Path: "not-empty",
			},
			{Value: map[string]any{"junk": 1}, Code: "UNKNOWN_FIELD", Path: "junk"},
		},
		Render:   "conf",
		Contains: []string{"# TYPE\tDATABASE\tUSER\tADDRESS\tMETHOD", "local\tall\tpostgres\t\tpeer"},
	})

	vals, _, err := PgHbaConf1().Resolve(full)
	if err != nil {
		t.Fatal(err)
	}

	out, err := PgHbaConf1().Render("conf", vals)
	if err != nil {
		t.Fatal(err)
	}

	wantLines(t, out,
		"local\tall\tpostgres\tpeer",
		"host\tstroppy\tstroppy\t10.0.0.0/8\tscram-sha-256",
		"host\treplication\treplicator\t10.0.0.0/8\tscram-sha-256",
		"hostssl\tall\tadmin\t0.0.0.0/0\tcert\tclientcert=verify-full",
	)
}
