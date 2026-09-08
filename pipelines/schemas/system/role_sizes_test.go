package system

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestRoleSizes(t *testing.T) {
	minimal := map[string]any{
		"roles": map[string]any{"db": map[string]any{"size": "M"}},
	}
	full := map[string]any{
		"roles": map[string]any{
			"db":          map[string]any{"size": "L", "disk": map[string]any{"type": "network-ssd-io-m3", "gb": int64(1000)}},
			"db-replica":  map[string]any{"size": "L"},
			"proxy":       map[string]any{"size": "S"},
			"runner":      map[string]any{"size": "M"},
			"coordinator": map[string]any{"size": "XS"},
		},
	}

	schematest.Run(t, RoleSizes(), schematest.Cases{
		Valid: []map[string]any{minimal, full},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{}, Code: "REQUIRED_MISSING", Path: "roles"},
			{
				Value: map[string]any{"roles": map[string]any{"db": map[string]any{}}},
				Code:  "REQUIRED_MISSING", Path: "roles.db.size",
			},
			{
				Value: map[string]any{"roles": map[string]any{"db": map[string]any{"size": "huge"}}},
				Code:  "CHOICE_NOT_ALLOWED", Path: "roles.db.size",
			},
			{Value: map[string]any{"roles": map[string]any{
				"db": map[string]any{"size": "M", "disk": map[string]any{"gb": int64(1)}},
			}}, Code: "GTE_VIOLATED", Path: "roles.db.disk.gb"},
			// A Pattern nested inside the map value schema.
			{Value: map[string]any{"roles": map[string]any{
				"db": map[string]any{"size": "M", "disk": map[string]any{"type": "Network SSD"}},
			}}, Code: "PATTERN_MISMATCH", Path: "roles.db.disk.type"},
			{Value: map[string]any{"roles": map[string]any{
				"db": map[string]any{"size": "M", "tier": "hot"},
			}}, Code: "UNKNOWN_FIELD", Path: "roles.db.tier"},
		},
	})
}
