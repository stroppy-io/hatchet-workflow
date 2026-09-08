package system

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func script(id string) map[string]any {
	return map[string]any{
		"id":        id,
		"title":     "TPC-C, raw transactions",
		"protocols": []any{"pg", "mysql"},
		"steps": []any{
			map[string]any{"id": "drop_schema", "phase": "bootstrap"},
			map[string]any{"id": "create_schema", "phase": "bootstrap"},
			map[string]any{"id": "load_data", "phase": "bootstrap"},
			map[string]any{"id": "workload", "phase": "workload"},
		},
	}
}

func version(v string, def bool) map[string]any {
	return map[string]any{
		"version":   v,
		"image":     "ghcr.io/stroppy-io/stroppy:" + v,
		"default":   def,
		"protocols": []any{"pg", "mysql", "ydb_grpc", "noop"},
		"scripts":   []any{script("tpcc/tx")},
	}
}

func TestStroppyCatalog(t *testing.T) {
	minimal := map[string]any{"versions": []any{version("5.1.2", true)}}

	full := map[string]any{
		"source": "release",
		"versions": []any{
			version("5.1.2", true),
			map[string]any{
				"version":    "5.0.9",
				"image":      "ghcr.io/stroppy-io/stroppy:5.0.9",
				"deprecated": true,
				"protocols":  []any{"pg"},
				"scripts": []any{
					script("tpcb/procs"),
					map[string]any{
						"id":            "tpch/tx",
						"title":         "TPC-H",
						"description":   "Relational load of eight tables plus the 22 query suite.",
						"steps":         []any{map[string]any{"id": "load_data", "phase": "bootstrap"}},
						"params_schema": "workload.segment@1",
					},
				},
			},
		},
	}

	schematest.Run(t, StroppyCatalog(), schematest.Cases{
		Valid: []map[string]any{minimal, full},
		Invalid: []schematest.Invalid{
			{
				Value: map[string]any{"versions": []any{version("5.1.2", false)}},
				Code:  "RULE_VIOLATED", Path: "",
			},
			{
				Value: map[string]any{"versions": []any{version("5.1.2", true), version("5.0.9", true)}},
				Code:  "RULE_VIOLATED", Path: "",
			},
			{
				Value: map[string]any{"versions": []any{version("v5", true)}},
				Code:  "PATTERN_MISMATCH", Path: "versions[0].version",
			},
			{Value: map[string]any{"versions": []any{
				map[string]any{
					"version": "5.1.2", "image": "x", "default": true,
					"protocols": []any{"oracle"}, "scripts": []any{script("tpcc/tx")},
				},
			}}, Code: "CHOICE_NOT_ALLOWED", Path: "versions[0].protocols[0]"},
			{Value: map[string]any{"versions": []any{}}, Code: "MIN_ITEMS_VIOLATED", Path: "versions"},
		},
	})
}
