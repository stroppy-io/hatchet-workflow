package graph_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/graph"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/include"
)

func testClusterDoc() *ast.ClusterDoc {
	return &ast.ClusterDoc{
		Machines: map[string]ast.MachineGroup{
			"db":  {Count: 3, Resources: ast.Resources{CPU: 8, RAM: 32 << 30}},
			"app": {Count: 1, Resources: ast.Resources{CPU: 2, RAM: 4 << 30}},
		},
		Services: map[string]ast.Service{
			"etcd":    {On: "db", Image: "etcd:latest"},
			"stroppy": {On: "app", Image: "stroppy:latest"},
		},
	}
}

func resolvedWith(jobs map[string]ast.Job) *include.Resolved {
	return &include.Resolved{
		Cluster: testClusterDoc(),
		Jobs:    jobs,
	}
}

// --- brief's required cases ---

func TestValidateCycleDetected(t *testing.T) {
	jobs := map[string]ast.Job{
		"a": {Needs: []string{"b"}, On: "db", Steps: []ast.Step{{Cmd: "x"}}},
		"b": {Needs: []string{"a"}, On: "db", Steps: []ast.Step{{Cmd: "y"}}},
	}
	diags := graph.Validate(resolvedWith(jobs))
	if !diags.HasErrors() {
		t.Fatal("expected a cycle error")
	}
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "cycle") && strings.Contains(d.Message, "a") && strings.Contains(d.Message, "b") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected cycle diagnostic naming both a and b, got: %+v", diags)
	}
}

func TestValidateMissingNeedsIsError(t *testing.T) {
	jobs := map[string]ast.Job{
		"a": {Needs: []string{"nosuch"}, On: "db", Steps: []ast.Step{{Cmd: "x"}}},
	}
	diags := graph.Validate(resolvedWith(jobs))
	if !diags.HasErrors() {
		t.Fatal("expected an error for needs: [nosuch]")
	}
}

func TestValidateMissingServiceIsError(t *testing.T) {
	jobs := map[string]ast.Job{
		"a": {Service: "nosuch"},
	}
	diags := graph.Validate(resolvedWith(jobs))
	if !diags.HasErrors() {
		t.Fatal("expected an error for service: nosuch")
	}
}

func TestValidateInvalidCELInStepCmdIsError(t *testing.T) {
	jobs := map[string]ast.Job{
		"a": {On: "db", Steps: []ast.Step{{Cmd: "${{ machines.db.cpu +^ }}"}}},
	}
	diags := graph.Validate(resolvedWith(jobs))
	if !diags.HasErrors() {
		t.Fatal("expected a CEL compile error for invalid expression in step cmd")
	}
}

func TestExpandMatrixTwoByTwo(t *testing.T) {
	jobs := map[string]ast.Job{
		"bench": {
			On:     "db",
			Matrix: map[string][]string{"workload": {"insert", "select"}, "size": {"small", "big"}},
			Steps:  []ast.Step{{Cmd: "run"}},
		},
		"report": {
			Needs: []string{"bench"},
			On:    "db",
			Steps: []ast.Step{{Cmd: "report"}},
		},
	}
	out := graph.Expand(resolvedWith(jobs))

	var benchInstances []string
	for name := range out {
		if strings.HasPrefix(name, "bench[") {
			benchInstances = append(benchInstances, name)
		}
	}
	sort.Strings(benchInstances)
	if len(benchInstances) != 4 {
		t.Fatalf("expected 4 bench instances, got %d: %+v", len(benchInstances), benchInstances)
	}

	want := []string{
		"bench[size=big,workload=insert]",
		"bench[size=big,workload=select]",
		"bench[size=small,workload=insert]",
		"bench[size=small,workload=select]",
	}
	for i, w := range want {
		if benchInstances[i] != w {
			t.Fatalf("instance names = %+v, want %+v", benchInstances, want)
		}
	}

	for _, name := range benchInstances {
		job := out[name]
		if job.Matrix != nil {
			t.Fatalf("instance %q: Matrix must be nil'd, got %+v", name, job.Matrix)
		}
		if job.MatrixValues["workload"] == "" || job.MatrixValues["size"] == "" {
			t.Fatalf("instance %q: MatrixValues not set: %+v", name, job.MatrixValues)
		}
	}

	report, ok := out["report"]
	if !ok {
		t.Fatal("report job missing from Expand output")
	}
	if len(report.Needs) != 4 {
		t.Fatalf("report.Needs = %+v, want all 4 bench instances", report.Needs)
	}
	sortedNeeds := append([]string{}, report.Needs...)
	sort.Strings(sortedNeeds)
	for i, w := range want {
		if sortedNeeds[i] != w {
			t.Fatalf("report.Needs = %+v, want %+v", sortedNeeds, want)
		}
	}
}

// --- extra cases per task instructions ---

func TestValidateDiamondDAGNoFalseCycle(t *testing.T) {
	jobs := map[string]ast.Job{
		"a": {On: "db", Steps: []ast.Step{{Cmd: "x"}}},
		"b": {Needs: []string{"a"}, On: "db", Steps: []ast.Step{{Cmd: "x"}}},
		"c": {Needs: []string{"a"}, On: "db", Steps: []ast.Step{{Cmd: "x"}}},
		"d": {Needs: []string{"b", "c"}, On: "db", Steps: []ast.Step{{Cmd: "x"}}},
	}
	diags := graph.Validate(resolvedWith(jobs))
	if diags.HasErrors() {
		t.Fatalf("diamond DAG must not be flagged as a cycle: %+v", diags)
	}
}

