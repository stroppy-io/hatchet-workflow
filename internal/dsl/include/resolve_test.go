package include_test

import (
	"sort"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/include"
)

const etcdComponentYAML = `
inputs:
  nodes: { type: machine_group }
jobs:
  install:
    on: ${{ inputs.nodes }}
    steps:
      - cmd: etcd-install
`

func testCluster(t *testing.T) *ast.ClusterDoc {
	t.Helper()
	src := []byte(`
machines:
  db:
    count: 3
    resources: { cpu: 4, ram: 8g }
  app:
    count: 1
    resources: { cpu: 2, ram: 4g }
`)
	doc, diags := ast.DecodeCluster("cluster.yaml", src, "yandex")
	if diags.HasErrors() {
		t.Fatalf("bad cluster fixture: %+v", diags)
	}
	return doc
}

func testWorkflow(t *testing.T, src string) *ast.WorkflowDoc {
	t.Helper()
	doc, diags := ast.DecodeWorkflow("workflow.yaml", []byte(src))
	if diags.HasErrors() {
		t.Fatalf("bad workflow fixture: %+v", diags)
	}
	return doc
}

// --- Step 1/brief: etcd fragment instantiation + needs rewrite ---

func TestResolveInstantiatesFragmentJobs(t *testing.T) {
	cluster := testCluster(t)
	wf := testWorkflow(t, `
jobs:
  prep:
    on: db
    steps:
      - cmd: mkfs.xfs /dev/vdb
  etcd:
    needs: [prep]
    include: components/etcd
    inputs: { nodes: db }
  bench:
    needs: [etcd]
    on: db
    steps:
      - cmd: run-bench
`)
	src := include.Sources{Files: map[string][]byte{
		"components/etcd/component.yaml": []byte(etcdComponentYAML),
	}}

	resolved, diags := include.Resolve(cluster, wf, src)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}

	if _, ok := resolved.Jobs["etcd"]; ok {
		t.Fatal("include job itself must not survive into Resolved.Jobs")
	}
	install, ok := resolved.Jobs["etcd/install"]
	if !ok {
		t.Fatalf("expected etcd/install job, got: %+v", keys(resolved.Jobs))
	}
	if install.On != "db" {
		t.Fatalf("On not substituted from inputs.nodes: %+v", install.On)
	}
	if len(install.Needs) != 1 || install.Needs[0] != "prep" {
		t.Fatalf("include job's own needs must land on fragment root: %+v", install.Needs)
	}

	bench, ok := resolved.Jobs["bench"]
	if !ok {
		t.Fatal("plain job bench must survive")
	}
	if len(bench.Needs) != 1 || bench.Needs[0] != "etcd/install" {
		t.Fatalf("external needs on include job must rewrite to instantiated jobs: %+v", bench.Needs)
	}

	if len(resolved.Components) != 1 {
		t.Fatalf("expected one bound component, got: %+v", resolved.Components)
	}
	bc := resolved.Components[0]
	if bc.Name != "etcd" {
		t.Fatalf("bad bound component name: %+v", bc.Name)
	}
	if bc.Inputs["nodes"] != "db" {
		t.Fatalf("bad bound component inputs: %+v", bc.Inputs)
	}
	if bc.Doc.Inputs["nodes"].Type != "machine_group" {
		t.Fatalf("BoundComponent.Doc not the decoded component: %+v", bc.Doc)
	}
}

func TestResolveMissingRequiredInputErrors(t *testing.T) {
	cluster := testCluster(t)
	wf := testWorkflow(t, `
jobs:
  etcd:
    include: components/etcd
`)
	src := include.Sources{Files: map[string][]byte{
		"components/etcd/component.yaml": []byte(etcdComponentYAML),
	}}

	_, diags := include.Resolve(cluster, wf, src)
	if !diags.HasErrors() {
		t.Fatal("missing required input must produce error diagnostic")
	}
}

