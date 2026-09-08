package cfg

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestHaproxy(t *testing.T) {
	schematest.Run(t, Haproxy2(), schematest.Cases{
		Valid: []map[string]any{
			{},
			{
				"maxconn":         int64(50000),
				"log":             "/dev/log local0 info",
				"nbthread":        int64(8),
				"stats_socket":    "/var/run/haproxy.sock",
				"timeout_connect": int64(4),
				"timeout_client":  int64(3600),
				"timeout_server":  int64(3600),
				"timeout_check":   int64(5),
				"retries":         int64(2),
				"option_tcplog":   true,
				"stats_enabled":   true,
				"stats_port":      int64(8404),
				"stats_uri":       "/stats",
				"stats_auth":      "admin:secret",
				"listeners": []any{
					map[string]any{
						"name":      "pg_rw",
						"bind_port": int64(5000),
						"balance":   "first",
						"check": map[string]any{
							"kind": "httpchk",
							"uri":  "/primary",
							"port": int64(8008),
						},
						"servers": []any{
							map[string]any{"name": "pg-1", "address": "10.0.0.11", "port": int64(5432)},
							map[string]any{"name": "pg-2", "address": "10.0.0.12", "port": int64(5432)},
							map[string]any{"name": "pg-3", "address": "10.0.0.13", "port": int64(5432)},
						},
					},
					map[string]any{
						"name":      "pg_ro",
						"bind_port": int64(5001),
						"balance":   "roundrobin",
						"check": map[string]any{
							"kind": "httpchk",
							"uri":  "/replica",
							"port": int64(8008),
						},
						"servers": []any{
							map[string]any{"name": "pg-2", "address": "10.0.0.12", "port": int64(5432)},
							map[string]any{"name": "pg-3", "address": "10.0.0.13", "port": int64(5432)},
							map[string]any{"name": "pg-1", "address": "10.0.0.11", "port": int64(5432), "backup": true},
						},
					},
				},
			},
		},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"listeners": []any{map[string]any{
				"bind_port": int64(5000),
				"check":     map[string]any{},
				"servers":   []any{map[string]any{"name": "a", "address": "1.2.3.4", "port": int64(5432)}},
			}}}, Code: "REQUIRED_MISSING", Path: "listeners[0].name"},
			{Value: map[string]any{"listeners": []any{map[string]any{
				"name": "rw", "bind_port": int64(5000), "balance": "sticky",
				"check":   map[string]any{},
				"servers": []any{map[string]any{"name": "a", "address": "1.2.3.4", "port": int64(5432)}},
			}}}, Code: "CHOICE_NOT_ALLOWED", Path: "listeners[0].balance"},
			{Value: map[string]any{"listeners": []any{map[string]any{
				"name": "rw", "bind_port": int64(5000),
				"check":   map[string]any{},
				"servers": []any{},
			}}}, Code: "MIN_ITEMS_VIOLATED", Path: "listeners[0].servers"},
			{Value: map[string]any{"maxconn": int64(1)}, Code: "GTE_VIOLATED", Path: "maxconn"},
			{Value: map[string]any{"junk": 1}, Code: "UNKNOWN_FIELD", Path: "junk"},
		},
		Render: "conf",
		Contains: []string{
			"global",
			"defaults",
			"    mode tcp",
			"listen stats",
		},
	})
}
