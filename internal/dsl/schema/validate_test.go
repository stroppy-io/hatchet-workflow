package schema_test

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/schema"
)

// clusterYAML mirrors the fixture in internal/dsl/ast/cluster_test.go.
const clusterYAML = `
version: 1
provider:
  use: yandex
  params: { zone: ru-central1-a }
machines:
  db:
    count: 3
    resources: { cpu: 8, ram: 32g, disk: { size: 100g, type: ssd } }
    yandex: { platform_id: standard-v3 }
services:
  postgres:
    on: db
    image: postgres:17
    network: host
    env: { A: "${{ machines.db[0].ip }}" }
    health: { http: ":8008/health", timeout: 120s }
`

func TestValidateClusterOK(t *testing.T) {
	diags := schema.Validate(schema.Cluster, "cluster.yaml", []byte(clusterYAML), nil)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
}

func TestValidateClusterNegativeCount(t *testing.T) {
	src := []byte(`
version: 1
machines:
  db:
    count: -1
    resources: { cpu: 8, ram: 32g, disk: { size: 100g, type: ssd } }
`)
	diags := schema.Validate(schema.Cluster, "cluster.yaml", src, nil)
	if !diags.HasErrors() {
		t.Fatal("count: -1 must produce an error diagnostic")
	}
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "/machines/db/count") && strings.Contains(d.Message, "minimum") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a diagnostic for instance path /machines/db/count naming the failing \"minimum\" keyword, got: %+v", diags)
	}
}

func TestValidateClusterBadHealthTimeoutType(t *testing.T) {
	src := []byte(`
version: 1
services:
  postgres:
    on: db
    image: postgres:17
    health: { http: ":8008/health", timeout: 999 }
`)
	diags := schema.Validate(schema.Cluster, "cluster.yaml", src, nil)
	if !diags.HasErrors() {
		t.Fatal("health.timeout: 999 (number, not a duration string) must produce an error diagnostic")
	}
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "/services/postgres/health/timeout") &&
			strings.Contains(d.Message, "string") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a diagnostic for instance path /services/postgres/health/timeout naming the string/type mismatch, got: %+v", diags)
	}
}

func TestValidateClusterBareIntByteSizeOK(t *testing.T) {
	// ast.ParseByteSize accepts a bare (unsuffixed) integer as a byte count,
	// and YAML decodes an unquoted `ram: 512` as a number, not a string, so
	// the schema must accept a bare integer here too (not just the
	// "512k"/"32g" string form) or it would reject documents the compiler
	// happily accepts.
	src := []byte(`
version: 1
machines:
  db:
    count: 1
    resources: { cpu: 8, ram: 512, disk: { size: 100g, type: ssd } }
`)
	diags := schema.Validate(schema.Cluster, "cluster.yaml", src, nil)
	if diags.HasErrors() {
		t.Fatalf("ram: 512 (bare integer byte count) must validate clean, got: %+v", diags)
	}
}

func TestValidateClusterBareIntDurationErrors(t *testing.T) {
	// Unlike ByteSize, ast.ParseDuration delegates to time.ParseDuration,
	// which rejects a bare integer ("512" -> "time: missing unit in
	// duration"); a duration string always needs a unit suffix (ns/us/ms/
	// s/m/h). So, unlike byteSize, duration must stay string-only: a bare
	// int here must still fail schema validation.
	src := []byte(`
version: 1
services:
  postgres:
    on: db
    image: postgres:17
    health: { http: ":8008/health", timeout: 120 }
`)
	diags := schema.Validate(schema.Cluster, "cluster.yaml", src, nil)
	if !diags.HasErrors() {
		t.Fatal("health.timeout: 120 (bare integer, no unit suffix) must still produce an error diagnostic")
	}
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "/services/postgres/health/timeout") &&
			strings.Contains(d.Message, "string") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a diagnostic for instance path /services/postgres/health/timeout naming the string/type mismatch, got: %+v", diags)
	}
}

// workflowYAML mirrors the fixture in internal/dsl/ast/workflow_test.go.
const workflowYAML = `
jobs:
  prep:
    on: db
    steps:
      - cmd: mkfs.xfs /dev/vdb
  etcd:
    needs: [prep]
    service: etcd
  bench:
    needs: [etcd]
    matrix: { workload: [insert, select] }
    service: stroppy
    with: { args: "--workload ${{ matrix.workload }}" }
`

