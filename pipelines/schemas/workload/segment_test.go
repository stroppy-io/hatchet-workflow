package workload

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestSegment(t *testing.T) {
	minimal := map[string]any{
		"name":   "load",
		"script": "tpcc/tx",
		"execution": map[string]any{
			"limit": map[string]any{"kind": "duration", "duration": "60s"},
		},
	}
	full := map[string]any{
		"name":   "steady-state",
		"script": "tpcc/tx",
		"execution": map[string]any{
			"vus":           int64(64),
			"limit":         map[string]any{"kind": "iterations", "count": int64(100000)},
			"quiet":         true,
			"no_thresholds": false,
			"extra_args":    []any{"--summary-trend-stats", "p(99)"},
		},
		"params": map[string]any{
			"pool_size":     int64(200),
			"scale_factor":  int64(10),
			"steps":         []any{"create_schema", "load_data"},
			"insert_method": "native",
			"bulk_size":     int64(5000),
			"env":           map[string]any{"WAREHOUSES": "10"},
		},
		"files": []any{
			map[string]any{"name": "pg.sql", "kind": "sql", "content": "create table t(id int);"},
			map[string]any{"name": "extra.sql", "kind": "sql", "ref": "artifact/abc"},
		},
		"thresholds": map[string]any{"p99_ms": 25.0, "error_rate": 0.01},
		"warmup":     "30s",
	}

	schematest.Run(t, Segment(), schematest.Cases{
		Valid: []map[string]any{minimal, full},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{
				"name":      "Load",
				"script":    "tpcc/tx",
				"execution": map[string]any{"limit": map[string]any{"kind": "duration", "duration": "60s"}},
			}, Code: "PATTERN_MISMATCH", Path: "name"},
			{Value: map[string]any{
				"name":      "load",
				"script":    "tpcc/tx",
				"execution": map[string]any{"limit": map[string]any{"kind": "forever"}},
			}, Code: "UNKNOWN_VARIANT", Path: "execution.limit"},
			{Value: map[string]any{
				"name":      "load",
				"script":    "tpcc/tx",
				"execution": map[string]any{"vus": int64(0), "limit": map[string]any{"kind": "duration", "duration": "60s"}},
			}, Code: "GTE_VIOLATED", Path: "execution.vus"},
			{Value: map[string]any{
				"name":      "load",
				"script":    "tpcc/tx",
				"execution": map[string]any{"limit": map[string]any{"kind": "duration", "duration": "60s"}},
				"params": map[string]any{
					"steps":    []any{"load_data", "create_schema"},
					"no_steps": []any{"load_data"},
				},
			}, Code: "RULE_VIOLATED", Path: "params"},
			// A nested duration inside the OneOf variant: coerced from its
			// string form, then checked against the bound.
			{Value: map[string]any{
				"name":      "load",
				"script":    "tpcc/tx",
				"execution": map[string]any{"limit": map[string]any{"kind": "duration", "duration": "0s"}},
			}, Code: "GT_VIOLATED", Path: "execution.limit.duration"},
			// Rules and constraints inside a map are real: the rule sees the
			// whole map, a value constraint reports under its key.
			{Value: map[string]any{
				"name":      "load",
				"script":    "tpcc/tx",
				"execution": map[string]any{"limit": map[string]any{"kind": "duration", "duration": "60s"}},
				"params":    map[string]any{"env": map[string]any{"lower-case": "1"}},
			}, Code: "RULE_VIOLATED", Path: "params.env"},
			{Value: map[string]any{
				"name":      "load",
				"script":    "tpcc/tx",
				"execution": map[string]any{"limit": map[string]any{"kind": "duration", "duration": "60s"}},
				"params":    map[string]any{"env": map[string]any{"OK": strings.Repeat("z", 5000)}},
			}, Code: "MAX_LEN_VIOLATED", Path: "params.env.OK"},
			{Value: map[string]any{
				"name":      "load",
				"script":    "tpcc/tx",
				"execution": map[string]any{"limit": map[string]any{"kind": "duration", "duration": "60s"}},
				"junk":      1,
			}, Code: "UNKNOWN_FIELD", Path: "junk"},
		},
	})
}
