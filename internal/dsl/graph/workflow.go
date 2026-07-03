package graph

import (
	"sort"
	"strings"

	"github.com/google/cel-go/cel"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/expr"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/include"
)

// Validate checks the flattened job graph of a resolved workflow (Task 9's
// include.Resolve output): that every `needs:` reference names an existing
// job, that the resulting DAG is acyclic, that every `on:`/`service:`
// reference names an existing cluster.yaml machine group/service, and that
// every "${{ ... }}" expression embedded in the workflow (and in
// r.Cluster.Services) typechecks against expr.ComponentEnv.
//
// Validate does not re-check anything include.Resolve already validated at
// include-expansion time (input binding/typing, include cycles, job-name
// collisions, ...) — it only looks at the flat r.Jobs map and r.Cluster.
//
// Every problem found is appended to the returned diag.List (not just the
// first); a caller should check HasErrors() before trusting the workflow.
//
// Diagnostics produced for a bad expression always carry the zero diag.Pos:
// expr.Extract works on the fully-decoded Go string (not the original
// yaml.Node), so there is no source line/column left to point at by the
// time a job/service reaches include.Resolved. Path names which document
// the field came from ("workflow.yaml" for job fields, "cluster.yaml" for
// cluster service fields) so a diagnostic is still locatable at the file
// level.
func Validate(r *include.Resolved) diag.List {
	var diags diag.List

	env, err := expr.ComponentEnv()
	if err != nil {
		diags.Errorf(workflowPath, diag.Pos{}, "building CEL environment: %v", err)
		return diags
	}

	validateJobNames(r, &diags)
	validateNeeds(r, &diags)
	validateCycle(r, &diags)
	validateRefs(r, &diags)
	validateJobExpressions(r, env, &diags)
	validateServiceExpressions(r, env, &diags)

	return diags
}

const (
	workflowPath = "workflow.yaml"
	clusterPath  = "cluster.yaml"
)

// validateJobNames reports an error for every job whose name contains
// reserved characters ([]=,) that are used to format matrix instance names.
func validateJobNames(r *include.Resolved, diags *diag.List) {
	reserved := map[rune]bool{'[': true, ']': true, '=': true, ',': true}
	for _, name := range sortedKeys(r.Jobs) {
		for _, ch := range name {
			if reserved[ch] {
				diags.Errorf(workflowPath, diag.Pos{}, "job name %q contains reserved characters ([],=) — reserved for matrix instance names", name)
				break
			}
		}
	}
}

// validateNeeds reports an error for every job whose `needs:` names a job
// absent from r.Jobs.
func validateNeeds(r *include.Resolved, diags *diag.List) {
	for _, name := range sortedKeys(r.Jobs) {
		for _, need := range r.Jobs[name].Needs {
			if _, ok := r.Jobs[need]; !ok {
				diags.Errorf(workflowPath, diag.Pos{}, "job %q: needs unknown job %q", name, need)
			}
		}
	}
}

// validateCycle runs Kahn's algorithm over r.Jobs and, if it can't fully
// drain the graph, reports the unprocessed (cyclic-or-downstream-of-cyclic)
// job names as a single error diagnostic, sorted for determinism.
//
// Edges for a `needs:` entry that names a job absent from r.Jobs are simply
// skipped here (validateNeeds already reports that separately) so a bad
// reference can't be mistaken for participating in a cycle.
func validateCycle(r *include.Resolved, diags *diag.List) {
	cyclic := detectCycle(r.Jobs)
	if len(cyclic) == 0 {
		return
	}
	diags.Errorf(workflowPath, diag.Pos{}, "cycle detected among jobs: %s", strings.Join(cyclic, ", "))
}

func detectCycle(jobs map[string]ast.Job) []string {
	indegree := make(map[string]int, len(jobs))
	dependents := make(map[string][]string, len(jobs))
	for name := range jobs {
		indegree[name] = 0
	}
	for _, name := range sortedKeys(jobs) {
		for _, need := range jobs[name].Needs {
			if _, ok := jobs[need]; !ok {
				continue
			}
			dependents[need] = append(dependents[need], name)
			indegree[name]++
		}
	}

	queue := make([]string, 0, len(jobs))
	for _, name := range sortedKeys(jobs) {
		if indegree[name] == 0 {
			queue = append(queue, name)
		}
	}

	processed := 0
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		processed++

		next := append([]string{}, dependents[n]...)
		sort.Strings(next)
		for _, m := range next {
			indegree[m]--
			if indegree[m] == 0 {
				queue = append(queue, m)
			}
		}
	}

	if processed == len(jobs) {
		return nil
	}

	cyclic := make([]string, 0, len(jobs)-processed)
	for name, deg := range indegree {
		if deg > 0 {
			cyclic = append(cyclic, name)
		}
	}
	sort.Strings(cyclic)
	return cyclic
}

