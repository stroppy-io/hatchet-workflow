package dag

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
)

// BuildSuiteDag composes a SuiteRun as ONE orchestration Dag: one dag_ref node per
// already-compiled per-test Dag, scheduled per the suite's SuitePreset.Scheduling
// (H13/B6: SuitePreset.Scheduling -> Dag.Scheduling). The per-test Dags are compiled
// by the caller (each needs its own provider-params resolution); this only wires the
// orchestration shape over them, so the SuiteRun is itself a Dag of dag_refs.
//
// Scheduling is expressed two ways, depending on the SuitePreset.Scheduling mode:
//
//   - sequential (the default, also for a nil scheduling): the dag_ref nodes are
//     chained with ordering edges node[i] -> node[i+1], so each test waits for the
//     previous one to finish. MaxParallelism stays 1.
//   - parallel{N}: no ordering edges are emitted, so the runtime is free to run all
//     tests concurrently; Dag.Scheduling.MaxParallelism caps the concurrency at N
//     (or at the test count when N is unbounded/0).
//
// on_node_failure passes straight through to Dag.Scheduling.OnNodeFailure.
func BuildSuiteDag(testDagIDs []string, sched *domain.SuitePreset_Scheduling) *primitive.Dag {
	refNodes := make([]*primitive.Dag_Node, 0, len(testDagIDs))
	for i, dagID := range testDagIDs {
		refNodes = append(refNodes, dagRefNode(fmt.Sprintf("test_%d", i), dagID))
	}

	scheduling, parallel := suiteScheduling(sched, len(testDagIDs))

	var edges []*primitive.Dag_Edge
	if !parallel {
		// Sequential: chain each test after its predecessor.
		for i := 1; i < len(refNodes); i++ {
			edges = append(edges, edge(refNodes[i-1].GetId(), refNodes[i].GetId()))
		}
	}

	return &primitive.Dag{
		Id:         ids.New(),
		Status:     primitive.Status_STATUS_PENDING,
		Nodes:      refNodes,
		Edges:      edges,
		Scheduling: scheduling,
	}
}

// suiteScheduling maps the SuitePreset scheduling onto the orchestration Dag's
// scheduling and reports whether the parallel mode was selected. Parallel runs up to
// max_parallel test dags at once (defaulting to the test count when unbounded);
// sequential (the default, including a nil scheduling) runs them one at a time.
func suiteScheduling(sched *domain.SuitePreset_Scheduling, testCount int) (*primitive.Dag_Scheduling, bool) {
	out := &primitive.Dag_Scheduling{
		MaxParallelism: 1,
		OnNodeFailure:  sched.GetOnNodeFailure(),
	}
	if p := sched.GetParallel(); p != nil {
		out.MaxParallelism = p.GetMaxParallel()
		if out.MaxParallelism == 0 {
			// Unbounded parallelism: cap at the number of tests so the runtime
			// has a concrete degree to schedule against.
			out.MaxParallelism = uint32(testCount)
		}
		return out, true
	}
	return out, false
}

// dagRefNode is a dag_ref orchestration node pointing at a per-test Dag.
func dagRefNode(id, dagID string) *primitive.Dag_Node {
	return &primitive.Dag_Node{
		Id:          id,
		ExecutionId: id,
		Status:      primitive.Status_STATUS_PENDING,
		Scheduling:  &primitive.Dag_Node_Scheduling{},
		Variant:     &primitive.Dag_Node_DagRef_{DagRef: &primitive.Dag_Node_DagRef{DagId: dagID}},
	}
}
