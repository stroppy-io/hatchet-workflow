// Package contract implements the DSL compiler's contract-check stage (spec
// §6): the machine-checkable cross-validation between three authors of a
// recipe bundle — the provider ("what can I build"), the component/workload
// ("what do I need from the domain"), and the org's cluster/workflow
// ("what did I actually assemble") — without any solver: it flags
// violations, it never picks values for the author.
//
// Check runs three kinds of validation, all against the compiled graph.Domain
// and the flattened include.BoundComponent set produced by earlier compiler
// stages (spec §6's algorithm):
//
//  1. Capability matching: every component's `requires: capability: X` must
//     be closed by some `provides: X` in the bundle — either the provider
//     manifest's own Provides or any bound component's Provides.
//  2. Component/workload CEL predicates: every component's `requires: expr:`
//     is evaluated in expr.ComponentEnv, with `inputs` bound to the
//     component's own bound input values (a machine_group-typed input is
//     resolved to its expr.MachineGroupView from the domain graph), `target`
//     bound to the view of the component's single machine_group input (see
//     below), and `machines` bound to every named group in the domain.
//  3. Provider CEL predicates: every capabilities.machine rule and
//     capabilities.constraints requirement is evaluated in
//     expr.ProviderEnv, once per machine of every named group in the domain,
//     with `machine` bound to that machine's expr.ProviderMachineView.
//
// # The `target` convention
//
// A component/workload's requires:expr may reference a bare `target` (see
// the workload formula example in spec §6) without saying which of its
// inputs it means. Check resolves this from the component's own declared
// Inputs, not from any hint in the CEL text: if the component declares
// exactly one machine_group-typed input, `target` is bound to that input's
// resolved group view; if it declares zero or more than one, `target` is
// left unbound entirely — a requires:expr that references `target` in that
// case surfaces as a normal Eval error diagnostic (see evalRequirementExpr),
// not a distinguished "no target" diagnostic. Component authors with more
// than one machine_group input must address them by their own input name via
// `inputs.<name>`, not `target`.
//
// # The ext-based provider idiom for domain fields with no ProviderMachineView field
//
// expr.ProviderMachineView intentionally exposes only the domain machine
// fields (ip/ram_gb/cpu/disk_gb/disks) plus the provider's own opaque `ext`
// map (see expr.env.go's doc) — it has no `disk.type` field. A provider
// capabilities.constraints rule that needs to react to a group's (lowered)
// disk type — the spec §6 example `machine.disk.type == 'local-ssd' ? ...`
// — must instead read it off ext, since Check populates
// ProviderMachineView.Ext with the group's GroupState.Ext plus
// (GroupState.LoweredDiskType, when non-empty, under the "disk_type" key —
// overwriting any ext["disk_type"] the group itself set, since it is the
// final lowered value a provider constrains on):
//
//	capabilities:
//	  constraints:
//	    - expr: "machine.ext.disk_type == 'local-ssd' ? machine.cpu >= 8 : true"
//	      message: "local-ssd только от 8 vCPU"
package contract

import (
	"fmt"
	"sort"

	"github.com/google/cel-go/cel"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/expr"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/graph"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/include"
)

