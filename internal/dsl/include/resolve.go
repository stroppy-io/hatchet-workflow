// Package include resolves the `include:` job syntax in a workflow.yaml
// document (Task 8 of the YAML-DSL compiler): a job of the form
//
//	jobs:
//	  ha:
//	    include: components/patroni
//	    inputs: { nodes: db }
//
// is replaced by the named component fragment's own Jobs, instantiated into
// the flat DAG under the "ha/" name prefix, with the fragment's inputs bound
// and type-checked against the component's declared InputSpecs.
//
// Resolve does no filesystem I/O: it reads component.yaml bytes purely from
// the in-memory Sources bundle passed in by the caller, and reports every
// problem it finds (missing files, unknown/missing inputs, bad input types,
// include cycles, job-name collisions) as diag.Diagnostic values rather than
// a Go error, matching the rest of the dsl compiler's decode/resolve stages.
package include

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
)

// Sources is the full in-memory recipe bundle an include may read from:
// Files is keyed by slash path relative to the bundle root (e.g.
// "components/etcd/component.yaml").
type Sources struct {
	Files map[string][]byte
}

// Resolved is the flattened result of expanding every include job in a
// workflow.
type Resolved struct {
	Cluster *ast.ClusterDoc
	// Jobs is every non-include job from wf, plus every job instantiated
	// from an include fragment (name-prefixed); include-jobs themselves do
	// not appear here.
	Jobs map[string]ast.Job
	// Components holds one entry per include instantiation (including
	// nested ones), for the later contract-check phase to validate
	// Requires/Provides against.
	Components []BoundComponent
}

// BoundComponent is one instantiated include: Name is the (possibly
// prefixed, for nested includes) job name that carried the include, Doc is
// the decoded component, and Inputs holds the bound input values (defaults
// applied, machine_group inputs kept as the group name string — contract
// checking resolves the group later).
type BoundComponent struct {
	Name   string
	Doc    *ast.ComponentDoc
	Inputs map[string]any
}

// Resolve expands every `include:` job in wf against src, producing a flat
// job DAG and the set of bound component instantiations. Every problem
// found is appended to the returned diag.List; a caller should check
// HasErrors() before trusting Resolved.
//
// Resolve applies exactly two narrow `${{ inputs.<name> }}` substitutions,
// both exact-match (whitespace-tolerant inside the braces) only — no CEL, no
// partial-string interpolation, everything else (cmd, when, with, nested
// inputs values that aren't a bare reference, ...) is left untouched for a
// later phase:
//
//   - a fragment job's On field is replaced with the bound input value
//     rendered as a string (see substituteOn). A reference to an
//     unknown/out-of-scope name is left as the literal template text,
//     unchanged — no diagnostic.
//
//   - a nested include job's own `inputs:` map values are replaced with the
//     *outer* fragment's already-bound, typed value for that name (verbatim,
//     not stringified) before that nested include's own bindInputs/
//     typecheck runs (see forwardInputs). This is what lets a wrapper
//     component forward one of its own bound inputs into a component it
//     includes, e.g. a fragment job `include: components/etcd, inputs:
//     {nodes: "${{ inputs.outer_nodes }}"}` inside a component that itself
//     declares `inputs: {outer_nodes: {type: machine_group}}`. Unlike the
//     On rule, a reference to an unknown outer input name here is an error.
//
// workflowInputs is the workflow document's own top-level bound input
// values (e.g. from a launch form's schema.SplitBakedValues) — it seeds the
// top-level fragmentCtx.boundInputs so a top-level job's `include:` inputs
// or On field can forward/substitute a `${{ inputs.<name> }}` reference the
// same way a nested include already can (see forwardInputs/substituteOn).
// nil (or empty) reproduces today's behavior: no top-level input is in
// scope, so any such reference is an error/left-as-is per the usual rules.
//
// Precondition: wf must already have passed ast.DecodeWorkflow without
// producing any error diagnostic — Resolve does not re-validate wf's own
// shape (mutually-exclusive Include/Service/On/Matrix, step actions, ...);
// it assumes the caller checked DecodeWorkflow's diag.List first.
func Resolve(cluster *ast.ClusterDoc, wf *ast.WorkflowDoc, src Sources, workflowInputs map[string]any) (*Resolved, diag.List) {
	r := &resolver{
		src:        src,
		cluster:    cluster,
		diags:      &diag.List{},
		out:        map[string]ast.Job{},
		jobOrigins: map[string]jobOrigin{},
	}
	r.expandFragment(wf.Jobs, "", nil, map[string]bool{}, fragmentCtx{
		path:        "workflow.yaml",
		boundInputs: workflowInputs,
	})

	return &Resolved{
		Cluster:    cluster,
		Jobs:       r.out,
		Components: r.comps,
	}, *r.diags
}

