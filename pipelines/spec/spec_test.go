package spec

import (
	"encoding/json"
	"testing"
	"time"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas"
)

// bake marshals v to JSON, decodes it into the generic form and bakes it
// through the named product schema — the round trip a server → pipeline
// hand-off makes. Any drift between the Go types and the schema shows up
// here as a validation error.
func bake(t *testing.T, id string, v any) {
	t.Helper()
	s, ok := schemas.ByID()[id]
	if !ok {
		t.Fatalf("schema %s not registered", id)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatal(err)
	}
	eng, err := schemapb.Compile(s)
	if err != nil {
		t.Fatal(err)
	}
	_, res, err := eng.Bake(generic)
	if err != nil {
		t.Fatalf("%s: bake: %v", id, err)
	}
	if res.Blocking() {
		for _, e := range res.GetErrors() {
			t.Errorf("%s: %s: %s %s", id, e.GetPath(), e.GetCode(), e.GetMessage())
		}
		t.Fatalf("%s: value does not fit the schema:\n%s", id, raw)
	}
}

func sampleRun() Run {
	return Run{
		RunID:  "8f1c3f2a-0000-4000-8000-000000000001",
		Tenant: "acme",
		Provider: Provider{
			Kind:               ProviderYandex,
			Settings:           json.RawMessage(`{"folder_id":"b1gia87mbaomkfvsleds"}`),
			CredentialsSecret:  "yc-sa-key",
			ProviderConfigName: "t-acme",
		},
		Network: Network{CIDR: "10.130.0.0/24", AllowPublicIPs: true, Ingress: []Ingress{{Port: 22, Proto: "tcp", CIDR: "0.0.0.0/0"}}},
		Machines: []Machine{
			{
				Name: "db-1", Role: "db", CPU: 8, MemoryGB: 32, Image: "ubuntu-2404-lts", Location: "ru-central1-d", InstanceType: "standard-v3",
				Disks: []Disk{{Name: "data", GB: 200, Type: "network-ssd", Mount: "/data"}}, Labels: map[string]string{"tier": "db"},
			},
			{Name: "runner-1", Role: "runner", CPU: 4, MemoryGB: 8, Image: "ubuntu-2404-lts", Location: "ru-central1-d", InstanceType: "standard-v3"},
		},
		Containers: []Container{{
			Name: "postgres", Role: "db", Machine: "db-1", Image: "docker.io/library/postgres:17",
			Env:         map[string]string{"POSTGRES_PASSWORD": "x"},
			Ports:       []Port{{Container: 5432, Host: 5432}},
			Files:       []File{{Path: "/etc/postgresql/postgresql.conf", Content: "shared_buffers = 8GB\n", Mode: "0644"}},
			Healthcheck: &Healthcheck{Cmd: []string{"pg_isready"}, Interval: Duration(10 * time.Second), Retries: 30},
			Restart:     "unless-stopped", Ulimits: map[string]int64{"nofile": 65536},
		}},
		HostPrep: []HostPrep{{Role: "db", Kind: HostPrepSysctl, Content: "vm.swappiness=1\n"}},
		Scrapes:  []Scrape{{Role: "db", URL: "http://127.0.0.1:9187/metrics", Job: "postgres"}},
		Flows:    []Flow{{FromRole: "runner", ToRole: "db", Protocol: "tcp", Port: 5432, Label: "postgres"}},
		Workload: Workload{
			RunnerRole: "runner", StroppyImage: "ghcr.io/stroppy-io/stroppy:5.1.2",
			Segments: []json.RawMessage{json.RawMessage(`{"name":"load","script":"tpcc/tx","execution":{"vus":1,"limit":{"kind":"duration","duration":"30s"}}}`)},
			Env:      map[string]string{"STROPPY_URL": "postgres://postgres:x@10.130.0.10:5432/postgres"},
		},
		Observability:      Observability{OTLPEndpoint: "http://otel.stroppy.io", Labels: map[string]string{"stroppy_run_id": "x"}},
		Keep:               Duration(time.Hour),
		ResultExpectations: []string{"tps"},
	}
}

func TestRunFitsSchema(t *testing.T) { bake(t, "spec.run@1", sampleRun()) }

func TestResultFitsSchema(t *testing.T) {
	bake(t, "spec.result.run@1", Result{
		Metrics: map[string]MetricValue{"tps": {Value: 1234.5, Unit: "tx/s", Min: 1000, Max: 1300, Avg: 1200}},
		Segments: []SegmentResult{{
			Name: "load", Status: SegmentCompleted, StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC(),
			Metrics: map[string]MetricValue{"tps": {Value: 1234.5}},
		}},
		Artifacts: []string{"artifact/stroppy-raw"},
		Summary:   Summary{TPS: 1234.5, LatencyP99Ms: 12.3, Errors: 0, Duration: Duration(30 * time.Second)},
	})
}

func TestSuiteFitsSchema(t *testing.T) {
	bake(t, "spec.suite@1", Suite{
		SuiteRunID: "8f1c3f2a-0000-4000-8000-000000000002", Tenant: "acme",
		Cells:       []SuiteCell{{ID: "pg17-m", RunSpec: sampleRun()}},
		Concurrency: 2, Defaults: SuiteDefaults{ContinueOnFailure: true, Labels: map[string]string{"suite": "nightly"}},
	})
}

func TestServicePipelinesFitSchema(t *testing.T) {
	bake(t, "spec.provider_verify@1", ProviderVerify{Provider: ProviderAWS, Settings: json.RawMessage(`{"region":"eu-central-1"}`), CredentialsSecret: "aws-keys", DryRun: true})
	bake(t, "spec.result.provider_verify@1", ProviderVerifyResult{OK: true, AccountID: "123", Scope: "folder", Permissions: []Permission{{Name: "compute.instances.create", Granted: true}}})
	bake(t, "spec.quotas@1", Quotas{Provider: ProviderYandex, Settings: json.RawMessage(`{"folder_id":"b1gia87mbaomkfvsleds"}`), CredentialsSecret: "yc-sa-key", Location: "ru-central1-d"})
	bake(t, "spec.result.quotas@1", QuotasResult{ObservedAt: time.Now().UTC(), Quotas: []Quota{{Name: "compute.instanceCores.count", Limit: 32, Used: 8, Unit: "cores"}}})
}

func TestDecodeSegments(t *testing.T) {
	segs, err := DecodeSegments(sampleRun().Workload.Segments)
	if err != nil {
		t.Fatal(err)
	}
	if segs[0].Execution.Limit.Kind != "duration" || segs[0].Execution.Limit.Duration.Std() != 30*time.Second {
		t.Fatalf("limit decoded wrong: %+v", segs[0].Execution.Limit)
	}
}