// Check validates dom (the compiled domain graph, spec §5) against comps
// (the bound component instantiations produced by include.Resolve, spec §6)
// and dom.Provider's own capabilities, returning every problem found as a
// diag.List rather than stopping at the first one. See the package doc for
// the full algorithm.
//
// A nil dom is reported as a single defensive Error diagnostic (a caller
// reaching Check with no domain graph at all is a programmatic
// inconsistency in the compiler pipeline, not a recipe-authoring mistake);
// comps may be nil or empty (no bundled components — only the provider's own
// capabilities are checked).
func Check(dom *graph.Domain, comps []include.BoundComponent) diag.List {
	var diags diag.List

	if dom == nil {
		diags.Add(diag.Diagnostic{
			Severity: diag.Error,
			Message:  "contract-check: nil domain graph",
		})
		return diags
	}

	compEnv, err := expr.ComponentEnv()
	if err != nil {
		diags.Add(diag.Diagnostic{
			Severity: diag.Error,
			Message:  fmt.Sprintf("contract-check: build ComponentEnv: %v", err),
		})
		return diags
	}
	provEnv, err := expr.ProviderEnv()
	if err != nil {
		diags.Add(diag.Diagnostic{
			Severity: diag.Error,
			Message:  fmt.Sprintf("contract-check: build ProviderEnv: %v", err),
		})
		return diags
	}

	provides := collectProvides(dom, comps)
	machinesVar := buildMachinesVar(dom)

	for _, comp := range comps {
		checkComponent(compEnv, dom, machinesVar, comp, provides, &diags)
	}

	if dom.Provider != nil {
		checkProviderCapabilities(provEnv, dom, &diags)
	}

	return diags
}

// collectProvides gathers step 1 of the algorithm: every capability name
// anyone in the bundle provides — the provider manifest's own Provides, plus
// every bound component's Provides. A capability offered by both (spec
// allows a component to re-provide something the provider already gives the
// domain, e.g. a provider that bundles its own etcd) dedups harmlessly
// through the set.
func collectProvides(dom *graph.Domain, comps []include.BoundComponent) map[string]bool {
	provides := map[string]bool{}
	if dom.Provider != nil {
		for _, p := range dom.Provider.Provides {
			provides[p] = true
		}
	}
	for _, c := range comps {
		if c.Doc == nil {
			continue
		}
		for _, p := range c.Doc.Provides {
			provides[p] = true
		}
	}
	return provides
}

// buildMachinesVar builds the `machines` CEL binding shared by every
// component's requires:expr evaluation: every named group in the domain,
// keyed by group name, mapped to its expr.MachineGroupView.
func buildMachinesVar(dom *graph.Domain) map[string]any {
	machines := make(map[string]any, len(dom.Groups))
	for name, gs := range dom.Groups {
		machines[name] = gs.View
	}
	return machines
}

// checkComponent runs steps 2 (capability matching) and 3 (CEL predicates)
// of the algorithm for one bound component, in its own Requires declaration
// order.
func checkComponent(
	env *cel.Env,
	dom *graph.Domain,
	machinesVar map[string]any,
	comp include.BoundComponent,
	provides map[string]bool,
	diags *diag.List,
) {
	if comp.Doc == nil {
		// A BoundComponent with no decoded document is a programmatic
		// inconsistency from an earlier compiler stage (include.Resolve
		// never produces one) — nothing to check here.
		return
	}

	vars := componentVars(dom, comp, machinesVar, diags)

	for _, req := range comp.Doc.Requires {
		switch {
		case req.Capability != "":
			checkCapabilityRequirement(req, comp.Name, provides, diags)
		case req.Expr != "":
			evalRequirementExpr(env, req, comp.Name, vars, diags)
		}
	}
}

// checkCapabilityRequirement implements step 2: req.Capability must be
// closed by the bundle-wide provides set collected in collectProvides.
func checkCapabilityRequirement(req ast.Requirement, componentName string, provides map[string]bool, diags *diag.List) {
	if provides[req.Capability] {
		return
	}
	diags.Add(diag.Diagnostic{
		Severity: diag.Error,
		Module:   componentName,
		Message:  fmt.Sprintf("component %q requires capability %q, which nothing in the bundle provides", componentName, req.Capability),
	})
}