// resolver carries the state threaded through the recursive expansion:
// where component bytes come from (src), the cluster to validate
// machine_group inputs against (cluster), where diagnostics accumulate
// (diags), and the flat output being built (out/comps/jobOrigins).
type resolver struct {
	src     Sources
	cluster *ast.ClusterDoc
	diags   *diag.List
	out     map[string]ast.Job
	comps   []BoundComponent
	// jobOrigins mirrors out's keys, recording how each landed there
	// (Finding 1: needed to name both origins in a collision diagnostic).
	jobOrigins map[string]jobOrigin
}

// fragmentCtx is the per-level context threaded alongside a jobsMap through
// expandFragment/expandInclude: where diagnostics produced while expanding
// *this* jobsMap's own jobs are anchored (path/module), what to call this
// jobsMap's jobs when they collide with something else (includeOrigin, ""
// for the top-level workflow), and this level's own already-bound, typed
// inputs (boundInputs, nil at top level) available for a nested include to
// forward from (see Resolve's godoc).
type fragmentCtx struct {
	path          string
	module        string
	includeOrigin string
	boundInputs   map[string]any
}

// jobOrigin records how a job landed in r.out, purely so a name collision
// (Finding 1) can name both origins in its diagnostic: a job directly
// authored in the jobs map being expanded (Literal — either a workflow.yaml
// job or a component's own fragment job) versus a job that exists only
// because some include job's fragment produced it.
type jobOrigin struct {
	literal        bool
	includeJobName string // set when !literal: the (prefixed) name of the job that carried `include:`
}

// describe renders o as a phrase suitable for either side of a "%s collides
// with %s" collision message; name is the shared (prefixed) job name.
func (o jobOrigin) describe(name string) string {
	if o.literal {
		return fmt.Sprintf("job %q", name)
	}
	return fmt.Sprintf("job instantiated from include %q", o.includeJobName)
}

// claimJob records newJob under newName in r.out, unless that name was
// already claimed by an earlier write — Finding 1: a literal job and an
// include's fragment job can produce the same prefixed name (e.g. a literal
// "etcd/install" job alongside an include named "etcd" whose fragment has a
// job "install"). On collision it reports an error diagnostic naming both
// origins and keeps the first entry; it never overwrites r.out.
func (r *resolver) claimJob(newName string, newJob ast.Job, origin jobOrigin, filePath, module string) bool {
	if existing, taken := r.jobOrigins[newName]; taken {
		r.errorf(filePath, module, "%s collides with %s", origin.describe(newName), existing.describe(newName))
		return false
	}
	r.out[newName] = newJob
	r.jobOrigins[newName] = origin
	return true
}

