package cfg

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestExporterNode(t *testing.T) {
	schematest.Run(t, ExporterNode(), schematest.Cases{
		Valid: []map[string]any{
			{},
			{
				"listen_port":                     int64(9100),
				"listen_address":                  "0.0.0.0",
				"disable_defaults":                true,
				"enable":                          []any{"cpu", "meminfo", "diskstats", "filesystem", "netdev", "loadavg", "pressure", "systemd"},
				"disable":                         []any{"zfs", "btrfs"},
				"filesystem_mount_points_exclude": "^/(dev|proc|sys)($|/)",
				"netdev_device_exclude":           "^(veth.*|lo)$",
				"diskstats_device_exclude":        "^(ram|loop)\\d+$",
				"textfile_directory":              "/var/lib/node_exporter/textfile",
				"log_level":                       "info",
				"log_format":                      "json",
			},
		},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"listen_port": int64(0)}, Code: "GTE_VIOLATED", Path: "listen_port"},
			{Value: map[string]any{"enable": []any{"not_a_collector"}}, Code: "NOT_IN_ALLOWED_SET", Path: "enable[0]"},
			{Value: map[string]any{"enable": []any{"cpu"}, "disable": []any{"cpu"}}, Code: "RULE_VIOLATED"},
			{Value: map[string]any{"log_level": "trace"}, Code: "CHOICE_NOT_ALLOWED", Path: "log_level"},
			{Value: map[string]any{"junk": 1}, Code: "UNKNOWN_FIELD", Path: "junk"},
		},
		Render: "conf",
		Contains: []string{
			"--web.listen-address=",
			"--collector.filesystem.mount-points-exclude=",
			"--log.level=",
		},
	})
}