// validateRefs reports an error for every job whose `on:`/`service:` names a
// machine group/service absent from r.Cluster.
func validateRefs(r *include.Resolved, diags *diag.List) {
	for _, name := range sortedKeys(r.Jobs) {
		job := r.Jobs[name]
		if job.On != "" {
			if r.Cluster == nil {
				diags.Errorf(workflowPath, diag.Pos{}, "job %q: on: %q but no cluster is available", name, job.On)
			} else if _, ok := r.Cluster.Machines[job.On]; !ok {
				diags.Errorf(workflowPath, diag.Pos{}, "job %q: on references unknown machine group %q", name, job.On)
			}
		}
		if job.Service != "" {
			if r.Cluster == nil {
				diags.Errorf(workflowPath, diag.Pos{}, "job %q: service: %q but no cluster is available", name, job.Service)
			} else if _, ok := r.Cluster.Services[job.Service]; !ok {
				diags.Errorf(workflowPath, diag.Pos{}, "job %q: service references unknown service %q", name, job.Service)
			}
		}
	}
}

// validateJobExpressions typechecks every "${{ ... }}" expression embedded
// in a job's steps/with (via expr.Extract) plus job.When, which is CEL in
// its entirety with no ${{ }} wrapper.
func validateJobExpressions(r *include.Resolved, env *cel.Env, diags *diag.List) {
	for _, name := range sortedKeys(r.Jobs) {
		job := r.Jobs[name]

		if job.When != "" {
			*diags = append(*diags, expr.Check(env, job.When, workflowPath, diag.Pos{})...)
		}

		for _, key := range sortedKeys(job.With) {
			checkEmbedded(env, job.With[key], workflowPath, diags)
		}

		for _, step := range job.Steps {
			checkEmbedded(env, step.Cmd, workflowPath, diags)
			checkEmbedded(env, step.Dir, workflowPath, diags)
			if step.WriteFile != nil {
				checkEmbedded(env, step.WriteFile.Content, workflowPath, diags)
				checkEmbedded(env, step.WriteFile.Dest, workflowPath, diags)
				checkEmbedded(env, step.WriteFile.Template, workflowPath, diags)
			}
			if step.Fetch != nil {
				checkEmbedded(env, step.Fetch.URL, workflowPath, diags)
				checkEmbedded(env, step.Fetch.Dest, workflowPath, diags)
			}
			if step.Wait != nil {
				checkEmbedded(env, step.Wait.HTTP, workflowPath, diags)
			}
		}
	}
}

// validateServiceExpressions typechecks every "${{ ... }}" expression
// embedded in r.Cluster.Services' Env values and string fields (On, Image,
// Network, Volumes entries, Configs[].Template/Dest, Health.HTTP).
func validateServiceExpressions(r *include.Resolved, env *cel.Env, diags *diag.List) {
	if r.Cluster == nil {
		return
	}
	for _, name := range sortedKeys(r.Cluster.Services) {
		svc := r.Cluster.Services[name]

		checkEmbedded(env, svc.On, clusterPath, diags)
		checkEmbedded(env, svc.Image, clusterPath, diags)
		checkEmbedded(env, svc.Network, clusterPath, diags)

		for _, key := range sortedKeys(svc.Env) {
			checkEmbedded(env, svc.Env[key], clusterPath, diags)
		}
		for _, v := range svc.Volumes {
			checkEmbedded(env, v, clusterPath, diags)
		}
		for _, cfg := range svc.Configs {
			checkEmbedded(env, cfg.Template, clusterPath, diags)
			checkEmbedded(env, cfg.Dest, clusterPath, diags)
		}
		if svc.Health != nil {
			checkEmbedded(env, svc.Health.HTTP, clusterPath, diags)
		}
	}
}

// checkEmbedded extracts every "${{ ... }}" expression from s and typechecks
// each one against env, appending any compile errors to diags.
func checkEmbedded(env *cel.Env, s, path string, diags *diag.List) {
	for _, e := range expr.Extract(s) {
		*diags = append(*diags, expr.Check(env, e, path, diag.Pos{})...)
	}
}

