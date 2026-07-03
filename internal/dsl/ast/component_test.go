package ast_test

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
)

// Fixtures below are transcribed verbatim from spec §6 ("Слой контрактов"),
// with §5's lowering table folded into the yandex manifest fixture since the
// spec presents them as two excerpts of the same providers/yandex/manifest.yaml
// file.

const etcdComponentYAML = `
inputs:
  nodes: { type: machine_group }
requires:
  - expr: "inputs.nodes.count >= 3 && inputs.nodes.count % 2 == 1"
    message: "etcd: нечётный кворум ≥ 3"
provides: [kv.etcd]
`

const patroniComponentYAML = `
requires:
  - capability: kv.etcd
  - expr: "inputs.nodes.every(m, m.ram_gb >= 4)"
`

const insertWorkloadYAML = `
inputs: { iterations: int, row_bytes: { type: int, default: 512 } }
requires:
  - expr: "target.disk_gb * 1e9 >= inputs.iterations * inputs.row_bytes * 2.5"
    message: "диск мал для заданного числа итераций"
`

const yandexManifestYAML = `
name: yandex
provides: [machines, disk.network-ssd, disk.local-ssd, network.private]
capabilities:
  machine:
    cpu:  { enum: [2, 4, 8, 16, 32, 64] }
    ram:  { expr: "ram_gb in [cpu, cpu*2, cpu*4, cpu*8]" }
  constraints:
    - expr: "machine.disk.type == 'local-ssd' ? machine.cpu >= 8 : true"
      message: "local-ssd только от 8 vCPU"
lowering:
  disk.type: { ssd: network-ssd, hdd: network-hdd, local: local-ssd }
`

func TestDecodeComponentEtcd(t *testing.T) {
	doc, diags := ast.DecodeComponent("component.yaml", []byte(etcdComponentYAML))
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	if doc.Inputs["nodes"].Type != "machine_group" {
		t.Fatalf("bad inputs.nodes: %+v", doc.Inputs["nodes"])
	}
	if len(doc.Requires) != 1 || doc.Requires[0].Expr == "" {
		t.Fatalf("bad requires: %+v", doc.Requires)
	}
	if doc.Requires[0].Capability != "" {
		t.Fatalf("capability must be empty when expr is set: %+v", doc.Requires[0])
	}
	if len(doc.Provides) != 1 || doc.Provides[0] != "kv.etcd" {
		t.Fatalf("bad provides: %+v", doc.Provides)
	}
}

func TestDecodeComponentPatroni(t *testing.T) {
	doc, diags := ast.DecodeComponent("component.yaml", []byte(patroniComponentYAML))
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	if len(doc.Requires) != 2 {
		t.Fatalf("bad requires len: %+v", doc.Requires)
	}
	if doc.Requires[0].Capability != "kv.etcd" || doc.Requires[0].Expr != "" {
		t.Fatalf("bad requires[0]: %+v", doc.Requires[0])
	}
	if doc.Requires[1].Expr == "" || doc.Requires[1].Capability != "" {
		t.Fatalf("bad requires[1]: %+v", doc.Requires[1])
	}
}

func TestDecodeComponentWorkloadInsert(t *testing.T) {
	doc, diags := ast.DecodeComponent("component.yaml", []byte(insertWorkloadYAML))
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	if doc.Inputs["iterations"].Type != "int" {
		t.Fatalf("bad shorthand input: %+v", doc.Inputs["iterations"])
	}
	rowBytes := doc.Inputs["row_bytes"]
	if rowBytes.Type != "int" || rowBytes.Default != 512 {
		t.Fatalf("bad row_bytes: %+v", rowBytes)
	}
	if len(doc.Requires) != 1 || doc.Requires[0].Expr == "" {
		t.Fatalf("bad requires: %+v", doc.Requires)
	}
}

