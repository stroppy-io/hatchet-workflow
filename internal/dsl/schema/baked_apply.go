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
	"sort"
	"strings"

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
// paramsSchema is the provider's declared params schema (the same *Schema
// ComposeFormSchema nested under "provider" when the launch form was
// composed). This is the second half of the defense-in-depth against the
// "silent config injection" hole ComposeFormSchema's Strict() guards on the
// BakeForm path: a Baked is a plain protojson-decodable message, so a
// caller can hand dsl.Compile one that never went through BakeForm at all
// (see models.Run.Baked / the Terraform/monitor protos that also carry a
// schemapb.Baked). Without this check, any key under "provider" -- baked or
// not -- would be copied verbatim into Provider.Params and flow straight
// into lower.go's tfvars. If baked carries ANY provider-param key
// paramsSchema doesn't declare (or paramsSchema is nil while baked still
// carries provider params), the whole call fails closed: an error is
// returned and Provider.Params is left completely untouched -- partial
// application of an otherwise-suspect payload would be worse than none.
func ApplyBakedInputs(resolved *include.Resolved, baked *spb.Baked, paramsSchema *spb.Schema) error {
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

	declared := make(map[string]bool, len(paramsSchema.GetFields()))
	for _, f := range paramsSchema.GetFields() {
		declared[f.GetName()] = true
	}

	var unknown []string
	for k := range providerParams {
		if !declared[k] {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("apply baked inputs: undeclared provider param(s): %s", strings.Join(unknown, ", "))
	}

	if resolved.Cluster.Provider.Params == nil {
		resolved.Cluster.Provider.Params = make(map[string]any, len(providerParams))
	}
	for k, v := range providerParams {
		resolved.Cluster.Provider.Params[k] = v
	}
	return nil
}