// componentVars builds the ComponentEnv variable bindings for one
// component's requires:expr evaluations (step 3): `inputs` (machine_group
// values resolved to their expr.MachineGroupView), `machines` (shared across
// all components), and `target` (see the package doc's "target convention").
func componentVars(dom *graph.Domain, comp include.BoundComponent, machinesVar map[string]any, diags *diag.List) map[string]any {
	inputs := make(map[string]any, len(comp.Inputs))
	for _, key := range sortedAnyKeys(comp.Inputs) {
		val := comp.Inputs[key]
		spec, known := comp.Doc.Inputs[key]
		if !known || spec.Type != "machine_group" {
			inputs[key] = val
			continue
		}
		groupName, isStr := val.(string)
		if !isStr {
			diags.Add(diag.Diagnostic{
				Severity: diag.Error,
				Module:   comp.Name,
				Message:  fmt.Sprintf("component %q: input %q: bound machine_group value is not a group name string (got %T)", comp.Name, key, val),
			})
			continue
		}
		group, exists := dom.Groups[groupName]
		if !exists {
			// A machine_group input bound to a group name absent from the
			// domain graph should have been caught at include-resolve time
			// (checkInputType validates the name against cluster.Machines);
			// reaching here is a programmatic inconsistency between stages,
			// reported defensively rather than panicking.
			diags.Add(diag.Diagnostic{
				Severity: diag.Error,
				Module:   comp.Name,
				Message:  fmt.Sprintf("component %q: input %q: machine group %q not found in domain graph", comp.Name, key, groupName),
			})
			continue
		}
		inputs[key] = group.View
	}

	vars := map[string]any{
		"inputs":   inputs,
		"machines": machinesVar,
	}

	if targetKey, ok := singleMachineGroupInput(comp.Doc.Inputs); ok {
		if view, resolved := inputs[targetKey].(expr.MachineGroupView); resolved {
			vars["target"] = view
		}
		// Unresolved (bad group name, etc.) — already diagnosed above by
		// the inputs loop; leave `target` unbound rather than double-report.
	}

	return vars
}

// singleMachineGroupInput implements the "target" convention: it returns the
// one input name declared machine_group-typed, and true, only when the
// component declares exactly one such input. Zero or multiple machine_group
// inputs leave target unbound (documented in the package doc).
func singleMachineGroupInput(specs map[string]ast.InputSpec) (name string, ok bool) {
	found := ""
	count := 0
	for key, spec := range specs {
		if spec.Type == "machine_group" {
			found = key
			count++
		}
	}
	if count != 1 {
		return "", false
	}
	return found, true
}

// evalRequirementExpr implements step 3's per-expression evaluation: Eval
// req.Expr in env against vars; a non-bool result or a compile/runtime error
// is reported with the CEL error text, and a false result is reported with
// req.Message (falling back to the expression text itself when the author
// left Message empty).
func evalRequirementExpr(env *cel.Env, req ast.Requirement, module string, vars map[string]any, diags *diag.List) {
	result, err := expr.Eval(env, req.Expr, vars)
	if err != nil {
		diags.Add(diag.Diagnostic{
			Severity: diag.Error,
			Module:   module,
			Message:  fmt.Sprintf("%s: requires expr %q: %v", module, req.Expr, err),
		})
		return
	}
	ok, isBool := result.(bool)
	if !isBool {
		diags.Add(diag.Diagnostic{
			Severity: diag.Error,
			Module:   module,
			Message:  fmt.Sprintf("%s: requires expr %q: expected a bool result, got %T", module, req.Expr, result),
		})
		return
	}
	if ok {
		return
	}
	msg := req.Message
	if msg == "" {
		msg = req.Expr
	}
	diags.Add(diag.Diagnostic{Severity: diag.Error, Module: module, Message: msg})
}

// checkProviderCapabilities implements step 4: capabilities.machine rules
// and capabilities.constraints requirements, evaluated once per machine of
// every named group in the domain (sorted for determinism).
func checkProviderCapabilities(env *cel.Env, dom *graph.Domain, diags *diag.List) {
	provider := dom.Provider
	capKeys := sortedCapRuleKeys(provider.Capabilities.Machine)

	for _, groupName := range sortedGroupStateKeys(dom.Groups) {
		gs := dom.Groups[groupName]
		if len(gs.View.Machines) == 0 {
			// Nothing to validate for an empty group.
			continue
		}
		for _, capKey := range capKeys {
			checkCapRule(env, gs, groupName, capKey, provider.Capabilities.Machine[capKey], provider, diags)
		}
		for _, req := range provider.Capabilities.Constraints {
			checkProviderConstraint(env, gs, groupName, req, provider, diags)
		}
	}
}

