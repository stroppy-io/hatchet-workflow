package dsl_test

import (
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/include"
)

// --- fixtures ---
//
// happy path: one machine group (db, with a "yandex:" ext block), one
// service (postgres), three jobs named so lexicographic (Id-sorted) order
// matches the brief's assertions exactly: cmd_job < service_job < wait_job.

const happyClusterYAML = `
version: 1
provider:
  use: yandex
machines:
  db:
    count: 1
    resources:
      cpu: 2
      ram: 4g
    yandex:
      platform_id: standard-v3
services:
  postgres:
    on: db
    image: postgres:16
`

const happyWorkflowYAML = `
jobs:
  cmd_job:
    on: db
    steps:
      - cmd: echo hello
  service_job:
    needs: [cmd_job]
    service: postgres
  wait_job:
    needs: [service_job]
    steps:
      - wait:
          http: http://db:5432/health
          timeout: 30s
`

func happyInput(t *testing.T) dsl.Input {
	t.Helper()
	return dsl.Input{
		Sources: include.Sources{Files: map[string][]byte{
			"cluster.yaml":  []byte(happyClusterYAML),
			"workflow.yaml": []byte(happyWorkflowYAML),
		}},
		Provider: &ast.ProviderManifest{Name: "yandex", Provides: []string{}},
	}
}

// --- Step 1/brief: happy path ---

func TestCompileHappyPath(t *testing.T) {
	plan, diags := dsl.Compile(happyInput(t))
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	if plan == nil {
		t.Fatal("expected a non-nil plan")
	}

	if len(plan.GetJobs()) != 3 {
		t.Fatalf("expected 3 jobs, got %d: %+v", len(plan.GetJobs()), plan.GetJobs())
	}

	cmdJob, serviceJob, waitJob := plan.GetJobs()[0], plan.GetJobs()[1], plan.GetJobs()[2]

	if cmdJob.GetId() != "cmd_job" {
		t.Fatalf("jobs[0].Id = %q, want %q (jobs must be sorted by name)", cmdJob.GetId(), "cmd_job")
	}
	steps := cmdJob.GetSteps().GetSteps()
	if len(steps) != 1 {
		t.Fatalf("cmd_job: expected 1 step, got %d", len(steps))
	}
	callCmd := steps[0].GetAgent().GetCallCmd()
	if callCmd == nil {
		t.Fatal("cmd_job: steps[0].agent.call_cmd not set")
	}
	if got := callCmd.GetSpec().GetScript().GetText(); got != "echo hello" {
		t.Fatalf("cmd_job: call_cmd script text = %q, want %q", got, "echo hello")
	}
	if got := steps[0].GetAgent().GetId(); got != "cmd_job/0" {
		t.Fatalf("cmd_job: steps[0].agent.id = %q, want %q", got, "cmd_job/0")
	}

	if serviceJob.GetId() != "service_job" {
		t.Fatalf("jobs[1].Id = %q, want %q", serviceJob.GetId(), "service_job")
	}
	if serviceJob.GetService() != "postgres" {
		t.Fatalf("jobs[1].Service = %q, want %q", serviceJob.GetService(), "postgres")
	}
	if got := serviceJob.GetNeeds(); len(got) != 1 || got[0] != "cmd_job" {
		t.Fatalf("service_job.Needs = %v, want [cmd_job] (needs must survive lowering)", got)
	}

	if waitJob.GetId() != "wait_job" {
		t.Fatalf("jobs[2].Id = %q, want %q", waitJob.GetId(), "wait_job")
	}
	if got := waitJob.GetNeeds(); len(got) != 1 || got[0] != "service_job" {
		t.Fatalf("wait_job.Needs = %v, want [service_job]", got)
	}
	waitSteps := waitJob.GetSteps().GetSteps()
	if len(waitSteps) != 1 || waitSteps[0].GetWait() == nil {
		t.Fatalf("wait_job: expected exactly one wait step, got: %+v", waitSteps)
	}
	if got := waitSteps[0].GetWait().GetHttp(); got != "http://db:5432/health" {
		t.Fatalf("wait_job: wait.http = %q", got)
	}

	if len(plan.GetMachineGroups()) != 1 {
		t.Fatalf("expected 1 machine group, got %d", len(plan.GetMachineGroups()))
	}
	if !strings.Contains(plan.GetMachineGroups()[0].GetExtJson(), "platform_id") {
		t.Fatalf("machine_groups[0].ExtJson = %q, want it to contain %q", plan.GetMachineGroups()[0].GetExtJson(), "platform_id")
	}

	if len(plan.GetServices()) != 1 || plan.GetServices()[0].GetName() != "postgres" {
		t.Fatalf("expected one service named postgres, got: %+v", plan.GetServices())
	}
}

// TestCompileWaitDurationRoundtrip locks that a Wait step's ast.Duration
// survives lowering to WaitStep.Timeout (a string, per the proto) and back:
// time.Duration.String() applied by lower.Lower must parse back to the
// original 30s via time.ParseDuration.
func TestCompileWaitDurationRoundtrip(t *testing.T) {
	plan, diags := dsl.Compile(happyInput(t))
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}

	var timeout string
	for _, job := range plan.GetJobs() {
		for _, step := range job.GetSteps().GetSteps() {
			if w := step.GetWait(); w != nil {
				timeout = w.GetTimeout()
			}
		}
	}
	if timeout == "" {
		t.Fatal("expected a wait step with a non-empty timeout")
	}

	got, err := time.ParseDuration(timeout)
	if err != nil {
		t.Fatalf("time.ParseDuration(%q): %v", timeout, err)
	}
	if got != 30*time.Second {
		t.Fatalf("timeout = %v, want 30s", got)
	}
}

