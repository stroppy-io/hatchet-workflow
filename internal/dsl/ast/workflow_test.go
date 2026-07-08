package ast_test

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
)

func TestDecodeWorkflow(t *testing.T) {
	src := []byte(`
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
`)
	doc, diags := ast.DecodeWorkflow("workflow.yaml", src)
	if diags.HasErrors() {
		t.Fatalf("diags: %+v", diags)
	}
	if doc.Jobs["bench"].Matrix["workload"][1] != "select" {
		t.Fatal("matrix lost")
	}
	if doc.Jobs["prep"].Steps[0].Cmd == "" {
		t.Fatal("cmd step lost")
	}
}

func TestDecodeWorkflowStepMustHaveOneAction(t *testing.T) {
	src := []byte("jobs:\n  a:\n    steps:\n      - { cmd: x, dir: /y }")
	_, diags := ast.DecodeWorkflow("workflow.yaml", src)
	if !diags.HasErrors() {
		t.Fatal("two actions in one step must error")
	}
}

// --- edge cases implied by the brief's semantics ---

func TestDecodeWorkflowUnknownJobKey(t *testing.T) {
	src := []byte("jobs:\n  a:\n    bogus: true\n    steps: []\n")
	_, diags := ast.DecodeWorkflow("workflow.yaml", src)
	if !diags.HasErrors() {
		t.Fatal("unknown key in job must produce error diagnostic")
	}
}

func TestDecodeWorkflowUnknownStepKey(t *testing.T) {
	src := []byte("jobs:\n  a:\n    steps:\n      - { cmd: x, bogus: 1 }\n")
	_, diags := ast.DecodeWorkflow("workflow.yaml", src)
	if !diags.HasErrors() {
		t.Fatal("unknown key in step must produce error diagnostic")
	}
}

func TestDecodeWorkflowEmptyStepsOKWhenService(t *testing.T) {
	src := []byte("jobs:\n  etcd:\n    service: etcd\n    steps: []\n")
	_, diags := ast.DecodeWorkflow("workflow.yaml", src)
	if diags.HasErrors() {
		t.Fatalf("empty steps list with a service job must not error: %+v", diags)
	}
}

func TestDecodeWorkflowDuplicateJobName(t *testing.T) {
	src := []byte(`
jobs:
  a:
    on: db
    steps:
      - cmd: x
  a:
    on: db
    steps:
      - cmd: y
`)
	_, diags := ast.DecodeWorkflow("workflow.yaml", src)
	if !diags.HasErrors() {
		t.Fatal("duplicate job name must produce error diagnostic")
	}
}

func TestDecodeWorkflowStepMustHaveAtLeastOneAction(t *testing.T) {
	src := []byte("jobs:\n  a:\n    steps:\n      - {}\n")
	_, diags := ast.DecodeWorkflow("workflow.yaml", src)
	if !diags.HasErrors() {
		t.Fatal("step with zero actions must error")
	}
}

func TestDecodeWorkflowStepFullFields(t *testing.T) {
	src := []byte(`
jobs:
  a:
    on: db
    steps:
      - write_file: { dest: /etc/x.conf, content: "hello", template: tmpl }
`)
	doc, diags := ast.DecodeWorkflow("workflow.yaml", src)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	wf := doc.Jobs["a"].Steps[0].WriteFile
	if wf == nil || wf.Dest != "/etc/x.conf" || wf.Content != "hello" || wf.Template != "tmpl" {
		t.Fatalf("bad write_file: %+v", wf)
	}
}

func TestDecodeWorkflowStepFetchAndWait(t *testing.T) {
	src := []byte(`
jobs:
  a:
    on: db
    steps:
      - fetch: { url: "https://example.com/f.tgz", dest: /tmp/f.tgz }
      - wait: { http: ":8008/health", timeout: 30s }
      - dir: /tmp
`)
	doc, diags := ast.DecodeWorkflow("workflow.yaml", src)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	steps := doc.Jobs["a"].Steps
	if steps[0].Fetch == nil || steps[0].Fetch.URL != "https://example.com/f.tgz" || steps[0].Fetch.Dest != "/tmp/f.tgz" {
		t.Fatalf("bad fetch: %+v", steps[0].Fetch)
	}
	if steps[1].Wait == nil || steps[1].Wait.HTTP != ":8008/health" || steps[1].Wait.Timeout != ast.Duration(30_000_000_000) {
		t.Fatalf("bad wait: %+v", steps[1].Wait)
	}
	if steps[2].Dir != "/tmp" {
		t.Fatalf("bad dir: %+v", steps[2])
	}
}

