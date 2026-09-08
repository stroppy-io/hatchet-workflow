package activities

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func segment() spec.Segment {
	return spec.Segment{
		Name: "load", Script: "tpcc/tx",
		Execution: spec.Execution{VUs: 8, Limit: spec.Limit{Kind: "duration", Duration: spec.Duration(90 * time.Second)}, Quiet: true, NoThresholds: true, ExtraArgs: []string{"--tag", "x=y"}},
		Params:    spec.SegmentParams{PoolSize: 64, ScaleFactor: 10, Steps: []string{"create_schema", "load_data"}, InsertMethod: "native", BulkSize: 2500, Env: map[string]string{"warehouses": "20"}},
	}
}

func TestK6Args(t *testing.T) {
	got := strings.Join(K6Args(segment()), " ")
	want := "--vus 8 --duration 1m30s --quiet --no-thresholds --summary-export /workspace/summary.json --tag x=y"
	if got != want {
		t.Errorf("K6Args = %q, want %q", got, want)
	}
	iter := segment()
	iter.Execution.Limit = spec.Limit{Kind: "iterations", Iterations: 1000}
	if got := strings.Join(K6Args(iter), " "); !strings.Contains(got, "--iterations 1000") || strings.Contains(got, "--duration") {
		t.Errorf("iterations: %q", got)
	}
}

func TestStroppyConfig(t *testing.T) {
	req := RunSegmentRequest{
		RunID: "r1", Segment: segment(), Image: "ghcr.io/stroppy-io/stroppy:5.1.2",
		Env: map[string]string{
			EnvDriverURL: "postgres://u:p@10.0.0.5:5432/db", EnvDriverType: "postgres", EnvDriverInsertMethod: "plain_bulk",
			EnvOTLPHeaders: "Authorization=Bearer x", "extra_flag": "1",
		},
		OTLPEndpoint: "https://otel.stroppy.io",
		Labels:       map[string]string{"stroppy_run_id": "r1"},
	}
	cfg := StroppyConfig(req)
	drv := cfg["drivers"].(map[string]any)["0"].(map[string]any)
	if drv["url"] != "postgres://u:p@10.0.0.5:5432/db" || drv["driverType"] != "postgres" {
		t.Errorf("driver = %v", drv)
	}
	if drv["defaultInsertMethod"] != "native" { // the segment overrides the workload default
		t.Errorf("insert method = %v", drv["defaultInsertMethod"])
	}
	if drv["bulkSize"] != 2500 {
		t.Errorf("bulkSize = %v", drv["bulkSize"])
	}
	env := cfg["env"].(map[string]string)
	if env["POOL_SIZE"] != "64" || env["SCALE_FACTOR"] != "10" || env["WAREHOUSES"] != "20" || env["EXTRA_FLAG"] != "1" {
		t.Errorf("env = %v", env)
	}
	if _, leaked := env[EnvOTLPHeaders]; leaked {
		t.Errorf("otlp headers leaked into script env")
	}
	if !reflect.DeepEqual(cfg["steps"], []string{"create_schema", "load_data"}) {
		t.Errorf("steps = %v", cfg["steps"])
	}
	exp := cfg["global"].(map[string]any)["exporter"].(map[string]any)["otlpExport"].(map[string]any)
	if exp["otlpHttpEndpoint"] != "otel.stroppy.io" || exp["otlpHeaders"] != "Authorization=Bearer x" || exp["otlpEndpointInsecure"] == true {
		t.Errorf("otlp = %v", exp)
	}
	if _, err := json.Marshal(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestParseK6Summary(t *testing.T) {
	nested := `{"metrics":{"iterations":{"type":"counter","values":{"count":1200,"rate":40.5}},
	  "iteration_duration":{"type":"trend","values":{"avg":12.1,"min":3,"med":10,"max":90,"p(90)":20,"p(95)":25.5,"p(99)":40}},
	  "checks":{"values":{"rate":1,"passes":10,"fails":0}}}}`
	m, err := parseK6Summary([]byte(nested))
	if err != nil {
		t.Fatal(err)
	}
	if m["iterations_rate"].Value != 40.5 || m["iteration_duration_p95"].Value != 25.5 || m["checks_fails"].Value != 0 {
		t.Errorf("parsed = %v", m)
	}
	flat := `{"metrics":{"iterations":{"count":5,"rate":1.5}}}`
	m, err = parseK6Summary([]byte(flat))
	if err != nil || m["iterations_count"].Value != 5 {
		t.Errorf("flat: %v %v", m, err)
	}
	h := Headline([]spec.SegmentResult{{Name: "boot"}, {Name: "load", Metrics: map[string]spec.MetricValue{
		"iterations_rate": {Value: 40.5}, "iteration_duration_med": {Value: 10}, "iteration_duration_p95": {Value: 25.5}, "iteration_duration_p99": {Value: 40},
	}}})
	if h.TPS != 40.5 || h.LatencyP99Ms != 40 {
		t.Errorf("headline = %+v", h)
	}
	merged := MergeMetrics([]spec.SegmentResult{{Name: "load", Metrics: map[string]spec.MetricValue{"iterations_rate": {Value: 1}}}})
	if _, ok := merged["load.iterations_rate"]; !ok {
		t.Errorf("merged = %v", merged)
	}
}