// checkCapRule validates one capabilities.machine field rule for one group:
// its Enum branch (if set) against the group's domain value, and its Expr
// branch (if set) per machine, both independently — a rule may set either,
// both, or (per ast decoding) neither.
func checkCapRule(env *cel.Env, gs graph.GroupState, groupName, capKey string, rule ast.CapRule, provider *ast.ProviderManifest, diags *diag.List) {
	if len(rule.Enum) > 0 {
		checkCapEnum(gs, groupName, capKey, rule.Enum, provider, diags)
	}
	if rule.Expr != "" {
		checkCapExpr(env, gs, groupName, capKey, rule.Expr, provider, diags)
	}
}

// checkCapEnum implements the Enum branch: mapping a capability key to the
// group's corresponding domain value is a closed, explicit list — today just
// "cpu" → the group's per-machine CPU (uniform across every machine in a
// group, see graph.buildView). A key this compiler doesn't know how to map
// to a domain value is reported as a Warning (not an Error) and skipped,
// per the brief: an unrecognized capabilities.machine key does not fail
// compilation, since the provider may simply be documenting a capability
// this compiler generation doesn't understand yet.
func checkCapEnum(gs graph.GroupState, groupName, capKey string, enum []any, provider *ast.ProviderManifest, diags *diag.List) {
	var domainValue any
	switch capKey {
	case "cpu":
		domainValue = gs.View.Machines[0].CPU
	default:
		diags.Add(diag.Diagnostic{
			Severity: diag.Warning,
			Module:   provider.Name,
			Message:  fmt.Sprintf("provider %s: capabilities.machine: unknown capability key %q, skipping enum check", provider.Name, capKey),
		})
		return
	}
	if enumContainsNumeric(enum, domainValue) {
		return
	}
	diags.Add(diag.Diagnostic{
		Severity: diag.Error,
		Module:   provider.Name,
		Message:  fmt.Sprintf("provider %s: group %q: %s=%v not in allowed values %v", provider.Name, groupName, capKey, domainValue, enum),
	})
}

// checkCapExpr implements the Expr branch of a capabilities.machine rule:
// evaluated in ProviderEnv per machine of the group, same as a
// capabilities.constraints requirement, but with a synthesized message
// (CapRule carries no author Message field).
func checkCapExpr(env *cel.Env, gs graph.GroupState, groupName, capKey, exprStr string, provider *ast.ProviderManifest, diags *diag.List) {
	evalPerMachine(env, gs, groupName, exprStr, provider, diags, func() string {
		return fmt.Sprintf("provider %s: group %q: capability %q expr %q not satisfied", provider.Name, groupName, capKey, exprStr)
	})
}

// checkProviderConstraint implements the capabilities.constraints branch of
// step 4: a Requirement (with an author Message, unlike CapRule), evaluated
// in ProviderEnv per machine of the group.
func checkProviderConstraint(env *cel.Env, gs graph.GroupState, groupName string, req ast.Requirement, provider *ast.ProviderManifest, diags *diag.List) {
	if req.Expr == "" {
		return
	}
	evalPerMachine(env, gs, groupName, req.Expr, provider, diags, func() string {
		msg := req.Message
		if msg == "" {
			msg = req.Expr
		}
		return msg
	})
}