func TestResolveNoSuchMachineGroupErrors(t *testing.T) {
	cluster := testCluster(t)
	wf := testWorkflow(t, `
jobs:
  etcd:
    include: components/etcd
    inputs: { nodes: nosuch }
`)
	src := include.Sources{Files: map[string][]byte{
		"components/etcd/component.yaml": []byte(etcdComponentYAML),
	}}

	_, diags := include.Resolve(cluster, wf, src)
	if !diags.HasErrors() {
		t.Fatal("inputs.nodes referencing a nonexistent machine group must error")
	}
}

func TestResolveMissingComponentFileErrors(t *testing.T) {
	cluster := testCluster(t)
	wf := testWorkflow(t, `
jobs:
  etcd:
    include: components/etcd
    inputs: { nodes: db }
`)
	src := include.Sources{Files: map[string][]byte{}}

	_, diags := include.Resolve(cluster, wf, src)
	if !diags.HasErrors() {
		t.Fatal("missing component.yaml must error")
	}
}

func TestResolveIncludeCycleErrorsWithoutHanging(t *testing.T) {
	cluster := testCluster(t)
	wf := testWorkflow(t, `
jobs:
  x:
    include: components/a
`)
	src := include.Sources{Files: map[string][]byte{
		"components/a/component.yaml": []byte("jobs:\n  step1:\n    include: components/b\n"),
		"components/b/component.yaml": []byte("jobs:\n  step1:\n    include: components/a\n"),
	}}

	// Resolve must terminate on its own (visiting-set cycle detection); if
	// it doesn't, go test's own timeout catches the hang and fails loudly
	// rather than this test blocking forever.
	_, diags := include.Resolve(cluster, wf, src)
	if !diags.HasErrors() {
		t.Fatal("include cycle A->B->A must produce an error diagnostic")
	}
}

// --- edge cases ---

func TestResolveUnknownInputKeyErrors(t *testing.T) {
	cluster := testCluster(t)
	wf := testWorkflow(t, `
jobs:
  etcd:
    include: components/etcd
    inputs: { nodes: db, bogus: 1 }
`)
	src := include.Sources{Files: map[string][]byte{
		"components/etcd/component.yaml": []byte(etcdComponentYAML),
	}}

	_, diags := include.Resolve(cluster, wf, src)
	if !diags.HasErrors() {
		t.Fatal("unknown input key must produce error diagnostic")
	}
}

func TestResolveDefaultApplied(t *testing.T) {
	cluster := testCluster(t)
	componentYAML := `
inputs:
  nodes: { type: machine_group }
  row_bytes: { type: int, default: 512 }
jobs:
  install:
    on: ${{ inputs.nodes }}
    steps:
      - cmd: install
`
	wf := testWorkflow(t, `
jobs:
  etcd:
    include: components/etcd
    inputs: { nodes: db }
`)
	src := include.Sources{Files: map[string][]byte{
		"components/etcd/component.yaml": []byte(componentYAML),
	}}

	resolved, diags := include.Resolve(cluster, wf, src)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	if len(resolved.Components) != 1 {
		t.Fatalf("expected one bound component: %+v", resolved.Components)
	}
	if resolved.Components[0].Inputs["row_bytes"] != 512 {
		t.Fatalf("default not applied: %+v", resolved.Components[0].Inputs)
	}
}

func TestResolveNestedIncludePrefixing(t *testing.T) {
	cluster := testCluster(t)
	wrapperYAML := `
jobs:
  inner:
    include: components/etcd
    inputs: { nodes: db }
`
	wf := testWorkflow(t, `
jobs:
  outer:
    include: components/wrapper
`)
	src := include.Sources{Files: map[string][]byte{
		"components/wrapper/component.yaml": []byte(wrapperYAML),
		"components/etcd/component.yaml":    []byte(etcdComponentYAML),
	}}

	resolved, diags := include.Resolve(cluster, wf, src)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	if _, ok := resolved.Jobs["outer/inner/install"]; !ok {
		t.Fatalf("expected nested-prefixed job outer/inner/install, got: %+v", keys(resolved.Jobs))
	}

	var names []string
	for _, bc := range resolved.Components {
		names = append(names, bc.Name)
	}
	sort.Strings(names)
	if len(names) != 2 || names[0] != "outer" || names[1] != "outer/inner" {
		t.Fatalf("bad nested bound component names: %+v", names)
	}
}

func keys(m map[string]ast.Job) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