func TestValidateWorkflowOK(t *testing.T) {
	diags := schema.Validate(schema.Workflow, "workflow.yaml", []byte(workflowYAML), nil)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
}

func TestValidateWorkflowStepTwoActionsErrors(t *testing.T) {
	src := []byte("jobs:\n  a:\n    steps:\n      - { cmd: x, dir: /y }\n")
	diags := schema.Validate(schema.Workflow, "workflow.yaml", src, nil)
	if !diags.HasErrors() {
		t.Fatal("a step with two actions (cmd and dir) must produce an error diagnostic (oneOf)")
	}
	found := false
	for _, d := range diags {
		// Two of the five oneOf branches (cmd, dir) match, so the library
		// reports "subschemas 0, 3 matched" (indices into the step's oneOf
		// list) rather than "none matched".
		if strings.Contains(d.Message, "/jobs/a/steps/0") &&
			strings.Contains(d.Message, "oneOf") &&
			strings.Contains(d.Message, "matched") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a diagnostic for instance path /jobs/a/steps/0 naming the failing oneOf, got: %+v", diags)
	}
}

func TestValidateWorkflowStepZeroActionsErrors(t *testing.T) {
	src := []byte("jobs:\n  a:\n    steps:\n      - {}\n")
	diags := schema.Validate(schema.Workflow, "workflow.yaml", src, nil)
	if !diags.HasErrors() {
		t.Fatal("a step with zero actions must produce an error diagnostic (oneOf)")
	}
	// None of the step's oneOf branches ({required:[cmd]}, {required:
	// [write_file]}, ...) match, so the library reports each branch's own
	// "missing property" cause instead of a single oneOf summary; every one
	// of them is anchored at the step's instance path.
	wantMissing := []string{"cmd", "write_file", "fetch", "dir", "wait"}
	for _, want := range wantMissing {
		found := false
		for _, d := range diags {
			if strings.Contains(d.Message, "/jobs/a/steps/0") &&
				strings.Contains(d.Message, "missing property") &&
				strings.Contains(d.Message, want) {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected a diagnostic for instance path /jobs/a/steps/0 naming missing property %q, got: %+v", want, diags)
		}
	}
}

// etcdComponentYAML mirrors the fixture in internal/dsl/ast/component_test.go.
const etcdComponentYAML = `
inputs:
  nodes: { type: machine_group }
requires:
  - expr: "inputs.nodes.count >= 3 && inputs.nodes.count % 2 == 1"
    message: "etcd: нечётный кворум ≥ 3"
provides: [kv.etcd]
`

func TestValidateComponentEtcdOK(t *testing.T) {
	diags := schema.Validate(schema.Component, "component.yaml", []byte(etcdComponentYAML), nil)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
}

func TestValidateComponentRequirementBothExprAndCapabilityErrors(t *testing.T) {
	src := []byte("requires:\n  - expr: \"true\"\n    capability: kv.etcd\n")
	diags := schema.Validate(schema.Component, "component.yaml", src, nil)
	if !diags.HasErrors() {
		t.Fatal("a requirement with both expr and capability set must produce an error diagnostic (oneOf)")
	}
	found := false
	for _, d := range diags {
		// Both oneOf branches ({required:[expr]}, {required:[capability]})
		// match, so the library reports "subschemas 0, 1 matched" rather
		// than "none matched".
		if strings.Contains(d.Message, "/requires/0") &&
			strings.Contains(d.Message, "oneOf") &&
			strings.Contains(d.Message, "matched") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a diagnostic for instance path /requires/0 naming the failing oneOf, got: %+v", diags)
	}
}

func TestMustCoreDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("MustCore panicked: %v", r)
		}
	}()
	if schema.MustCore() == nil {
		t.Fatal("MustCore returned nil")
	}
}

func TestValidateBadYAMLProducesDiagnostic(t *testing.T) {
	src := []byte("version: [1\n")
	diags := schema.Validate(schema.Cluster, "cluster.yaml", src, nil)
	if !diags.HasErrors() {
		t.Fatal("malformed YAML must produce an error diagnostic")
	}
}
