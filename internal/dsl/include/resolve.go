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
// include cycles) as diag.Diagnostic values rather than a Go error, matching
// the rest of the dsl compiler's decode/resolve stages.
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
func Resolve(cluster *ast.ClusterDoc, wf *ast.WorkflowDoc, src Sources) (*Resolved, diag.List) {
	r := &resolver{
		src:     src,
		cluster: cluster,
		diags:   &diag.List{},
		out:     map[string]ast.Job{},
	}
	r.expandFragment(wf.Jobs, "", nil, map[string]bool{})

	return &Resolved{
		Cluster:    cluster,
		Jobs:       r.out,
		Components: r.comps,
	}, *r.diags
}

// resolver carries the state threaded through the recursive expansion:
// where component bytes come from (src), the cluster to validate
// machine_group inputs against (cluster), where diagnostics accumulate
// (diags), and the flat output being built (out/comps).
type resolver struct {
	src     Sources
	cluster *ast.ClusterDoc
	diags   *diag.List
	out     map[string]ast.Job
	comps   []BoundComponent
}

// expandFragment expands one map of jobs — either the top-level workflow's
// Jobs (prefix "", inputSubst nil) or a component fragment's own Jobs
// (prefix "<includeJobName>/...", inputSubst the fragment's bound inputs,
// rendered as strings for the narrow `On == "${{ inputs.x }}"` substitution
// rule) — into r.out.
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
		inst, roots := r.expandInclude(name, includeJobs[name], prefix, visiting)
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
		r.out[newName] = newJob
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
// src, binds/type-checks its inputs, records a BoundComponent, and
// recursively expands the component's own Jobs under prefix+name+"/".
func (r *resolver) expandInclude(name string, job ast.Job, prefix string, visiting map[string]bool) (allNames, rootNames []string) {
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

	boundInputs, ok := r.bindInputs(jobName, componentPath, module, compDoc, job.Inputs)
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

	return r.expandFragment(compDoc.Jobs, jobName+"/", inputSubst, childVisiting)
}

// bindInputs validates job-supplied inputs against compDoc's InputSpecs:
// unknown keys and bad types are errors, required inputs (no Default) must
// be present, and omitted inputs with a Default get it applied. It reports
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
		bound[key] = spec.Default
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
			return nil, fmt.Sprintf("no such machine group %q", name)
		}
		if _, exists := cluster.Machines[name]; !exists {
			return nil, fmt.Sprintf("no such machine group %q", name)
		}
		return name, ""
	case "int":
		if n, isInt := val.(int); isInt {
			return n, ""
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

// substituteOn implements the one narrow templating rule Resolve performs
// itself (everything else — CEL in `when:`, ${{ matrix.x }} in steps, ...
// — is left untouched for a later phase): a job's On field that is
// *exactly* (whitespace-tolerant) "${{ inputs.<name> }}" is replaced with
// the bound input's value rendered as a string, so a fragment job lands on
// a real machine group at instantiation time. Any other On value, including
// one that merely contains such a reference alongside other text, is left
// as-is.
func substituteOn(on string, inputSubst map[string]string) string {
	trimmed := strings.TrimSpace(on)
	if !strings.HasPrefix(trimmed, "${{") || !strings.HasSuffix(trimmed, "}}") {
		return on
	}
	inner := strings.TrimSpace(trimmed[len("${{") : len(trimmed)-len("}}")])
	name, isInputsRef := strings.CutPrefix(inner, "inputs.")
	if !isInputsRef {
		return on
	}
	name = strings.TrimSpace(name)
	if v, ok := inputSubst[name]; ok {
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