// evalPerMachine evaluates exprStr in ProviderEnv once per machine of gs,
// with `machine` bound to that machine's expr.ProviderMachineView (see
// providerMachineView). It stops at the first machine that fails — either a
// compile/runtime error, a non-bool result, or a false result — since every
// machine in a group shares the same view today (graph.buildView produces
// per-group-uniform machines), so continuing would only produce duplicate
// diagnostics.
func evalPerMachine(
	env *cel.Env,
	gs graph.GroupState,
	groupName, exprStr string,
	provider *ast.ProviderManifest,
	diags *diag.List,
	falseMessage func() string,
) {
	for _, m := range gs.View.Machines {
		pv, err := providerMachineView(m, gs)
		if err != nil {
			diags.Add(diag.Diagnostic{
				Severity: diag.Error,
				Module:   provider.Name,
				Message:  fmt.Sprintf("provider %s: group %q: build provider machine view: %v", provider.Name, groupName, err),
			})
			return
		}

		result, err := expr.Eval(env, exprStr, map[string]any{"machine": pv})
		if err != nil {
			diags.Add(diag.Diagnostic{
				Severity: diag.Error,
				Module:   provider.Name,
				Message:  fmt.Sprintf("provider %s: group %q: expr %q: %v", provider.Name, groupName, exprStr, err),
			})
			return
		}
		ok, isBool := result.(bool)
		if !isBool {
			diags.Add(diag.Diagnostic{
				Severity: diag.Error,
				Module:   provider.Name,
				Message:  fmt.Sprintf("provider %s: group %q: expr %q: expected a bool result, got %T", provider.Name, groupName, exprStr, result),
			})
			return
		}
		if !ok {
			diags.Add(diag.Diagnostic{Severity: diag.Error, Module: provider.Name, Message: falseMessage()})
			return
		}
	}
}

// providerMachineView builds the expr.ProviderMachineView a provider's CEL
// sees for one machine of a group: the domain fields verbatim from m, plus
// Ext built from gs.Ext with gs.LoweredDiskType layered in under
// "disk_type" (see the package doc's ext-based idiom) whenever the group has
// a non-empty lowered disk type — this is the *final* lowered value a
// provider constrains on, so it always wins over any ext["disk_type"] the
// group itself carried into the domain graph.
func providerMachineView(m expr.MachineView, gs graph.GroupState) (expr.ProviderMachineView, error) {
	ext := make(map[string]any, len(gs.Ext)+1)
	for k, v := range gs.Ext {
		ext[k] = v
	}
	if gs.LoweredDiskType != "" {
		ext["disk_type"] = gs.LoweredDiskType
	}

	extStruct, err := expr.NewProviderExt(ext)
	if err != nil {
		return expr.ProviderMachineView{}, err
	}

	return expr.ProviderMachineView{
		IP:     m.IP,
		RAMGb:  m.RAMGb,
		CPU:    m.CPU,
		DiskGb: m.DiskGb,
		Disks:  m.Disks,
		Ext:    extStruct,
	}, nil
}

// enumContainsNumeric reports whether v is present in enum, comparing
// numeric values (int/int32/int64/float32/float64, in any combination) by
// their float64 magnitude rather than by Go type — a YAML-decoded enum list
// holds plain `int` values (see ast.CapRule.Enum), while a domain value
// flowing through an expr view (e.g. MachineView.CPU) is int64; these must
// compare equal. Non-numeric values (e.g. a future string-enum capability)
// fall back to direct equality.
func enumContainsNumeric(enum []any, v any) bool {
	vf, vIsNum := toFloat64(v)
	for _, e := range enum {
		if ef, eIsNum := toFloat64(e); vIsNum && eIsNum {
			if ef == vf {
				return true
			}
			continue
		}
		if e == v {
			return true
		}
	}
	return false
}

// toFloat64 converts v to a float64 and reports success, for every Go
// numeric type enumContainsNumeric might see on either side of a comparison.
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

// sortedAnyKeys returns m's keys sorted, for deterministic diagnostic order
// when walking a component's bound Inputs (map[string]any).
func sortedAnyKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sortedGroupStateKeys returns m's keys sorted, for deterministic iteration
// over dom.Groups.
func sortedGroupStateKeys(m map[string]graph.GroupState) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sortedCapRuleKeys returns m's keys sorted, for deterministic iteration
// over provider.Capabilities.Machine.
func sortedCapRuleKeys(m map[string]ast.CapRule) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
