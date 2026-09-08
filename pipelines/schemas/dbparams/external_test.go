package dbparams

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestExternal(t *testing.T) {
	schematest.Run(t, External(), schematest.Cases{
		Valid: []map[string]any{
			{"dsn": "postgres://u:p@db.internal:5432/stroppy"},
			{
				"protocol":            "ydb_grpcs",
				"dsn":                 "grpcs://ydb.internal:2135/?database=/Root/stroppy",
				"tls_skip_verify":     false,
				"probe_query":         "SELECT 1;",
				"truncate_before_run": true,
			},
		},
		Invalid: []schematest.Invalid{
			// the DSN is the whole point of this kind
			{Value: map[string]any{}, Code: "REQUIRED_MISSING", Path: "dsn"},
			// unknown field
			{
				Value: map[string]any{"dsn": "postgres://x", "tags": []any{"a"}},
				Code:  "UNKNOWN_FIELD", Path: "tags",
			},
			// closed enum
			{
				Value: map[string]any{"dsn": "postgres://x", "protocol": "mongodb"},
				Code:  "CHOICE_NOT_ALLOWED", Path: "protocol",
			},
			// range
			{Value: map[string]any{"dsn": "x"}, Code: "MIN_LEN_VIOLATED", Path: "dsn"},
			{
				Value: map[string]any{"dsn": "postgres://x", "probe_query": ""},
				Code:  "MIN_LEN_VIOLATED", Path: "probe_query",
			},
		},
	})
}
