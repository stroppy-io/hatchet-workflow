package ast

import (
	"gopkg.in/yaml.v3"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
)

// WorkflowDoc is the decoded form of a workflow.yaml document.
type WorkflowDoc struct {
	// Inputs declares the workflow-level launch-form inputs (same InputSpec
	// shape as ComponentDoc.Inputs — see component.go) surfaced by the
	// DslService.ComposedSchema RPC's form schema. Optional: a workflow.yaml
	// with no "inputs:" key decodes to an empty (non-nil) map, matching
	// ComponentDoc's own zero-value convention.
	Inputs map[string]InputSpec
	Jobs   map[string]Job
}

// Job describes one named job in a workflow.
//
// Include/Inputs let a job instantiate a component fragment's Jobs (see
// internal/dsl/include) instead of running steps/a service directly:
//
//	jobs:
//	  ha:
//	    include: components/patroni
//	    inputs: { nodes: db }
//
// A job with Include set may also carry Needs (the fragment's root jobs
// inherit them) but must not combine Include with Service, Steps, On or
// Matrix — decodeJob reports a diagnostic if it does.
type Job struct {
	Needs   []string
	On      string // machine group; пусто для service-jobs
	Service string // ссылка на services из cluster.yaml
	With    map[string]string
	Matrix  map[string][]string
	When    string // CEL
	Steps   []Step
	Include string         // путь до каталога с component.yaml внутри бандла
	Inputs  map[string]any // inputs для include-джобы

	// MatrixValues carries one matrix key→value assignment for a job
	// instance produced by internal/dsl/graph.Expand from a Matrix job
	// (Matrix itself is nil'd on the instance). It is a runtime-only field:
	// DecodeWorkflow never sets it, and there is no "matrix_values:" YAML
	// key for it — decodeJob's handlers map below simply has no entry for
	// that key, so a document declaring one gets the same "unknown key"
	// diagnostic as any other typo.
	MatrixValues map[string]string
}

// Step is one action within a job's step list. Exactly one of Cmd,
// WriteFile, Fetch, Dir, Wait must be set; DecodeWorkflow reports a
// diagnostic if a step has zero or more than one action set.
type Step struct {
	Cmd       string
	WriteFile *WriteFile
	Fetch     *Fetch
	Dir       string
	Wait      *Wait
}

// WriteFile describes a step that renders content (optionally through a
// template) to a destination path.
type WriteFile struct {
	Dest     string
	Content  string
	Template string
}

// Fetch describes a step that downloads a URL to a destination path.
type Fetch struct {
	URL  string
	Dest string
}

// Wait describes a step that waits for an HTTP endpoint to become healthy.
type Wait struct {
	HTTP    string
	Timeout Duration
}

// DecodeWorkflow strictly decodes a workflow.yaml document. Every problem
// found (parse errors, unknown keys, malformed scalars, steps with the
// wrong number of actions) is reported as a diagnostic in the returned
// diag.List rather than as a Go error, so a caller sees every problem in the
// document in one pass instead of just the first one.
func DecodeWorkflow(path string, src []byte) (*WorkflowDoc, diag.List) {
	var diags diag.List

	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		diags.Errorf(path, diag.Pos{}, "yaml parse: %v", err)
		return nil, diags
	}
	if len(root.Content) == 0 {
		diags.Errorf(path, diag.Pos{}, "empty document")
		return nil, diags
	}
	docNode := root.Content[0]
	if docNode.Kind != yaml.MappingNode {
		diags.Errorf(path, posOf(docNode), "workflow document must be a mapping, got %s", nodeKindName(docNode.Kind))
		return nil, diags
	}

	dec := &workflowDecoder{decoderBase{path: path, diags: &diags}}
	doc := &WorkflowDoc{Inputs: map[string]InputSpec{}, Jobs: map[string]Job{}}
	dec.decodeDoc(docNode, doc)

	return doc, diags
}

// workflowDecoder embeds decoderBase (errorf/decodeValue/alwaysOK/mapping,
// and the generic decodeInputs/decodeJobs/decodeSteps/... helpers);
// workflow.yaml has no state of its own beyond that, unlike clusterDecoder's
// providerKey.
type workflowDecoder struct {
	decoderBase
}

func (d *workflowDecoder) decodeDoc(node *yaml.Node, doc *WorkflowDoc) {
	d.mapping(node, "workflow", map[string]func(*yaml.Node) error{
		"inputs": d.alwaysOK(func(v *yaml.Node) { d.decodeInputs(v, doc.Inputs) }),
		"jobs":   d.alwaysOK(func(v *yaml.Node) { d.decodeJobs(v, doc.Jobs) }),
	})
}

// decodeInputs, decodeInputSpec, decodeJobs, decodeJob, decodeSteps,
// decodeStep, decodeWriteFile, decodeFetch and decodeWait are generic (not
// tied to workflow.yaml specifically — a component.yaml can nest the same
// `inputs:`/`jobs:` shapes) and live on decoderBase in component.go/
// yamlwalk.go; workflowDecoder inherits them through embedding.
