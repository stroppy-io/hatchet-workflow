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
