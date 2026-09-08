package spec

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestSuite(t *testing.T) {
	cell := func(id string) map[string]any {
		return map[string]any{"id": id, "run_spec": baseRun()}
	}
	minimal := map[string]any{
		"suite_run_id": runID,
		"tenant":       "acme",
		"cells":        []any{cell("pg-17-m")},
	}
	full := map[string]any{
		"suite_run_id": runID,
		"tenant":       "acme",
		"cells":        []any{cell("pg-17-m"), cell("pg-17-l")},
		"concurrency":  int64(2),
		"defaults": map[string]any{
			"continue_on_failure": false,
			"labels":              map[string]any{"matrix": "size"},
		},
	}

	schematest.Run(t, Suite(), schematest.Cases{
		Valid: []map[string]any{minimal, full},
		Invalid: []schematest.Invalid{
			{
				Value: map[string]any{"suite_run_id": runID, "tenant": "acme", "cells": []any{}},
				Code:  "MIN_ITEMS_VIOLATED", Path: "cells",
			},
			{Value: map[string]any{
				"suite_run_id": runID, "tenant": "acme",
				"cells": []any{cell("a"), cell("a")},
			}, Code: "RULE_VIOLATED", Path: ""},
			{Value: map[string]any{
				"suite_run_id": runID, "tenant": "acme",
				"cells": []any{cell("a")}, "concurrency": int64(0),
			}, Code: "GTE_VIOLATED", Path: "concurrency"},
			{Value: map[string]any{
				"suite_run_id": runID, "tenant": "acme",
				"cells": []any{map[string]any{"id": "a"}},
			}, Code: "REQUIRED_MISSING", Path: "cells[0].run_spec"},
		},
	})
}
