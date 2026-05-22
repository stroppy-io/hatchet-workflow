package dag

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
)

// BuildSuiteDag composes a SuiteRun as ONE orchestration Dag: one dag_ref node per
// already-compiled per-test Dag, scheduled per the suite's SuitePreset.Scheduling
// (H13: SuitePreset.Scheduling -> Dag.Scheduling). The per-test Dags are compiled
// by the caller (each needs its own provider-params resolution); this only wires the
// orchestration shape over them, so the SuiteRun is itself a Dag of dag_refs.
//
// TODO(dag): scheduling is coarse — sequential/parallel modes only set
// MaxParallelism (sequential => 1, parallel => max_parallel). Explicit
// edge-ordering between dag_refs from a richer SuitePreset.Scheduling is not yet
// expressed. Reported.
func BuildSuiteDag(testDagIDs []string, sched *domain.SuitePreset_Scheduling) *primitive.Dag {
	refNodes := make([]*primitive.Dag_Node, 0, len(testDagIDs))
	for i, dagID := range testDagIDs {
		refNodes = append(refNodes, dagRefNode(fmt.Sprintf("test_%d", i), dagID))
	}
	return &primitive.Dag{
		Id:         ids.New(),
		Status:     primitive.Status_STATUS_PENDING,
		Nodes:      refNodes,
		Scheduling: suiteScheduling(sched),
	}
}

// suiteScheduling maps the SuitePreset scheduling onto the orchestration Dag's
// scheduling: parallel runs up to max_parallel test dags at once, sequential (the
// default) runs them one at a time.
func suiteScheduling(sched *domain.SuitePreset_Scheduling) *primitive.Dag_Scheduling {
	maxParallel := uint32(1)
	if p := sched.GetParallel(); p != nil && p.GetMaxParallel() > 0 {
		maxParallel = p.GetMaxParallel()
	}
	return &primitive.Dag_Scheduling{
		MaxParallelism: maxParallel,
		OnNodeFailure:  sched.GetOnNodeFailure(),
	}
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