func TestDecodeProviderManifestYandex(t *testing.T) {
	doc, diags := ast.DecodeProviderManifest("manifest.yaml", []byte(yandexManifestYAML))
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	if doc.Name != "yandex" {
		t.Fatalf("bad name: %+v", doc.Name)
	}
	found := false
	for _, p := range doc.Provides {
		if p == "disk.local-ssd" {
			found = true
		}
	}
	if !found {
		t.Fatalf("bad provides: %+v", doc.Provides)
	}
	cpu := doc.Capabilities.Machine["cpu"]
	if len(cpu.Enum) != 6 {
		t.Fatalf("bad cpu enum: %+v", cpu)
	}
	ram := doc.Capabilities.Machine["ram"]
	if ram.Expr == "" {
		t.Fatalf("bad ram expr: %+v", ram)
	}
	if len(doc.Capabilities.Constraints) != 1 || doc.Capabilities.Constraints[0].Expr == "" {
		t.Fatalf("bad constraints: %+v", doc.Capabilities.Constraints)
	}
	if doc.Lowering["disk.type"]["ssd"] != "network-ssd" {
		t.Fatalf("bad lowering: %+v", doc.Lowering)
	}
}

// --- XOR rule on Requirement.Expr / Requirement.Capability ---

func TestDecodeComponentRequirementBothExprAndCapabilityErrors(t *testing.T) {
	src := []byte("requires:\n  - expr: \"true\"\n    capability: kv.etcd\n")
	_, diags := ast.DecodeComponent("component.yaml", src)
	if !diags.HasErrors() {
		t.Fatal("requirement with both expr and capability set must error")
	}
}

func TestDecodeComponentRequirementNeitherExprNorCapabilityErrors(t *testing.T) {
	src := []byte("requires:\n  - message: \"no expr, no capability\"\n")
	_, diags := ast.DecodeComponent("component.yaml", src)
	if !diags.HasErrors() {
		t.Fatal("requirement with neither expr nor capability set must error")
	}
}

// --- strict decode: unknown key position ---

func TestDecodeComponentUnknownKeyHasExactPosition(t *testing.T) {
	src := []byte("inputs:\n  nodes: { type: machine_group }\nrequires: []\nprovides: []\nbogus: true\n")
	_, diags := ast.DecodeComponent("component.yaml", src)
	if !diags.HasErrors() {
		t.Fatal("unknown top-level key must produce error diagnostic")
	}
	found := false
	for _, d := range diags {
		if d.Pos.Line == 5 && d.Pos.Col == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a diagnostic anchored at line 5 col 1 (the %q key), got: %+v", "bogus", diags)
	}
}

// --- duplicate map-entry names ---

func TestDecodeComponentDuplicateInputNamePreservesFirst(t *testing.T) {
	src := []byte("inputs:\n  nodes: { type: machine_group }\n  nodes: { type: int }\n")
	doc, diags := ast.DecodeComponent("component.yaml", src)
	if !diags.HasErrors() {
		t.Fatal("duplicate input name must produce error diagnostic")
	}
	if doc.Inputs["nodes"].Type != "machine_group" {
		t.Fatalf("first entry must be preserved, got: %+v", doc.Inputs["nodes"])
	}
}

// --- reuse of ClusterFragment (services) and Jobs ---

func TestDecodeComponentClusterFragmentAndJobs(t *testing.T) {
	src := []byte(`
cluster:
  services:
    etcd:
      on: db
      image: quay.io/etcd:v3
jobs:
  bootstrap:
    on: db
    steps:
      - cmd: etcd-init
`)
	doc, diags := ast.DecodeComponent("component.yaml", src)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	if doc.Cluster == nil || doc.Cluster.Services["etcd"].Image != "quay.io/etcd:v3" {
		t.Fatalf("bad cluster fragment: %+v", doc.Cluster)
	}
	if doc.Jobs["bootstrap"].Steps[0].Cmd != "etcd-init" {
		t.Fatalf("bad jobs: %+v", doc.Jobs)
	}
}