func TestExpandMatrixToMatrixNeeds(t *testing.T) {
	jobs := map[string]ast.Job{
		"load": {
			On:     "db",
			Matrix: map[string][]string{"phase": {"a", "b"}},
			Steps:  []ast.Step{{Cmd: "load"}},
		},
		"bench": {
			Needs:  []string{"load"},
			On:     "db",
			Matrix: map[string][]string{"workload": {"insert", "select"}},
			Steps:  []ast.Step{{Cmd: "bench"}},
		},
	}
	out := graph.Expand(resolvedWith(jobs))

	for name, job := range out {
		if !strings.HasPrefix(name, "bench[") {
			continue
		}
		if len(job.Needs) != 2 {
			t.Fatalf("instance %q needs = %+v, want both load instances (no matrix alignment in v1)", name, job.Needs)
		}
		sorted := append([]string{}, job.Needs...)
		sort.Strings(sorted)
		if sorted[0] != "load[phase=a]" || sorted[1] != "load[phase=b]" {
			t.Fatalf("instance %q needs = %+v, want [load[phase=a] load[phase=b]]", name, sorted)
		}
	}
}

func TestExpandEmptyMatrixValueYieldsZeroInstancesAndDanglingNeedsDrop(t *testing.T) {
	jobs := map[string]ast.Job{
		"bench": {
			On:     "db",
			Matrix: map[string][]string{"workload": {}},
			Steps:  []ast.Step{{Cmd: "bench"}},
		},
		"report": {
			Needs: []string{"bench"},
			On:    "db",
			Steps: []ast.Step{{Cmd: "report"}},
		},
	}
	out := graph.Expand(resolvedWith(jobs))

	for name := range out {
		if strings.HasPrefix(name, "bench") {
			t.Fatalf("expected bench to vanish entirely (zero instances), found %q", name)
		}
	}

	report, ok := out["report"]
	if !ok {
		t.Fatal("report job missing from Expand output")
	}
	if len(report.Needs) != 0 {
		t.Fatalf("report.Needs = %+v, want empty (dependency vanished)", report.Needs)
	}
}

func TestExpandJobsWithoutMatrixCopiedAsIs(t *testing.T) {
	jobs := map[string]ast.Job{
		"prep": {On: "db", Steps: []ast.Step{{Cmd: "mkfs"}}},
	}
	out := graph.Expand(resolvedWith(jobs))
	if len(out) != 1 {
		t.Fatalf("expected exactly one job, got %+v", out)
	}
	if out["prep"].On != "db" || out["prep"].Steps[0].Cmd != "mkfs" {
		t.Fatalf("non-matrix job not copied as-is: %+v", out["prep"])
	}
}

func TestValidateWhenCELChecked(t *testing.T) {
	jobs := map[string]ast.Job{
		"a": {On: "db", When: "matrix.workload +^", Steps: []ast.Step{{Cmd: "x"}}},
	}
	diags := graph.Validate(resolvedWith(jobs))
	if !diags.HasErrors() {
		t.Fatal("expected a CEL compile error for invalid job.When expression")
	}
}

func TestValidateServiceEnvCELChecked(t *testing.T) {
	r := resolvedWith(map[string]ast.Job{
		"a": {On: "db", Steps: []ast.Step{{Cmd: "x"}}},
	})
	r.Cluster.Services["etcd"] = ast.Service{
		On:    "db",
		Image: "etcd:latest",
		Env:   map[string]string{"PEERS": "${{ machines.db.cpu +^ }}"},
	}
	diags := graph.Validate(r)
	if !diags.HasErrors() {
		t.Fatal("expected a CEL compile error for invalid expression in service env value")
	}
}

func TestValidateOnUnknownGroupIsError(t *testing.T) {
	jobs := map[string]ast.Job{
		"a": {On: "nosuch", Steps: []ast.Step{{Cmd: "x"}}},
	}
	diags := graph.Validate(resolvedWith(jobs))
	if !diags.HasErrors() {
		t.Fatal("expected an error for on: referencing an unknown machine group")
	}
}

func TestValidateGoodWorkflowNoErrors(t *testing.T) {
	jobs := map[string]ast.Job{
		"prep": {On: "db", Steps: []ast.Step{{Cmd: "mkfs.xfs /dev/vdb"}}},
		"etcd": {Needs: []string{"prep"}, Service: "etcd"},
		"bench": {
			Needs:  []string{"etcd"},
			On:     "db",
			Matrix: map[string][]string{"workload": {"insert", "select"}},
			When:   "matrix.workload == 'insert'",
			With:   map[string]string{"args": "--workload ${{ matrix.workload }}"},
			Steps:  []ast.Step{{Cmd: "stroppy run --cpu ${{ machines.db.machines[0].cpu }}"}},
		},
	}
	diags := graph.Validate(resolvedWith(jobs))
	if diags.HasErrors() {
		t.Fatalf("valid workflow must not produce errors: %+v", diags)
	}
}

func TestValidateRejectsReservedCharsInJobNames(t *testing.T) {
	jobs := map[string]ast.Job{
		"bench[workload=insert]": {On: "db", Steps: []ast.Step{{Cmd: "x"}}},
	}
	diags := graph.Validate(resolvedWith(jobs))
	if !diags.HasErrors() {
		t.Fatal("expected an error for job name with reserved characters []=,")
	}
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "bench[workload=insert]") && strings.Contains(d.Message, "reserved") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected diagnostic naming the job and 'reserved', got: %+v", diags)
	}
}