// TestDecodeWorkflowInputs verifies workflow.yaml's top-level "inputs:" key
// decodes into WorkflowDoc.Inputs, mirroring ComponentDoc.Inputs's existing
// {bare scalar type | {type, default} mapping} shape (component_test.go
// covers that shape in depth; this only proves workflow.yaml recognizes the
// key at all rather than reporting it "unknown key").
func TestDecodeWorkflowInputs(t *testing.T) {
	src := []byte(`
inputs:
  iterations: int
  row_bytes: { type: int, default: 512 }
  label: { type: string, default: "bench" }
jobs:
  a:
    on: db
    steps:
      - cmd: x
`)
	doc, diags := ast.DecodeWorkflow("workflow.yaml", src)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	if got := doc.Inputs["iterations"]; got.Type != "int" {
		t.Fatalf("bad iterations input: %+v", got)
	}
	if got := doc.Inputs["row_bytes"]; got.Type != "int" || got.Default != 512 {
		t.Fatalf("bad row_bytes input: %+v", got)
	}
	if got := doc.Inputs["label"]; got.Type != "string" || got.Default != "bench" {
		t.Fatalf("bad label input: %+v", got)
	}
}

// TestDecodeWorkflowNoInputsOK verifies a workflow.yaml with no "inputs:"
// key decodes cleanly to an empty (non-nil) Inputs map, matching
// ComponentDoc's zero-value convention.
func TestDecodeWorkflowNoInputsOK(t *testing.T) {
	src := []byte("jobs:\n  a:\n    on: db\n    steps:\n      - cmd: x\n")
	doc, diags := ast.DecodeWorkflow("workflow.yaml", src)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	if doc.Inputs == nil || len(doc.Inputs) != 0 {
		t.Fatalf("expected empty non-nil Inputs map, got %+v", doc.Inputs)
	}
}

func TestDecodeWorkflowJobFields(t *testing.T) {
	src := []byte(`
jobs:
  bench:
    needs: [etcd, prep]
    when: "matrix.workload == 'insert'"
    matrix: { workload: [insert, select] }
    service: stroppy
    with: { args: "x" }
    steps: []
`)
	doc, diags := ast.DecodeWorkflow("workflow.yaml", src)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	job := doc.Jobs["bench"]
	if len(job.Needs) != 2 || job.Needs[0] != "etcd" || job.Needs[1] != "prep" {
		t.Fatalf("bad needs: %+v", job.Needs)
	}
	if job.When != "matrix.workload == 'insert'" {
		t.Fatalf("bad when: %+v", job.When)
	}
	if job.With["args"] != "x" {
		t.Fatalf("bad with: %+v", job.With)
	}
}

// --- include/inputs fields (Task 8) ---

func TestDecodeWorkflowJobInclude(t *testing.T) {
	src := []byte(`
jobs:
  ha:
    needs: [prep]
    include: components/patroni
    inputs: { nodes: db }
`)
	doc, diags := ast.DecodeWorkflow("workflow.yaml", src)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	job := doc.Jobs["ha"]
	if job.Include != "components/patroni" {
		t.Fatalf("bad include: %+v", job.Include)
	}
	if job.Inputs["nodes"] != "db" {
		t.Fatalf("bad inputs: %+v", job.Inputs)
	}
	if len(job.Needs) != 1 || job.Needs[0] != "prep" {
		t.Fatalf("bad needs: %+v", job.Needs)
	}
}

func TestDecodeWorkflowJobIncludeWithServiceErrors(t *testing.T) {
	src := []byte("jobs:\n  ha:\n    include: components/patroni\n    service: etcd\n")
	_, diags := ast.DecodeWorkflow("workflow.yaml", src)
	if !diags.HasErrors() {
		t.Fatal("include combined with service must produce error diagnostic")
	}
}

func TestDecodeWorkflowJobIncludeWithStepsErrors(t *testing.T) {
	src := []byte("jobs:\n  ha:\n    include: components/patroni\n    steps:\n      - cmd: x\n")
	_, diags := ast.DecodeWorkflow("workflow.yaml", src)
	if !diags.HasErrors() {
		t.Fatal("include combined with steps must produce error diagnostic")
	}
}

func TestDecodeWorkflowJobIncludeWithOnErrors(t *testing.T) {
	src := []byte("jobs:\n  ha:\n    include: components/patroni\n    on: db\n")
	_, diags := ast.DecodeWorkflow("workflow.yaml", src)
	if !diags.HasErrors() {
		t.Fatal("include combined with on must produce error diagnostic")
	}
}

func TestDecodeWorkflowJobIncludeWithMatrixErrors(t *testing.T) {
	src := []byte("jobs:\n  ha:\n    include: components/patroni\n    matrix: { x: [a, b] }\n")
	_, diags := ast.DecodeWorkflow("workflow.yaml", src)
	if !diags.HasErrors() {
		t.Fatal("include combined with matrix must produce error diagnostic")
	}
}

// --- MatrixValues is runtime-only (Task 11) ---

func TestDecodeWorkflowRejectsMatrixValuesKey(t *testing.T) {
	src := []byte("jobs:\n  a:\n    on: db\n    matrix_values: { x: a }\n    steps:\n      - cmd: y\n")
	_, diags := ast.DecodeWorkflow("workflow.yaml", src)
	if !diags.HasErrors() {
		t.Fatal("matrix_values: is a runtime-only field and must not decode from YAML")
	}
}
