package cfg

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestExporterPostgres1(t *testing.T) {
	full := map[string]any{
		"data_source_name":          "postgresql://exporter:pw@10.0.0.11:5432/postgres?sslmode=disable",
		"web_listen_port":           int64(9188),
		"web_telemetry_path":        "/metrics",
		"log_level":                 "debug",
		"log_format":                "json",
		"disable_default_metrics":   "off",
		"metric_prefix":             "pg",
		"collection_timeout":        "30s",
		"auto_discover_databases":   "on",
		"exclude_databases":         "template0,template1",
		"include_databases":         "stroppy",
		"collector_stat_statements": "on",
		"collector_locks":           "off",
		"collector_wal":             "on",
	}

	schematest.Run(t, ExporterPostgres1(), schematest.Cases{
		Valid: []map[string]any{{}, full},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"web_listen_port": int64(0)}, Code: "GTE_VIOLATED", Path: "web_listen_port"},
			{Value: map[string]any{"log_level": "trace"}, Code: "CHOICE_NOT_ALLOWED", Path: "log_level"},
			{Value: map[string]any{"collection_timeout": "forever"}, Code: "PATTERN_MISMATCH", Path: "collection_timeout"},
			{
				Value: map[string]any{"include_databases": "stroppy"},
				Code:  "RULE_VIOLATED", Path: "include-needs-discovery",
			},
			{
				Value: map[string]any{"disable_default_metrics": "on"},
				Code:  "RULE_VIOLATED", Path: "stat-statements-needs-defaults",
			},
			{Value: map[string]any{"junk": 1}, Code: "UNKNOWN_FIELD", Path: "junk"},
		},
		Render: "conf",
		Contains: []string{
			"PG_EXPORTER_WEB_TELEMETRY_PATH=/metrics",
			"PG_EXPORTER_METRIC_PREFIX=pg",
			"PG_EXPORTER_COLLECTION_TIMEOUT=",
			"POSTGRES_EXPORTER_OPTS=--web.listen-address=:",
			"--collector.stat_statements",
			"--no-collector.postmaster",
		},
	})

	out := renderDefaults(t, ExporterPostgres1())
	wantLines(t, out, "PG_EXPORTER_COLLECTION_TIMEOUT=1m")

	for _, want := range []string{
		"--web.listen-address=:9187",
		"--log.level=info",
		"--log.format=logfmt",
		"--collector.database ",
		"--no-collector.database_wraparound",
		"--collector.stat_statements",
		"--no-collector.buffercache_summary",
		"--exclude-databases=template0,template1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered env lacks %q:\n%s", want, out)
		}
	}

	if strings.Contains(out, "DATA_SOURCE_NAME=") {
		t.Errorf("DATA_SOURCE_NAME must stay absent until the server fills it:\n%s", out)
	}

	vals, _, err := ExporterPostgres1().Resolve(full)
	if err != nil {
		t.Fatal(err)
	}

	got, err := ExporterPostgres1().Render("conf", vals)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"DATA_SOURCE_NAME=postgresql://exporter:pw@10.0.0.11:5432/postgres?sslmode=disable",
		"--web.listen-address=:9188",
		"--auto-discover-databases",
		"--include-databases=stroppy",
		"--no-collector.locks",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered env lacks %q:\n%s", want, got)
		}
	}
}
