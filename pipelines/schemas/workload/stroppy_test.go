package workload

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func segmentValue(name string) map[string]any {
	return map[string]any{
		"name":      name,
		"script":    "tpcc/tx",
		"execution": map[string]any{"limit": map[string]any{"kind": "duration", "duration": "60s"}},
	}
}

func TestStroppy(t *testing.T) {
	minimal := map[string]any{
		"stroppy_version": "5.1.2",
		"protocol":        "pg",
		"segments":        []any{segmentValue("load")},
	}
	full := map[string]any{
		"stroppy_version": "5.1.2-rc.1",
		"protocol":        "ydb_grpcs",
		"segments":        []any{segmentValue("load"), segmentValue("steady")},
		"driver_options": map[string]any{
			"kind":  "ydb",
			"grpcs": true,
			"token": "t1.secret",
		},
	}

	schematest.Run(t, Stroppy(), schematest.Cases{
		Valid: []map[string]any{minimal, full},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{
				"stroppy_version": "v5",
				"protocol":        "pg",
				"segments":        []any{segmentValue("load")},
			}, Code: "PATTERN_MISMATCH", Path: "stroppy_version"},
			{Value: map[string]any{
				"stroppy_version": "5.1.2",
				"protocol":        "oracle",
				"segments":        []any{segmentValue("load")},
			}, Code: "CHOICE_NOT_ALLOWED", Path: "protocol"},
			{Value: map[string]any{
				"stroppy_version": "5.1.2",
				"protocol":        "pg",
				"segments":        []any{},
			}, Code: "MIN_ITEMS_VIOLATED", Path: "segments"},
			{Value: map[string]any{
				"stroppy_version": "5.1.2",
				"protocol":        "pg",
				"segments":        []any{segmentValue("load")},
				"driver_options":  map[string]any{"kind": "mysql"},
			}, Code: "RULE_VIOLATED", Path: ""},
			{Value: map[string]any{
				"stroppy_version": "5.1.2",
				"protocol":        "pg",
				"segments":        []any{segmentValue("load"), segmentValue("load")},
			}, Code: "RULE_VIOLATED", Path: ""},
		},
	})
}
