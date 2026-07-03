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
		if strings.Contains(d.Message, "count") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a diagnostic mentioning the instance path for machines.db.count, got: %+v", diags)
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
}

func TestValidateWorkflowStepZeroActionsErrors(t *testing.T) {
	src := []byte("jobs:\n  a:\n    steps:\n      - {}\n")
	diags := schema.Validate(schema.Workflow, "workflow.yaml", src, nil)
	if !diags.HasErrors() {
		t.Fatal("a step with zero actions must produce an error diagnostic (oneOf)")
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
