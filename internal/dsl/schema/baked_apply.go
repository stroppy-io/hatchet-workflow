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

	if unknown := unknownKeys(paramsSchema.GetFields(), providerParams); len(unknown) > 0 {
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

// unknownKeys is the recursive counterpart of the top-level-only `declared`
// check this function used to do inline (see SP-I1): it walks values against
// fields the same way schemapb.Schema.Bake's own strict-mode check would
// (schemapb/validate.go's checkObject -> validateFields), so a Baked that
// bypassed BakeForm entirely still gets the same defense this package's own
// ComposeFormSchema/BakeForm path gets from schemapb's Strict flag.
//
// A nil/empty fields slice makes every key in values unknown -- this is the
// load-bearing fail-closed behavior for a nil paramsSchema (docker has no
// terraform module, so no derivable schema at all: the empty top-level
// `declared` set is what rejects everything).
//
// Object-kind fields are the ONLY ones recursed into, and only when they
// declare a non-empty fixed field set: a terraform map(T) (e.g.
// map(object({...})) -- yandex's subnets/vms) degrades in
// DeriveProviderParamsSchemapb to a permissive Object with zero declared
// fields (tfvars_schemapb.go's `case "map":`), and that shape is
// indistinguishable, at the schemapb.Schema level, from a "real" object()
// with no attributes -- so an empty field set is treated as "unvalidatable
// free-form value" (accept whatever is under it) rather than "reject
// everything under it", matching what schemapb.Bake itself would do for a
// non-strict, fieldless nested Schema. This is the same schemapb gap (no
// map-with-typed-values kind) DeriveProviderParamsSchemapb's own map(T)
// fallback has to live with -- see this package's SP-I1 report.
func unknownKeys(fields []*spb.Schema_Filed, values map[string]any) []string {
	declared := make(map[string]*spb.Schema_Filed, len(fields))
	for _, f := range fields {
		declared[f.GetName()] = f
	}

	var unknown []string
	for k, v := range values {
		f, ok := declared[k]
		if !ok {
			unknown = append(unknown, k)
			continue
		}
		obj := f.GetObject()
		if obj == nil || obj.GetSchema() == nil || len(obj.GetSchema().GetFields()) == 0 {
			continue // scalar/list/map-fallback/etc.: nothing further to recurse into
		}
		nested, ok := v.(map[string]any)
		if !ok {
			continue // type mismatch is schemapb.Bake's job to report, not this defense-in-depth check's
		}
		for _, u := range unknownKeys(obj.GetSchema().GetFields(), nested) {
			unknown = append(unknown, k+"."+u)
		}
	}
	return unknown
}