// expandFragment expands one map of jobs — either the top-level workflow's
// Jobs (ctx.includeOrigin "", inputSubst nil) or a component fragment's own
// Jobs (ctx.includeOrigin the include job that produced this fragment,
// inputSubst the fragment's bound inputs rendered as strings for the narrow
// `On == "${{ inputs.x }}"` substitution rule) — into r.out.
//
// It returns:
//   - allNames: every job name instantiated from jobsMap (prefixed), used
//     by the caller to rewrite an *external* `needs: [includeJobName]`
//     reference into a dependency on every job the include produced.
//   - rootNames: the subset of allNames that have no need referring to
//     another job within this same jobsMap ("fragment roots"), used by the
//     caller to attach an include job's own `needs:` to the right place —
//     for a nested include, the roots bubble up from the deepest fragment
//     that actually owns them.
func (r *resolver) expandFragment(
	jobsMap map[string]ast.Job,
	prefix string,
	inputSubst map[string]string,
	visiting map[string]bool,
	ctx fragmentCtx,
) (allNames, rootNames []string) {
	includeJobs := map[string]ast.Job{}
	plainJobs := map[string]ast.Job{}
	for name, job := range jobsMap {
		if job.Include != "" {
			includeJobs[name] = job
		} else {
			plainJobs[name] = job
		}
	}

	// Expand every include job first (independent of siblings): each
	// produces its own instantiated names and fragment-root names,
	// needed below to rewrite any sibling's `needs:` that names it.
	nestedInstantiated := map[string][]string{}
	nestedRoots := map[string][]string{}
	for _, name := range sortedKeys(includeJobs) {
		inst, roots := r.expandInclude(name, includeJobs[name], prefix, visiting, ctx)
		nestedInstantiated[name] = inst
		nestedRoots[name] = roots
	}

	rewriteNeeds := func(needs []string) []string {
		if len(needs) == 0 {
			return nil
		}
		out := make([]string, 0, len(needs))
		for _, n := range needs {
			if inst, ok := nestedInstantiated[n]; ok {
				out = append(out, inst...)
				continue
			}
			out = append(out, prefix+n)
		}
		return out
	}

	isFragmentRoot := func(needs []string) bool {
		for _, n := range needs {
			if _, ok := jobsMap[n]; ok {
				return false
			}
		}
		return true
	}

	for _, name := range sortedKeys(plainJobs) {
		job := plainJobs[name]
		newJob := job
		newJob.Needs = rewriteNeeds(job.Needs)
		if inputSubst != nil {
			newJob.On = substituteOn(job.On, inputSubst)
		}
		newName := prefix + name
		origin := jobOrigin{literal: ctx.includeOrigin == ""}
		if !origin.literal {
			origin.includeJobName = ctx.includeOrigin
		}
		if !r.claimJob(newName, newJob, origin, ctx.path, ctx.module) {
			continue
		}
		allNames = append(allNames, newName)
		if isFragmentRoot(job.Needs) {
			rootNames = append(rootNames, newName)
		}
	}

	for _, name := range sortedKeys(includeJobs) {
		job := includeJobs[name]
		inst := nestedInstantiated[name]
		roots := nestedRoots[name]
		allNames = append(allNames, inst...)
		if isFragmentRoot(job.Needs) {
			rootNames = append(rootNames, roots...)
		}
		// The include job's own needs (declared at this level, referring
		// to siblings within jobsMap) become needs of every root job the
		// fragment it includes produced.
		rewritten := rewriteNeeds(job.Needs)
		for _, rn := range roots {
			rj := r.out[rn]
			rj.Needs = append(append([]string{}, rewritten...), rj.Needs...)
			r.out[rn] = rj
		}
	}

	sort.Strings(allNames)
	sort.Strings(rootNames)

	return allNames, rootNames
}

// expandInclude resolves one include job: reads its component.yaml from
// src, forwards any outer-scoped inputs (Resolve's second narrow
// substitution rule), binds/type-checks the result, records a
// BoundComponent, and recursively expands the component's own Jobs under
// prefix+name+"/". outerCtx is the context of the jobsMap this include job
// itself lives in — its path/module anchor forwarding-diagnostics, and its
// boundInputs is what a `${{ inputs.<name> }}` reference in this include's
// own Inputs resolves against.
func (r *resolver) expandInclude(name string, job ast.Job, prefix string, visiting map[string]bool, outerCtx fragmentCtx) (allNames, rootNames []string) {
	includePath := strings.TrimSuffix(strings.TrimSpace(job.Include), "/")
	componentPath := includePath + "/component.yaml"
	module := path.Base(includePath)
	jobName := prefix + name

	if visiting[includePath] {
		r.errorf(componentPath, module, "include cycle detected at %q (job %q)", includePath, jobName)
		return nil, nil
	}

	src, ok := r.src.Files[componentPath]
	if !ok {
		r.errorf(componentPath, module, "include %q (job %q): missing %s", includePath, jobName, componentPath)
		return nil, nil
	}

	compDoc, compDiags := ast.DecodeComponent(componentPath, src)
	for _, d := range compDiags {
		if d.Module == "" {
			d.Module = module
		}
		r.diags.Add(d)
	}
	if compDiags.HasErrors() {
		return nil, nil
	}

	forwardedInputs, fwdOK := r.forwardInputs(job.Inputs, outerCtx.boundInputs, jobName, outerCtx.path, outerCtx.module)
	if !fwdOK {
		return nil, nil
	}

	boundInputs, ok := r.bindInputs(jobName, componentPath, module, compDoc, forwardedInputs)
	if !ok {
		return nil, nil
	}

	r.comps = append(r.comps, BoundComponent{
		Name:   jobName,
		Doc:    compDoc,
		Inputs: boundInputs,
	})

	inputSubst := make(map[string]string, len(boundInputs))
	for k, v := range boundInputs {
		inputSubst[k] = fmt.Sprintf("%v", v)
	}

	childVisiting := make(map[string]bool, len(visiting)+1)
	for k := range visiting {
		childVisiting[k] = true
	}
	childVisiting[includePath] = true

	return r.expandFragment(compDoc.Jobs, jobName+"/", inputSubst, childVisiting, fragmentCtx{
		path:          componentPath,
		module:        module,
		includeOrigin: jobName,
		boundInputs:   boundInputs,
	})
}

