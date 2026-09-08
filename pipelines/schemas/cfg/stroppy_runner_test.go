package cfg

import (
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestStroppyRunner(t *testing.T) {
	schematest.Run(t, StroppyRunner(), schematest.Cases{
		Valid: []map[string]any{
			{
				"driver": map[string]any{"driverType": "noop"},
			},
			{
				"log_level":                   "debug",
				"log_format":                  "json",
				"otlp_protocol":               "grpc",
				"otlp_endpoint":               "127.0.0.1:4317",
				"otlp_insecure":               true,
				"otlp_service_name":           "stroppy",
				"otlp_metric_prefix":          "k6_",
				"otlp_headers":                map[string]any{"X-Run-Id": "run-42"},
				"metrics_flush_interval":      5 * time.Second,
				"threads":                     int64(16),
				"pool_max_conns":              int64(400),
				"pool_min_conns":              int64(400),
				"pool_conn_lifetime":          2 * time.Hour,
				"pool_conn_idle_time":         10 * time.Minute,
				"insert_progress_interval":    30 * time.Second,
				"insert_progress_stall_after": 2 * time.Minute,
				"insert_progress_mode":        "both",
				"error_mode":                  "throw",
				"bulk_size":                   int64(5000),
				"default_insert_method":       "native",
				"driver_url":                  "grpcs://10.0.0.10:2135/local",
				"driver": map[string]any{
					"driverType":               "ydb",
					"grpcs":                    true,
					"ca_cert_file":             "/etc/ydb/certs/ca.pem",
					"auth_user":                "root",
					"auth_password":            "secret",
					"tls_insecure_skip_verify": false,
				},
			},
		},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{}, Code: "REQUIRED_MISSING", Path: "driver"},
			{Value: map[string]any{"driver": map[string]any{"driverType": "cassandra"}}, Code: "UNKNOWN_VARIANT", Path: "driver"},
			{Value: map[string]any{
				"driver":         map[string]any{"driverType": "noop"},
				"pool_max_conns": int64(10), "pool_min_conns": int64(20),
			}, Code: "RULE_VIOLATED"},
			{
				Value: map[string]any{"driver": map[string]any{"driverType": "noop"}, "log_level": "trace"},
				Code:  "CHOICE_NOT_ALLOWED", Path: "log_level",
			},
			{
				Value: map[string]any{"driver": map[string]any{"driverType": "noop"}, "junk": 1},
				Code:  "UNKNOWN_FIELD", Path: "junk",
			},
		},
		Render: "conf",
		Contains: []string{
			"LOG_LEVEL=",
			"K6_OTEL_SERVICE_NAME=stroppy",
			`STROPPY_DRIVER_0={"driverType":`,
		},
	})
}
