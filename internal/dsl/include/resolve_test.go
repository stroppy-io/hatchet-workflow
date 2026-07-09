package include_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
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

	resolved, diags := include.Resolve(cluster, wf, src, nil)
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

	_, diags := include.Resolve(cluster, wf, src, nil)
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

	_, diags := include.Resolve(cluster, wf, src, nil)
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

	_, diags := include.Resolve(cluster, wf, src, nil)
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
	_, diags := include.Resolve(cluster, wf, src, nil)
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

	_, diags := include.Resolve(cluster, wf, src, nil)
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

	resolved, diags := include.Resolve(cluster, wf, src, nil)
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

	resolved, diags := include.Resolve(cluster, wf, src, nil)
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

// --- Finding 1: job-name collisions between a literal job and an include-
// instantiated job must error, not silently clobber ---

func TestResolveJobNameCollisionErrors(t *testing.T) {
	cluster := testCluster(t)
	wf := testWorkflow(t, `
jobs:
  etcd/install:
    on: db
    steps:
      - cmd: literal-install
  etcd:
    include: components/etcd
    inputs: { nodes: db }
`)
	src := include.Sources{Files: map[string][]byte{
		"components/etcd/component.yaml": []byte(etcdComponentYAML),
	}}

	resolved, diags := include.Resolve(cluster, wf, src, nil)
	if !diags.HasErrors() {
		t.Fatal("literal job name colliding with an include-instantiated job name must error")
	}

	foundCollision := false
	for _, d := range diags {
		if d.Severity == diag.Error && strings.Contains(d.Message, "collides") {
			foundCollision = true
		}
	}
	if !foundCollision {
		t.Fatalf(`expected a diagnostic message containing "collides", got: %+v`, diags)
	}

	install, ok := resolved.Jobs["etcd/install"]
	if !ok {
		t.Fatal("the first-claimed entry for the colliding name must be kept, not dropped")
	}
	if len(install.Steps) != 1 || install.Steps[0].Cmd != "etcd-install" {
		t.Fatalf("collision must keep the FIRST-claimed entry (include jobs expand before "+
			"plain jobs at the same level, so the include-instantiated job wins here) and must "+
			"not be silently overwritten by the literal job, got: %+v", install)
	}
}

// --- Finding 2: nested-include input forwarding via the narrow
// ${{ inputs.<name> }} rule ---

func TestResolveNestedIncludeForwardsBoundInput(t *testing.T) {
	cluster := testCluster(t)
	wrapperYAML := `
inputs:
  outer_nodes: { type: machine_group }
jobs:
  etcd:
    include: components/etcd
    inputs: { nodes: "${{ inputs.outer_nodes }}" }
`
	wf := testWorkflow(t, `
jobs:
  wrapper:
    include: components/wrapper
    inputs: { outer_nodes: db }
`)
	src := include.Sources{Files: map[string][]byte{
		"components/wrapper/component.yaml": []byte(wrapperYAML),
		"components/etcd/component.yaml":    []byte(etcdComponentYAML),
	}}

	resolved, diags := include.Resolve(cluster, wf, src, nil)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}

	install, ok := resolved.Jobs["wrapper/etcd/install"]
	if !ok {
		t.Fatalf("expected wrapper/etcd/install job, got: %+v", keys(resolved.Jobs))
	}
	if install.On != "db" {
		t.Fatalf("forwarded outer input not resolved to the outer bound value: %+v", install.On)
	}
}

func TestResolveNestedIncludeForwardsUnknownOuterInputErrors(t *testing.T) {
	cluster := testCluster(t)
	wrapperYAML := `
inputs:
  outer_nodes: { type: machine_group }
jobs:
  etcd:
    include: components/etcd
    inputs: { nodes: "${{ inputs.no_such_input }}" }
`
	wf := testWorkflow(t, `
jobs:
  wrapper:
    include: components/wrapper
    inputs: { outer_nodes: db }
`)
	src := include.Sources{Files: map[string][]byte{
		"components/wrapper/component.yaml": []byte(wrapperYAML),
		"components/etcd/component.yaml":    []byte(etcdComponentYAML),
	}}

	_, diags := include.Resolve(cluster, wf, src, nil)
	if !diags.HasErrors() {
		t.Fatal("forwarding a nested include input from an unknown outer input name must error")
	}
}

// --- Minors ---

