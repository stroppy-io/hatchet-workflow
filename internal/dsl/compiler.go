// Package dsl is the facade for the stroppy YAML-DSL compiler: Compile runs
// every earlier task's stage (schema validation, ast decode, include
// resolution, domain-graph construction, contract checking, workflow-graph
// validation/expansion, and lowering) in sequence over one recipe bundle,
// accumulating diagnostics from every stage rather than stopping at the
// first problem found.
package dsl

import (
	"github.com/santhosh-tekuri/jsonschema/v6"
	spb "github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/contract"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/graph"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/include"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/lower"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/schema"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
)

// clusterFile/workflowFile are the two bundle-root files every recipe must
// carry (spec §2): Compile reports a missing one as an Error diagnostic
// rather than a Go error, matching every other problem this compiler finds.
const (
	clusterFile  = "cluster.yaml"
	workflowFile = "workflow.yaml"
)

// Input is everything Compile needs to compile one recipe bundle.
type Input struct {
	// Sources is the full in-memory recipe bundle: cluster.yaml,
	// workflow.yaml, and every components/**/component.yaml and files/**
	// entry an `include:` job or a rendered config may reference.
	Sources include.Sources
	// Provider is the decoded manifest of the provider the bundle selects
	// (cluster.yaml's "provider.use"). A nil Provider is not itself an
	// Error here — graph.Build reports that — since Compile has no
	// business second-guessing how its caller resolved the provider
	// reference.
	Provider *ast.ProviderManifest
	// Composed is the per-provider/per-document composed JSON Schema from
	// schema.Compose (Task 6), tightening $defs.providerParams/machineExt
	// to the selected provider's declared shape. nil means core.schema.json
	// validation only (schema.MustCore's $defs.cluster/$defs.workflow).
	Composed *jsonschema.Schema
	// Baked is the sealed launch-form values from a generated-launch-form
	// submission (SP-D), if any. nil means a non-form launch: no workflow
	// input or provider param overlay is applied. See
	// schema.SplitBakedValues/schema.ApplyBakedInputs for where its two
	// halves (workflow inputs, provider params) actually flow.
	Baked *spb.Baked
}

// Compile runs the full compiler pipeline over in, in stage order:
//
//  1. parse — both cluster.yaml and workflow.yaml are required; cluster.yaml
//     is schema-validated against Composed if non-nil (schema.Compose only
//     ever tightens core.schema.json's $defs.cluster — providerParams and
//     machineExt are both reachable exclusively from $defs.cluster, never
//     from $defs.workflow), else the core $defs.cluster subschema;
//     workflow.yaml is always validated against the core $defs.workflow
//     subschema, since no per-provider composition applies to it. Both are
//     ast-decoded only once schema validation for that pair passed cleanly.
//     A missing file or a schema violation is reported without ever
//     reaching ast decode for that pair, so a caller never sees a decode
//     diagnostic layered on top of a schema one for the same root cause.
//  2. include.Resolve — expands every `include:` job into the flat job DAG
//     plus the bound component set contract-check validates against.
//  3. graph.Build — the domain graph (per-group lowered disk types, CEL
//     views) for cluster against in.Provider.
//  4. contract.Check — capability/CEL cross-validation between the
//     provider, every bound component, and the domain graph.
//  5. graph.Validate then graph.Expand — job-graph shape (needs/cycles/
//     refs/expressions) and matrix instantiation.
//  6. lower.Lower — the validated, expanded result to a dslpb.CompiledPlan.
//
// Every stage's diagnostics are appended to the single returned diag.List;
// a stage does not run if the list produced so far HasErrors() (warnings
// never block a later stage). Compile returns a nil plan whenever the
// returned list HasErrors(), and a non-nil plan only once every stage has
// completed without an Error diagnostic.
func Compile(in Input) (*dslpb.CompiledPlan, diag.List) {
	cluster, wf, diags := parse(in)
	if diags.HasErrors() {
		return nil, diags
	}

	workflowInputs, _ := schema.SplitBakedValues(in.Baked) // nil-safe: SplitBakedValues(nil) returns nil, nil
	resolved, resolveDiags := include.Resolve(cluster, wf, in.Sources, workflowInputs)
	diags = append(diags, resolveDiags...)
	if diags.HasErrors() {
		return nil, diags
	}

	if err := schema.ApplyBakedInputs(resolved, in.Baked); err != nil {
		diags.Errorf("", diag.Pos{}, "apply baked inputs: %v", err)
		return nil, diags
	}

	dom, buildDiags := graph.Build(cluster, in.Provider)
	diags = append(diags, buildDiags...)
	if diags.HasErrors() {
		return nil, diags
	}

	checkDiags := contract.Check(dom, resolved.Components)
	diags = append(diags, checkDiags...)
	if diags.HasErrors() {
		return nil, diags
	}

	validateDiags := graph.Validate(resolved)
	diags = append(diags, validateDiags...)
	if diags.HasErrors() {
		return nil, diags
	}

	jobs := graph.Expand(resolved)

	plan, err := lower.Lower(cluster, dom, jobs, resolved.Components)
	if err != nil {
		diags.Errorf("", diag.Pos{}, "lower: %v", err)
		return nil, diags
	}

	return plan, diags
}

// parse reads cluster.yaml/workflow.yaml out of in.Sources, schema-validates
// each, and — only once both pass schema validation cleanly — ast-decodes
// each. Either file missing from in.Sources short-circuits before any
// validation/decode runs at all (there is nothing to validate); a schema
// violation in either file short-circuits before decode runs for *both*
// files, so a caller never sees an ast decode diagnostic for a document
// this stage already rejected.
func parse(in Input) (*ast.ClusterDoc, *ast.WorkflowDoc, diag.List) {
	var diags diag.List

	clusterSrc, haveCluster := in.Sources.Files[clusterFile]
	if !haveCluster {
		diags.Errorf(clusterFile, diag.Pos{}, "missing required file %s", clusterFile)
	}
	workflowSrc, haveWorkflow := in.Sources.Files[workflowFile]
	if !haveWorkflow {
		diags.Errorf(workflowFile, diag.Pos{}, "missing required file %s", workflowFile)
	}
	if !haveCluster || !haveWorkflow {
		return nil, nil, diags
	}

	diags = append(diags, schema.Validate(schema.Cluster, clusterFile, clusterSrc, in.Composed)...)
	// workflow.yaml has no composed counterpart (schema.Compose only ever
	// tightens $defs.cluster) — always validate it against the core
	// $defs.workflow subschema, passing nil regardless of in.Composed.
	diags = append(diags, schema.Validate(schema.Workflow, workflowFile, workflowSrc, nil)...)
	if diags.HasErrors() {
		return nil, nil, diags
	}

	providerKey := ""
	if in.Provider != nil {
		providerKey = in.Provider.Name
	}

	cluster, clusterDiags := ast.DecodeCluster(clusterFile, clusterSrc, providerKey)
	diags = append(diags, clusterDiags...)

	wf, wfDiags := ast.DecodeWorkflow(workflowFile, workflowSrc)
	diags = append(diags, wfDiags...)

	return cluster, wf, diags
}
