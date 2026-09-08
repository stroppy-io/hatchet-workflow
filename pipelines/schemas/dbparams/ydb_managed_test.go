package dbparams

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestYdbManaged(t *testing.T) {
	schematest.Run(t, YdbManaged(), schematest.Cases{
		Valid: []map[string]any{
			{"type": "serverless"},
			{
				"type":                "dedicated",
				"location_id":         "ru-central1",
				"resource_preset_id":  "oltp-c16-m128",
				"scale_policy":        "auto",
				"auto_scale":          map[string]any{"min_size": int64(3), "max_size": int64(12), "cpu_utilization_percent": int64(60)},
				"storage_groups":      int64(8),
				"storage_type":        "ssd",
				"assign_public_ips":   true,
				"deletion_protection": false,
			},
			{
				"type":                  "serverless",
				"throttling_rcu_limit":  int64(1000),
				"provisioned_rcu_limit": int64(5000),
				"storage_size_limit_gb": int64(200),
			},
		},
		Invalid: []schematest.Invalid{
			// range
			{
				Value: map[string]any{"type": "dedicated", "storage_groups": int64(0)},
				Code:  "GTE_VIOLATED", Path: "storage_groups",
			},
			// unknown field: OLTP/OLAP is a per-table choice, not a database knob
			{
				Value: map[string]any{"type": "dedicated", "compute_type": "OLAP"},
				Code:  "UNKNOWN_FIELD", Path: "compute_type",
			},
			// cross-field: inverted autoscaling bounds
			{
				Value: map[string]any{
					"type": "dedicated", "scale_policy": "auto",
					"auto_scale": map[string]any{"min_size": int64(9), "max_size": int64(2), "cpu_utilization_percent": int64(70)},
				},
				Code: "RULE_VIOLATED", Path: "autoscale-min-le-max",
			},
			// nested range
			{
				Value: map[string]any{
					"type": "dedicated", "scale_policy": "auto",
					"auto_scale": map[string]any{"min_size": int64(1), "max_size": int64(2), "cpu_utilization_percent": int64(500)},
				},
				Code: "LTE_VIOLATED", Path: "auto_scale.cpu_utilization_percent",
			},
			{Value: map[string]any{"type": "cluster"}, Code: "CHOICE_NOT_ALLOWED", Path: "type"},
		},
	})
}