// forwardInputs implements Resolve's second narrow substitution rule
// (Finding 2): every provided input value that is *exactly*
// (whitespace-tolerant) a "${{ inputs.<name> }}" reference is replaced with
// outerBoundInputs[<name>] — the enclosing fragment's own already-bound,
// typed value — before bindInputs/typecheck runs on this include. Any other
// value (not a string, or a string that isn't such a reference) passes
// through unchanged. A reference to a name absent from outerBoundInputs
// (including when outerBoundInputs is nil, i.e. there is no enclosing
// fragment to forward from at all) is an error diagnostic anchored to
// filePath/module — the *outer* fragment, not the nested include.
func (r *resolver) forwardInputs(provided, outerBoundInputs map[string]any, jobName, filePath, module string) (map[string]any, bool) {
	if len(provided) == 0 {
		return provided, true
	}

	ok := true
	out := make(map[string]any, len(provided))

	keys := make([]string, 0, len(provided))
	for k := range provided {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		val := provided[key]
		s, isStr := val.(string)
		if !isStr {
			out[key] = val
			continue
		}
		refName, isRef := parseInputRef(s)
		if !isRef {
			out[key] = val
			continue
		}
		outerVal, known := outerBoundInputs[refName]
		if !known {
			r.errorf(filePath, module, "include (job %q): input %q: unknown outer input %q", jobName, key, refName)
			ok = false
			continue
		}
		out[key] = outerVal
	}

	return out, ok
}

// bindInputs validates job-supplied inputs against compDoc's InputSpecs:
// unknown keys and bad types are errors, required inputs (no Default) must
// be present, and omitted inputs with a Default get it applied — run
// through the same checkInputType as any explicitly-provided value, so a
// malformed Default is reported instead of silently bound. It reports
// whether binding succeeded (no errors) alongside the bound value map.
func (r *resolver) bindInputs(jobName, filePath, module string, compDoc *ast.ComponentDoc, provided map[string]any) (map[string]any, bool) {
	ok := true
	bound := make(map[string]any, len(compDoc.Inputs))

	providedKeys := make([]string, 0, len(provided))
	for k := range provided {
		providedKeys = append(providedKeys, k)
	}
	sort.Strings(providedKeys)

	for _, key := range providedKeys {
		val := provided[key]
		spec, known := compDoc.Inputs[key]
		if !known {
			r.errorf(filePath, module, "include (job %q): unknown input %q", jobName, key)
			ok = false
			continue
		}
		checked, errMsg := checkInputType(r.cluster, spec, val)
		if errMsg != "" {
			r.errorf(filePath, module, "include (job %q): input %q: %s", jobName, key, errMsg)
			ok = false
			continue
		}
		bound[key] = checked
	}

	for _, key := range sortedInputSpecKeys(compDoc.Inputs) {
		if _, has := provided[key]; has {
			continue
		}
		spec := compDoc.Inputs[key]
		if spec.Default == nil {
			r.errorf(filePath, module, "include (job %q): missing required input %q", jobName, key)
			ok = false
			continue
		}
		checked, errMsg := checkInputType(r.cluster, spec, spec.Default)
		if errMsg != "" {
			r.errorf(filePath, module, "include (job %q): input %q: bad default: %s", jobName, key, errMsg)
			ok = false
			continue
		}
		bound[key] = checked
	}

	return bound, ok
}

