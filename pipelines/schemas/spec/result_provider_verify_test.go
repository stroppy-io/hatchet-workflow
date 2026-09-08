package spec

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestResultProviderVerify(t *testing.T) {
	schematest.Run(t, ResultProviderVerify(), schematest.Cases{
		Valid: []map[string]any{
			{"ok": true},
			{
				"ok":         false,
				"account_id": "ajepg0mjt06s0000abcd",
				"scope":      "b1gia87mbaomkfvsleds",
				"permissions": []any{
					map[string]any{"name": "compute.instances.create", "granted": true},
					map[string]any{"name": "vpc.networks.create", "granted": false},
				},
				"error": "permission denied: vpc.networks.create",
			},
		},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{}, Code: "REQUIRED_MISSING", Path: "ok"},
			{Value: map[string]any{"ok": false}, Code: "RULE_VIOLATED", Path: ""},
			{
				Value: map[string]any{"ok": true, "permissions": []any{map[string]any{"granted": true}}},
				Code:  "REQUIRED_MISSING", Path: "permissions[0].name",
			},
			{Value: map[string]any{"ok": true, "reason": "x"}, Code: "UNKNOWN_FIELD", Path: "reason"},
		},
	})
}
