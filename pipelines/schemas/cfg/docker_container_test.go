package cfg

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestDockerContainer(t *testing.T) {
	schematest.Run(t, DockerContainer(), schematest.Cases{
		Valid: []map[string]any{
			{},
			{
				"restart_policy":      "on-failure",
				"restart_max_retries": int64(5),
				"log_driver":          "json-file",
				"log_max_size":        "100m",
				"log_max_file":        int64(5),
				"ulimit_nofile_soft":  int64(524288),
				"ulimit_nofile_hard":  int64(1048576),
				"ulimit_nproc":        int64(65535),
				"ulimit_memlock":      int64(-1),
				"shm_size_mb":         int64(4096),
				"pids_limit":          int64(65535),
				"limit_cpu":           true,
				"limit_memory":        true,
				"cpus":                8.0,
				"memory_mb":           int64(32768),
				"privileged":          true,
				"network_mode":        "bridge",
				"extra_hosts":         []any{"db-1:10.0.0.11", "db-2:10.0.0.12"},
				"sysctls":             map[string]any{"net.core.somaxconn": "65535"},
				"cap_add":             []any{"IPC_LOCK", "SYS_NICE"},
			},
		},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"log_max_size": "100megs"}, Code: "PATTERN_MISMATCH", Path: "log_max_size"},
			{Value: map[string]any{"network_mode": "overlay"}, Code: "CHOICE_NOT_ALLOWED", Path: "network_mode"},
			{Value: map[string]any{"ulimit_nofile_soft": int64(2048), "ulimit_nofile_hard": int64(1024)}, Code: "RULE_VIOLATED"},
			{Value: map[string]any{"sysctls": map[string]any{"NotASysctl": "1"}}, Code: "RULE_VIOLATED", Path: "sysctls"},
			{Value: map[string]any{"junk": 1}, Code: "UNKNOWN_FIELD", Path: "junk"},
		},
		Render: "conf",
		Contains: []string{
			"--restart ",
			"--ulimit nofile=",
			"--shm-size ",
			"--network ",
		},
	})
}