// checkInputType validates val against spec's declared type, returning the
// value to bind (verbatim for scalars, the group name string for
// machine_group) or a non-empty error message. Input types not in this list
// (a component author typo, say) are accepted as-is — contract-check, not
// Resolve, is the place that would eventually reject an unknown type name.
func checkInputType(cluster *ast.ClusterDoc, spec ast.InputSpec, val any) (bound any, errMsg string) {
	switch spec.Type {
	case "machine_group":
		name, isStr := val.(string)
		if !isStr {
			return nil, fmt.Sprintf("expected machine group name (string), got %T", val)
		}
		if cluster == nil {
			return nil, fmt.Sprintf("machine_group input %q requires a cluster context", name)
		}
		if _, exists := cluster.Machines[name]; !exists {
			return nil, fmt.Sprintf("no such machine group %q", name)
		}
		return name, ""
	case "int":
		if n, isInt := val.(int); isInt {
			return n, ""
		}
		// A workflow-level `${{ inputs.<name> }}` value forwarded here from a
		// launch form (schema.SplitBakedValues -> Resolve's workflowInputs
		// parameter) is ALWAYS a float64, never a Go int: schemapb's own
		// documented numeric contract (form.go's "Numeric contract" doc
		// comment) decodes every numeric baked value as float64, including
		// int64-kind fields — google.protobuf.Value/structpb.Struct.AsMap()
		// has no separate integer representation. A plain `val.(int)` check
		// alone therefore accepts a static YAML-authored int (cluster.yaml/
		// component.yaml default, decoded by the YAML library as Go int) but
		// always rejects the exact same logical value when it arrived via a
		// baked launch-form submission — which is every "typed int input"
		// launched through the form path. Accept an integral float64 too,
		// coerced to int; a non-integral float64 (e.g. a form bug that let a
		// fractional value through) still errors.
		if f, isFloat := val.(float64); isFloat {
			if f == float64(int(f)) {
				return int(f), ""
			}
			return nil, fmt.Sprintf("expected int, got non-integral float64 %v", f)
		}
		return nil, fmt.Sprintf("expected int, got %T", val)
	case "string":
		if s, isStr := val.(string); isStr {
			return s, ""
		}
		return nil, fmt.Sprintf("expected string, got %T", val)
	case "bool":
		if b, isBool := val.(bool); isBool {
			return b, ""
		}
		return nil, fmt.Sprintf("expected bool, got %T", val)
	default:
		return val, ""
	}
}

// parseInputRef reports whether s is, once surrounding whitespace is
// trimmed, exactly a single "${{ inputs.<name> }}" placeholder
// (whitespace-tolerant inside the braces too), returning <name>. Both of
// Resolve's narrow substitution rules (Job.On via substituteOn, a nested
// include's `inputs:` values via forwardInputs — see Resolve's godoc) share
// this exact-match parsing; they differ only in what happens when the name
// turns out to be unknown in scope.
func parseInputRef(s string) (name string, ok bool) {
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "${{") || !strings.HasSuffix(trimmed, "}}") {
		return "", false
	}
	inner := strings.TrimSpace(trimmed[len("${{") : len(trimmed)-len("}}")])
	refName, isInputsRef := strings.CutPrefix(inner, "inputs.")
	if !isInputsRef {
		return "", false
	}
	return strings.TrimSpace(refName), true
}

// substituteOn implements Resolve's first narrow templating rule (see
// Resolve's godoc): a job's On field that is *exactly* a parseInputRef match
// is replaced with the bound input's value rendered as a string, so a
// fragment job lands on a real machine group at instantiation time. Any
// other On value — including one that merely contains such a reference
// alongside other text, or references a name unknown in inputSubst — is
// left as-is; unlike forwardInputs, an unknown name here is not an error.
func substituteOn(on string, inputSubst map[string]string) string {
	name, isRef := parseInputRef(on)
	if !isRef {
		return on
	}
	if v, known := inputSubst[name]; known {
		return v
	}
	return on
}

func (r *resolver) errorf(filePath, module, format string, args ...any) {
	r.diags.Add(diag.Diagnostic{
		Severity: diag.Error,
		Path:     filePath,
		Message:  fmt.Sprintf(format, args...),
		Module:   module,
	})
}

func sortedKeys(m map[string]ast.Job) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedInputSpecKeys(m map[string]ast.InputSpec) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
