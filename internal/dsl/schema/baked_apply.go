// Package schema (this file): the launch-time bridge from a sealed
// schemapb.Baked launch form to the two places its typed values actually
// flow before/around compilation — see docs/superpowers/plans/
// 2026-07-08-sp-d-launch-form.md Task 6/8 for why this is NOT a
// BoundComponent.Inputs overlay (the spec's original sketch): provider
// params live on ast.ClusterDoc.Provider.Params (consumed by lower.go's
// lowerProvider), a completely different field from an include's bound
// component inputs (which are already typed and forwarded during
// include.Resolve itself, via the workflowInputs parameter Resolve now
// takes — see internal/dsl/include/resolve.go).
package schema

import (
	"fmt"

	spb "github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/include"
)

// SplitBakedValues splits a Baked launch form's values into the two
// destinations ComposeFormSchema's own layout implies (SP-A: workflow
// inputs hoisted to the top level, provider params nested under
// "provider" — see form.go's ComposeFormSchema): workflowInputs is
// everything except the "provider" key (to feed include.Resolve's
// workflowInputs parameter), providerParams is the "provider" object's
// contents (to overlay onto ast.ClusterDoc.Provider.Params). A nil baked
// (no form submitted — e.g. a non-form launch path) returns (nil, nil).
func SplitBakedValues(baked *spb.Baked) (workflowInputs, providerParams map[string]any) {
	if baked.GetValues() == nil {
		return nil, nil
	}
	all := baked.GetValues().AsMap()
	if p, ok := all["provider"].(map[string]any); ok {
		providerParams = p
	}
	workflowInputs = make(map[string]any, len(all))
	for k, v := range all {
		if k == "provider" {
			continue
		}
		workflowInputs[k] = v
	}
	return workflowInputs, providerParams
}

// ApplyBakedInputs overlays baked's provider-param values onto
// resolved.Cluster.Provider.Params (baked values win over cluster.yaml's
// static provider.params — the launch form is the source of truth for a
// form-driven launch). It is a no-op (nil error) when baked is nil, so
// callers can invoke it unconditionally on the non-form launch path.
//
// Workflow-level input values are NOT applied here: they must be known
// BEFORE/DURING include.Resolve (its `${{ inputs.x }}` substitution runs
// inline during Resolve, not as a post-process — see resolve.go's
// forwardInputs/substituteOn), so callers thread them via
// SplitBakedValues + include.Resolve's workflowInputs parameter instead
// (see internal/dsl/compiler.go's Compile).
func ApplyBakedInputs(resolved *include.Resolved, baked *spb.Baked) error {
	if baked == nil {
		return nil
	}
	if resolved == nil || resolved.Cluster == nil {
		return fmt.Errorf("apply baked inputs: resolved cluster is required")
	}
	_, providerParams := SplitBakedValues(baked)
	if len(providerParams) == 0 {
		return nil
	}
	if resolved.Cluster.Provider.Params == nil {
		resolved.Cluster.Provider.Params = make(map[string]any, len(providerParams))
	}
	for k, v := range providerParams {
		resolved.Cluster.Provider.Params[k] = v
	}
	return nil
}
