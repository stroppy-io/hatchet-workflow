package runtime

import "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"

// Built-in edge predicate names. An edge that sets one of these in PredicateName
// fires only when its source node ends in the matching terminal state — the seam
// for conditional dag branches (e.g. a cleanup branch wired on_failure). Plain
// edges (no predicate, no on_status) keep defaulting to "source COMPLETED", so
// linear/fan dags need no registry at all.
const (
	PredicateOnSuccess = "on_success"
	PredicateOnFailure = "on_failure"
	PredicateAlways    = "always"
)

// DefaultPredicates is the generic predicate registry wired into the processor. It
// covers success/failure/always branching out of the box; domain-specific
// predicates (e.g. metric thresholds) are added here as the planner emits them.
func DefaultPredicates() PredicateRegistryMap {
	srcStatus := func(ctx DagContext, edge *primitive.Dag_Edge) (primitive.Status, bool) {
		for _, n := range ctx.Dag().GetNodes() {
			if n.GetId() == edge.GetSource() {
				return n.GetStatus(), true
			}
		}
		return primitive.Status_STATUS_UNSPECIFIED, false
	}
	return PredicateRegistryMap{
		PredicateOnSuccess: func(ctx DagContext, edge *primitive.Dag_Edge) bool {
			st, ok := srcStatus(ctx, edge)
			return ok && st == primitive.Status_STATUS_COMPLETED
		},
		PredicateOnFailure: func(ctx DagContext, edge *primitive.Dag_Edge) bool {
			st, ok := srcStatus(ctx, edge)
			return ok && st == primitive.Status_STATUS_FAILED
		},
		PredicateAlways: func(_ DagContext, _ *primitive.Dag_Edge) bool { return true },
	}
}
