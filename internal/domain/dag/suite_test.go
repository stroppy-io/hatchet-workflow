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
	// Default (nil) scheduling is sequential => MaxParallelism 1.
	if got := suiteDag.GetScheduling().GetMaxParallelism(); got != 1 {
		t.Errorf("default MaxParallelism = %d, want 1", got)
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
	if got := suiteDag.GetScheduling().GetMaxParallelism(); got != 4 {
		t.Errorf("MaxParallelism = %d, want 4", got)
	}
	if got := suiteDag.GetScheduling().GetOnNodeFailure(); got != primitive.Dag_Scheduling_ON_NODE_FAILURE_CONTINUE {
		t.Errorf("OnNodeFailure = %s, want CONTINUE", got)
	}
}
