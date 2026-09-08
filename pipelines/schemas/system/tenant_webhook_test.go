package system

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestTenantWebhook(t *testing.T) {
	minimal := map[string]any{
		"url":     "https://ci.example.com/hooks/stroppy",
		"events":  []any{"run.finished"},
		"secret":  "0123456789abcdef0123",
		"enabled": true,
	}
	full := map[string]any{
		"url":         "https://ci.example.com/hooks/stroppy?team=db",
		"events":      []any{"run.started", "run.finished", "run.failed", "suite.finished"},
		"secret":      "whsec_0123456789abcdef",
		"enabled":     false,
		"description": "Posts finished runs into the release dashboard.",
	}

	schematest.Run(t, TenantWebhook(), schematest.Cases{
		Valid: []map[string]any{minimal, full},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{
				"url": "http://ci.example.com/hook", "events": []any{"run.finished"}, "secret": "0123456789abcdef",
			}, Code: "PATTERN_MISMATCH", Path: "url"},
			{Value: map[string]any{
				"url": "https://ci.example.com/hook", "events": []any{}, "secret": "0123456789abcdef",
			}, Code: "MIN_ITEMS_VIOLATED", Path: "events"},
			{Value: map[string]any{
				"url": "https://ci.example.com/hook", "events": []any{"run.exploded"}, "secret": "0123456789abcdef",
			}, Code: "CHOICE_NOT_ALLOWED", Path: "events[0]"},
			{Value: map[string]any{
				"url": "https://ci.example.com/hook", "events": []any{"run.finished"}, "secret": "short",
			}, Code: "MIN_LEN_VIOLATED", Path: "secret"},
			{Value: map[string]any{
				"url":    "https://ci.example.com/hook",
				"events": []any{"run.finished", "run.finished"},
				"secret": "0123456789abcdef",
			}, Code: "NOT_UNIQUE", Path: "events[1]"},
		},
	})
}