// TestCompileDeterministic locks that two Compile calls over the same Input
// produce proto.Equal plans (MachineGroups/Services/Jobs are each sorted by
// name in lower.Lower specifically to guarantee this).
func TestCompileDeterministic(t *testing.T) {
	in := happyInput(t)

	plan1, diags1 := dsl.Compile(in)
	if diags1.HasErrors() {
		t.Fatalf("unexpected diags (first run): %+v", diags1)
	}
	plan2, diags2 := dsl.Compile(in)
	if diags2.HasErrors() {
		t.Fatalf("unexpected diags (second run): %+v", diags2)
	}

	if !proto.Equal(plan1, plan2) {
		t.Fatalf("two Compile calls produced different plans:\n1: %v\n2: %v", plan1, plan2)
	}
}

// --- Step 1/brief: contract violation e2e (proves phase wiring) ---

const etcdClusterYAML = `
version: 1
provider:
  use: docker
machines:
  db:
    count: 2
    resources:
      cpu: 4
      ram: 8g
`

const etcdWorkflowYAML = `
jobs:
  etcd:
    include: components/etcd
    inputs:
      nodes: db
`

const etcdComponentYAML = `
inputs:
  nodes: { type: machine_group }
requires:
  - expr: "inputs.nodes.count >= 3 && inputs.nodes.count % 2 == 1"
    message: "etcd: нечётный кворум ≥ 3"
provides: [kv.etcd]
jobs:
  install:
    on: ${{ inputs.nodes }}
    steps:
      - cmd: etcd-install
`

func TestCompileContractViolationEtcdQuorum(t *testing.T) {
	in := dsl.Input{
		Sources: include.Sources{Files: map[string][]byte{
			"cluster.yaml":                   []byte(etcdClusterYAML),
			"workflow.yaml":                  []byte(etcdWorkflowYAML),
			"components/etcd/component.yaml": []byte(etcdComponentYAML),
		}},
		Provider: &ast.ProviderManifest{Name: "docker", Provides: []string{}},
	}

	plan, diags := dsl.Compile(in)
	if plan != nil {
		t.Fatalf("expected a nil plan on contract violation, got: %+v", plan)
	}
	if !diags.HasErrors() {
		t.Fatal("expected a contract violation diagnostic")
	}

	found := false
	for _, d := range diags {
		if d.Module == "etcd" && strings.Contains(d.Message, "кворум") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a diag with Module=%q about the quorum, got: %+v", "etcd", diags)
	}
}

// --- extras ---

func TestCompileMissingClusterFileErrors(t *testing.T) {
	in := dsl.Input{
		Sources: include.Sources{Files: map[string][]byte{
			"workflow.yaml": []byte("jobs: {}\n"),
		}},
		Provider: &ast.ProviderManifest{Name: "yandex"},
	}

	plan, diags := dsl.Compile(in)
	if plan != nil {
		t.Fatalf("expected a nil plan, got: %+v", plan)
	}
	if !diags.HasErrors() {
		t.Fatal("expected an error diagnostic for the missing cluster.yaml")
	}
	found := false
	for _, d := range diags {
		if d.Path == "cluster.yaml" && strings.Contains(d.Message, "missing") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a diag naming the missing cluster.yaml, got: %+v", diags)
	}
}

// TestCompileSchemaInvalidClusterGatesDecode locks the phase-gating
// contract stated in the brief: a schema violation in cluster.yaml must
// stop the pipeline before ast.DecodeCluster ever runs, so the returned
// diagnostics are schema-only. The fixture below is deliberately something
// *both* schema and ast decode would reject on their own (a top-level key
// neither declares) — proving ast decode was actually skipped rather than
// merely agreeing with schema — by asserting that no diagnostic carries
// ast decode's own "unknown key" phrasing (see internal/dsl/ast/yamlwalk.go
// decoderBase.mapping).
func TestCompileSchemaInvalidClusterGatesDecode(t *testing.T) {
	const badClusterYAML = `
version: 1
extra_bogus_key: 1
machines:
  db:
    count: 1
`
	in := dsl.Input{
		Sources: include.Sources{Files: map[string][]byte{
			"cluster.yaml":  []byte(badClusterYAML),
			"workflow.yaml": []byte("jobs: {}\n"),
		}},
		Provider: &ast.ProviderManifest{Name: "yandex"},
	}

	plan, diags := dsl.Compile(in)
	if plan != nil {
		t.Fatalf("expected a nil plan, got: %+v", plan)
	}
	if !diags.HasErrors() {
		t.Fatal("expected a schema violation diagnostic")
	}
	for _, d := range diags {
		if strings.Contains(d.Message, "unknown key") {
			t.Fatalf("ast decode must not have run once schema validation failed, but got an ast-decode diagnostic: %+v", d)
		}
	}
}
