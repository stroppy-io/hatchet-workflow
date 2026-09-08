package system

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestLimits(t *testing.T) {
	schematest.Run(t, Limits(), schematest.Cases{
		Valid: []map[string]any{
			{},
			{
				"max_concurrent_runs":    int64(10),
				"max_machines_per_run":   int64(16),
				"max_size":               "XL",
				"max_keep":               "72h",
				"run_retention_max_days": int64(365),
			},
		},
		Invalid: []schematest.Invalid{
			{
				Value: map[string]any{"max_concurrent_runs": int64(0)},
				Code:  "GTE_VIOLATED", Path: "max_concurrent_runs",
			},
			{
				Value: map[string]any{"max_machines_per_run": int64(1000)},
				Code:  "LTE_VIOLATED", Path: "max_machines_per_run",
			},
			{Value: map[string]any{"max_size": "XXL"}, Code: "CHOICE_NOT_ALLOWED", Path: "max_size"},
			{Value: map[string]any{"max_keep": "9000h"}, Code: "LTE_VIOLATED", Path: "max_keep"},
		},
	})
}