func TestResolveBadDefaultErrors(t *testing.T) {
	cluster := testCluster(t)
	componentYAML := `
inputs:
  nodes: { type: machine_group }
  row_bytes: { type: int, default: "oops" }
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

	_, diags := include.Resolve(cluster, wf, src, nil)
	if !diags.HasErrors() {
		t.Fatal("a default value that fails typecheck must produce an error diagnostic, not be bound as-is")
	}
}

func TestResolveMachineGroupInputRequiresClusterContext(t *testing.T) {
	wf := testWorkflow(t, `
jobs:
  etcd:
    include: components/etcd
    inputs: { nodes: db }
`)
	src := include.Sources{Files: map[string][]byte{
		"components/etcd/component.yaml": []byte(etcdComponentYAML),
	}}

	_, diags := include.Resolve(nil, wf, src, nil)
	if !diags.HasErrors() {
		t.Fatal("machine_group input with no cluster context must error")
	}
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "requires a cluster context") {
			found = true
		}
	}
	if !found {
		t.Fatalf(`expected a "requires a cluster context" diagnostic distinct from "no such machine group", got: %+v`, diags)
	}
}

func TestResolveInputTypeMismatchErrors(t *testing.T) {
	componentYAML := `
inputs:
  iterations: { type: int }
  label: { type: string }
  verbose: { type: bool }
jobs:
  install:
    on: db
    steps:
      - cmd: install
`
	cluster := testCluster(t)

	cases := []struct {
		name   string
		inputs string
	}{
		{"int", `inputs: { iterations: "not-an-int", label: x, verbose: true }`},
		{"string", `inputs: { iterations: 3, label: 42, verbose: true }`},
		{"bool", `inputs: { iterations: 3, label: x, verbose: "not-a-bool" }`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wf := testWorkflow(t, `
jobs:
  etcd:
    include: components/etcd
    `+tc.inputs+`
`)
			src := include.Sources{Files: map[string][]byte{
				"components/etcd/component.yaml": []byte(componentYAML),
			}}

			_, diags := include.Resolve(cluster, wf, src, nil)
			if !diags.HasErrors() {
				t.Fatalf("wrong-typed %s input must produce an error diagnostic", tc.name)
			}
		})
	}
}

// TestResolve_TopLevelWorkflowInputForwarding is the RED/GREEN case for the
// D4-groundwork defect: prior to threading workflowInputs into the
// top-level fragmentCtx, a workflow.yaml job's own `include: ...inputs:
// {x: "${{ inputs.y }}"}` could never resolve, because Resolve's top-level
// expandFragment call always passed boundInputs: nil. Only nested includes
// (a component forwarding one of its own bound inputs into a component it
// itself includes) had a non-nil boundInputs to forward from.
func TestResolve_TopLevelWorkflowInputForwarding(t *testing.T) {
	cluster := testCluster(t)
	wf := testWorkflow(t, `
jobs:
  ha:
    include: components/etcd
    inputs: { nodes: "${{ inputs.target_group }}" }
`)
	resolved, diags := include.Resolve(cluster, wf, include.Sources{Files: map[string][]byte{
		"components/etcd/component.yaml": []byte(etcdComponentYAML),
	}}, map[string]any{"target_group": "db"})

	require.False(t, diags.HasErrors(), diags.String())
	require.Len(t, resolved.Components, 1)
	require.Equal(t, "db", resolved.Components[0].Inputs["nodes"], "workflow-level input forwarded into the top-level include job")
}

// TestResolve_TopLevelWorkflowInputForwardingIntFromFloat64 is the SP-D
// live-stand regression: schemapb's own documented numeric contract
// (internal/dsl/schema/form.go's "Numeric contract" doc comment) decodes
// EVERY numeric baked launch-form value as float64, including int64-kind
// fields — google.protobuf.Value/structpb.Struct.AsMap() has no separate
// integer representation. Before checkInputType accepted float64 for an
// "int"-typed component input, a workflow.yaml top-level `inputs: {reps:
// {type: int}}` forwarded via `${{ inputs.reps }}` into an included
// component's own `inputs: {reps: {type: int}}` (exactly the
// spd-livestand-probe fixture's `counter` include) always failed with
// "expected int, got float64" the moment the value actually came from a
// baked launch form — i.e. every real form submission of an int field
// forwarded into a component, the entire point of SP-D's typed-scalar
// resolved_inputs guarantee.
func TestResolve_TopLevelWorkflowInputForwardingIntFromFloat64(t *testing.T) {
	cluster := testCluster(t)
	componentYAML := `
inputs:
  reps: { type: int }
jobs:
  run:
    steps:
      - cmd: echo reps
`
	wf := testWorkflow(t, `
jobs:
  counter:
    include: components/counter
    inputs: { reps: "${{ inputs.reps }}" }
`)
	resolved, diags := include.Resolve(cluster, wf, include.Sources{Files: map[string][]byte{
		"components/counter/component.yaml": []byte(componentYAML),
	}}, map[string]any{"reps": float64(7)})

	require.False(t, diags.HasErrors(), diags.String())
	require.Len(t, resolved.Components, 1)
	require.Equal(t, 7, resolved.Components[0].Inputs["reps"], "float64(7) from a baked launch form must bind as Go int 7")
}

// TestResolve_TopLevelWorkflowInputForwardingIntFromNonIntegralFloat64Errors
// asserts the coercion stays narrow: a non-integral float64 (which should
// never legitimately occur for an int-kind schemapb field, but defense in
// depth costs nothing here) is still rejected, not silently truncated.
func TestResolve_TopLevelWorkflowInputForwardingIntFromNonIntegralFloat64Errors(t *testing.T) {
	cluster := testCluster(t)
	componentYAML := `
inputs:
  reps: { type: int }
jobs:
  run:
    steps:
      - cmd: echo reps
`
	wf := testWorkflow(t, `
jobs:
  counter:
    include: components/counter
    inputs: { reps: "${{ inputs.reps }}" }
`)
	_, diags := include.Resolve(cluster, wf, include.Sources{Files: map[string][]byte{
		"components/counter/component.yaml": []byte(componentYAML),
	}}, map[string]any{"reps": 7.5})

	require.True(t, diags.HasErrors(), "a non-integral float64 must still produce an error diagnostic")
}

func keys(m map[string]ast.Job) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
