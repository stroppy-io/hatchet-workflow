package dag

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"github.com/stroppy-io/stroppy-cloud/internal/runtime"
)

func TestBuildSuiteDagRefsEveryTest(t *testing.T) {
	suiteDag := BuildSuiteDag([]string{"dag-a", "dag-b", "dag-c"}, nil)
	if err := runtime.ValidateDag(suiteDag); err != nil {
		t.Fatalf("validate suite dag: %v", err)
	}
	if got := len(suiteDag.GetNodes()); got != 3 {
		t.Fatalf("nodes = %d, want 3", got)
	}
	wantRefs := map[string]string{"test_0": "dag-a", "test_1": "dag-b", "test_2": "dag-c"}
	for _, n := range suiteDag.GetNodes() {
		ref := n.GetDagRef()
		if ref == nil {
			t.Fatalf("node %q is not a dag_ref", n.GetId())
		}
		if want := wantRefs[n.GetId()]; ref.GetDagId() != want {
			t.Errorf("node %q dag_ref = %q, want %q", n.GetId(), ref.GetDagId(), want)
		}
	}
	if got := suiteDag.GetStatus(); got != primitive.Status_STATUS_PENDING {
		t.Errorf("status = %s, want PENDING", got)
	}
	// Default (nil) scheduling is sequential => MaxParallelism 1.
	if got := suiteDag.GetScheduling().GetMaxParallelism(); got != 1 {
		t.Errorf("default MaxParallelism = %d, want 1", got)
	}
}

// edgeSet returns the set of "src->dst" ordering edges in the dag.
func edgeSet(dag *primitive.Dag) map[string]bool {
	set := make(map[string]bool, len(dag.GetEdges()))
	for _, e := range dag.GetEdges() {
		set[e.GetSource()+"->"+e.GetTarget()] = true
	}
	return set
}

func TestBuildSuiteDagSequentialChainsEdges(t *testing.T) {
	sched := &domain.SuitePreset_Scheduling{
		OnNodeFailure: primitive.Dag_Scheduling_ON_NODE_FAILURE_STOP,
		Mode:          &domain.SuitePreset_Scheduling_Sequential{Sequential: true},
	}
	suiteDag := BuildSuiteDag([]string{"a", "b", "c"}, sched)
	if err := runtime.ValidateDag(suiteDag); err != nil {
		t.Fatalf("validate suite dag: %v", err)
	}
	// Sequential => each test chained after the previous: test_0 -> test_1 -> test_2.
	got := edgeSet(suiteDag)
	want := []string{"test_0->test_1", "test_1->test_2"}
	if len(got) != len(want) {
		t.Fatalf("edges = %v, want exactly %v", got, want)
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing chaining edge %q (have %v)", w, got)
		}
	}
	// Sequential keeps MaxParallelism at 1.
	if mp := suiteDag.GetScheduling().GetMaxParallelism(); mp != 1 {
		t.Errorf("sequential MaxParallelism = %d, want 1", mp)
	}
	// on_node_failure maps straight through.
	if onf := suiteDag.GetScheduling().GetOnNodeFailure(); onf != primitive.Dag_Scheduling_ON_NODE_FAILURE_STOP {
		t.Errorf("OnNodeFailure = %s, want STOP", onf)
	}
}

func TestBuildSuiteDagParallelScheduling(t *testing.T) {
	sched := &domain.SuitePreset_Scheduling{
		OnNodeFailure: primitive.Dag_Scheduling_ON_NODE_FAILURE_CONTINUE,
		Mode: &domain.SuitePreset_Scheduling_Parallel_{
			Parallel: &domain.SuitePreset_Scheduling_Parallel{MaxParallel: 4},
		},
	}
	suiteDag := BuildSuiteDag([]string{"a", "b"}, sched)
	if err := runtime.ValidateDag(suiteDag); err != nil {
		t.Fatalf("validate suite dag: %v", err)
	}
	// Parallel => requested degree, no ordering edges between dag_refs.
	if got := suiteDag.GetScheduling().GetMaxParallelism(); got != 4 {
		t.Errorf("MaxParallelism = %d, want 4", got)
	}
	if got := len(suiteDag.GetEdges()); got != 0 {
		t.Errorf("parallel edges = %d, want 0 (no ordering edges)", got)
	}
	// on_node_failure maps straight through.
	if got := suiteDag.GetScheduling().GetOnNodeFailure(); got != primitive.Dag_Scheduling_ON_NODE_FAILURE_CONTINUE {
		t.Errorf("OnNodeFailure = %s, want CONTINUE", got)
	}
}

func TestBuildSuiteDagParallelUnboundedCapsAtTestCount(t *testing.T) {
	sched := &domain.SuitePreset_Scheduling{
		Mode: &domain.SuitePreset_Scheduling_Parallel_{
			Parallel: &domain.SuitePreset_Scheduling_Parallel{MaxParallel: 0}, // unbounded
		},
	}
	suiteDag := BuildSuiteDag([]string{"a", "b", "c"}, sched)
	if err := runtime.ValidateDag(suiteDag); err != nil {
		t.Fatalf("validate suite dag: %v", err)
	}
	// Unbounded parallel => capped at the number of tests (3), still no edges.
	if got := suiteDag.GetScheduling().GetMaxParallelism(); got != 3 {
		t.Errorf("unbounded MaxParallelism = %d, want 3 (test count)", got)
	}
	if got := len(suiteDag.GetEdges()); got != 0 {
		t.Errorf("parallel edges = %d, want 0", got)
	}
}
