package dbparams

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestPgNoop(t *testing.T) {
	schematest.Run(t, PgNoop(), schematest.Cases{
		Valid: []map[string]any{
			{},
			{"version": "0.1.2", "workers": int64(64), "latency_ms": int64(5), "error_rate": 0.0, "port": int64(6432)},
		},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"error_rate": 1.5}, Code: "LTE_VIOLATED", Path: "error_rate"},
			{Value: map[string]any{"workers": int64(-1)}, Code: "GTE_VIOLATED", Path: "workers"},
			{Value: map[string]any{"options": map[string]any{}}, Code: "UNKNOWN_FIELD", Path: "options"},
			{Value: map[string]any{"port": int64(0)}, Code: "GTE_VIOLATED", Path: "port"},
			{Value: map[string]any{"version": "0.9"}, Code: "CHOICE_NOT_ALLOWED", Path: "version"},
		},
	})
}