// sortedKeys returns m's keys sorted lexicographically, for deterministic
// iteration order wherever this file needs to walk a map (job names,
// service names, With/Env keys, ...).
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Expand replaces every job with a non-empty Matrix in r.Jobs with one
// instance per element of the matrix's cartesian product, named
// "<job>[k1=v1,k2=v2]" (keys sorted lexicographically, values in the order
// their list was declared) and carrying that assignment in the instance's
// MatrixValues (Matrix itself is nil'd on the instance). Jobs with no
// Matrix are copied through unchanged aside from `needs:` rewriting.
//
// needs rewriting: any `needs:` entry naming a matrix job M is replaced with
// every instance M produced (sorted). This applies uniformly regardless of
// whether the referencing job itself has a matrix: a plain job needing a
// matrix job needs all its instances, and — per the v1 semantics documented
// here — a matrix job needing another matrix job also has *every* instance
// need *every* instance of the dependency; there is no per-instance
// matrix-alignment (e.g. "bench[workload=insert]" does not selectively need
// only "load[phase=a]"). A `needs:` entry naming a plain (non-matrix) job is
// left unchanged.
//
// A matrix key with an empty value list (e.g. `matrix: { workload: [] }`)
// makes that job's cartesian product empty: the job produces zero instances
// and simply vanishes from Expand's output. Any other job's `needs:` on it
// still gets rewritten (to the — now empty — set of its instances), so the
// dependency silently disappears from the dependent's needs rather than
// being left dangling. Validate does not currently flag an empty matrix
// value list itself; a caller wanting to reject that shape outright would
// need to add that check separately.
//
// Expand's precondition is that r has already passed Validate; it does not
// re-validate needs/on/service/expression correctness, but it does not panic
// on a `needs:` entry naming a job absent from r.Jobs — such an entry has no
// matching instances map entry and so is left as-is, referring to nothing,
// same as it did in r.Jobs. Instance names are collision-free because Validate
// rejects user job names containing reserved characters ([],=).
func Expand(r *include.Resolved) map[string]ast.Job {
	names := sortedKeys(r.Jobs)

	// instances[name] holds the sorted instance names produced for a
	// matrix job (including an empty, non-nil slice for a matrix job whose
	// cartesian product is empty) — its presence as a key (not just a
	// non-empty value) is what lets rewriteNeeds distinguish "this was a
	// matrix job, rewrite to (possibly zero) instances" from "this is a
	// plain job, leave the needs entry alone".
	instances := make(map[string][]string, len(r.Jobs))
	for _, name := range names {
		job := r.Jobs[name]
		if len(job.Matrix) == 0 {
			continue
		}
		keys, combos := expandMatrix(job.Matrix)
		instNames := make([]string, 0, len(combos))
		for _, combo := range combos {
			instNames = append(instNames, instanceName(name, keys, combo))
		}
		sort.Strings(instNames)
		instances[name] = instNames
	}

	rewriteNeeds := func(needs []string) []string {
		if len(needs) == 0 {
			return nil
		}
		rewritten := make([]string, 0, len(needs))
		for _, need := range needs {
			if inst, ok := instances[need]; ok {
				rewritten = append(rewritten, inst...)
				continue
			}
			rewritten = append(rewritten, need)
		}
		sort.Strings(rewritten)
		return rewritten
	}

	out := make(map[string]ast.Job, len(r.Jobs))
	for _, name := range names {
		job := r.Jobs[name]

		if len(job.Matrix) == 0 {
			newJob := job
			newJob.Needs = rewriteNeeds(job.Needs)
			out[name] = newJob
			continue
		}

		keys, combos := expandMatrix(job.Matrix)
		for _, combo := range combos {
			instJob := job
			instJob.Matrix = nil
			instJob.MatrixValues = combo
			instJob.Needs = rewriteNeeds(job.Needs)
			out[instanceName(name, keys, combo)] = instJob
		}
	}

	return out
}

// expandMatrix computes matrix's cartesian product: keys is matrix's keys
// sorted lexicographically (the order instanceName brackets them in), combos
// is one map[string]string per element of the product, built by iterating
// each key's value list in declaration order. If any key has an empty value
// list, the product is empty (combos is nil) — the job it belongs to
// produces zero instances (see Expand's godoc).
func expandMatrix(matrix map[string][]string) (keys []string, combos []map[string]string) {
	keys = sortedKeys(matrix)

	combos = []map[string]string{{}}
	for _, k := range keys {
		values := matrix[k]
		if len(values) == 0 {
			return keys, nil
		}
		next := make([]map[string]string, 0, len(combos)*len(values))
		for _, combo := range combos {
			for _, v := range values {
				nc := make(map[string]string, len(combo)+1)
				for kk, vv := range combo {
					nc[kk] = vv
				}
				nc[k] = v
				next = append(next, nc)
			}
		}
		combos = next
	}
	return keys, combos
}

// instanceName renders a matrix job instance's name: jobName followed by
// "[k1=v1,k2=v2]" with keys in the order given (expandMatrix already sorts
// them) and values taken from combo.
func instanceName(jobName string, keys []string, combo map[string]string) string {
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + combo[k]
	}
	return jobName + "[" + strings.Join(parts, ",") + "]"
}
