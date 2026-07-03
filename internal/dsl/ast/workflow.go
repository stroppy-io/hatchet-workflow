package ast

import (
	"gopkg.in/yaml.v3"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
)

// WorkflowDoc is the decoded form of a workflow.yaml document.
type WorkflowDoc struct {
	Jobs map[string]Job
}

// Job describes one named job in a workflow.
type Job struct {
	Needs   []string
	On      string // machine group; пусто для service-jobs
	Service string // ссылка на services из cluster.yaml
	With    map[string]string
	Matrix  map[string][]string
	When    string // CEL
	Steps   []Step
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
	doc := &WorkflowDoc{Jobs: map[string]Job{}}
	dec.decodeDoc(docNode, doc)

	return doc, diags
}

// workflowDecoder embeds decoderBase (errorf/decodeValue/alwaysOK/mapping,
// and the generic decodeJobs/decodeSteps/... helpers); workflow.yaml has no
// state of its own beyond that, unlike clusterDecoder's providerKey.
type workflowDecoder struct {
	decoderBase
}

func (d *workflowDecoder) decodeDoc(node *yaml.Node, doc *WorkflowDoc) {
	d.mapping(node, "workflow", map[string]func(*yaml.Node) error{
		"jobs": d.alwaysOK(func(v *yaml.Node) { d.decodeJobs(v, doc.Jobs) }),
	})
}

// decodeJobs, decodeJob, decodeSteps, decodeStep, decodeWriteFile,
// decodeFetch and decodeWait are generic (not tied to workflow.yaml
// specifically — a component.yaml can nest the same `jobs:` shape) and live
// on decoderBase in yamlwalk.go; workflowDecoder inherits them through
// embedding.
