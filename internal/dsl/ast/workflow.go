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

	dec := &workflowDecoder{path: path, diags: &diags}
	doc := &WorkflowDoc{Jobs: map[string]Job{}}
	dec.decodeDoc(docNode, doc)

	return doc, diags
}

// workflowDecoder carries the state shared across all the nested decode
// helpers below: where diagnostics get attached (path) and where they
// accumulate (diags). Mirrors clusterDecoder from cluster.go.
type workflowDecoder struct {
	path  string
	diags *diag.List
}

func (d *workflowDecoder) errorf(n *yaml.Node, format string, args ...any) {
	d.diags.Errorf(d.path, posOf(n), format, args...)
}

// decodeValue calls value.Decode(target) and, on failure, records a
// diagnostic pointing at value's position instead of returning an error.
// It reports whether decoding succeeded.
func (d *workflowDecoder) decodeValue(value *yaml.Node, target any, field string) bool {
	if err := value.Decode(target); err != nil {
		d.errorf(value, "%s: %v", field, err)
		return false
	}
	return true
}

// alwaysOK adapts a handler that reports its own problems through the
// shared *workflowDecoder (and so never fails the walk itself) to the
// func(*yaml.Node) error shape decodeMapping's handlers map requires.
func (d *workflowDecoder) alwaysOK(fn func(*yaml.Node)) func(*yaml.Node) error {
	return func(v *yaml.Node) error {
		fn(v)
		return nil
	}
}

// mapping walks node as a strict mapping: any key not in handlers is
// reported as an unknown-key diagnostic. Structural mismatches (node isn't
// actually a mapping) are also turned into a diagnostic here so callers
// never need to check decodeMapping's own error return.
func (d *workflowDecoder) mapping(node *yaml.Node, context string, handlers map[string]func(*yaml.Node) error) {
	err := decodeMapping(node, handlers, func(key string, keyNode, _ *yaml.Node) {
		d.errorf(keyNode, "%s: unknown key %q", context, key)
	})
	if err != nil {
		d.errorf(node, "%s: %v", context, err)
	}
}

func (d *workflowDecoder) decodeDoc(node *yaml.Node, doc *WorkflowDoc) {
	d.mapping(node, "workflow", map[string]func(*yaml.Node) error{
		"jobs": d.alwaysOK(func(v *yaml.Node) { d.decodeJobs(v, doc.Jobs) }),
	})
}

// decodeJobs walks the "jobs" mapping, whose keys are arbitrary job names
// (not a fixed schema), so it iterates node.Content directly rather than
// going through decodeMapping's known-key dispatch. A duplicate job name is
// reported as a diagnostic (anchored to the second occurrence's key node)
// instead of silently overwriting the earlier entry.
func (d *workflowDecoder) decodeJobs(node *yaml.Node, out map[string]Job) {
	if node.Kind != yaml.MappingNode {
		d.errorf(node, "jobs: expected a mapping, got %s", nodeKindName(node.Kind))
		return
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		nameNode, jobNode := node.Content[i], node.Content[i+1]
		if _, dup := out[nameNode.Value]; dup {
			d.errorf(nameNode, "jobs: duplicate job name %q", nameNode.Value)
			continue
		}
		out[nameNode.Value] = d.decodeJob(jobNode)
	}
}

func (d *workflowDecoder) decodeJob(node *yaml.Node) Job {
	var job Job
	d.mapping(node, "jobs", map[string]func(*yaml.Node) error{
		"needs":   d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &job.Needs, "jobs.needs") }),
		"on":      d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &job.On, "jobs.on") }),
		"service": d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &job.Service, "jobs.service") }),
		"with":    d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &job.With, "jobs.with") }),
		"matrix":  d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &job.Matrix, "jobs.matrix") }),
		"when":    d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &job.When, "jobs.when") }),
		"steps":   d.alwaysOK(func(v *yaml.Node) { job.Steps = d.decodeSteps(v) }),
	})
	return job
}

func (d *workflowDecoder) decodeSteps(node *yaml.Node) []Step {
	if node.Kind != yaml.SequenceNode {
		d.errorf(node, "jobs.steps: expected a sequence, got %s", nodeKindName(node.Kind))
		return nil
	}
	steps := make([]Step, 0, len(node.Content))
	for _, item := range node.Content {
		steps = append(steps, d.decodeStep(item))
	}
	return steps
}

func (d *workflowDecoder) decodeStep(node *yaml.Node) Step {
	var step Step
	actions := 0

	d.mapping(node, "jobs.steps[]", map[string]func(*yaml.Node) error{
		"cmd": d.alwaysOK(func(v *yaml.Node) {
			if d.decodeValue(v, &step.Cmd, "jobs.steps[].cmd") {
				actions++
			}
		}),
		"write_file": d.alwaysOK(func(v *yaml.Node) {
			wf := d.decodeWriteFile(v)
			step.WriteFile = &wf
			actions++
		}),
		"fetch": d.alwaysOK(func(v *yaml.Node) {
			f := d.decodeFetch(v)
			step.Fetch = &f
			actions++
		}),
		"dir": d.alwaysOK(func(v *yaml.Node) {
			if d.decodeValue(v, &step.Dir, "jobs.steps[].dir") {
				actions++
			}
		}),
		"wait": d.alwaysOK(func(v *yaml.Node) {
			w := d.decodeWait(v)
			step.Wait = &w
			actions++
		}),
	})

	if actions != 1 {
		d.errorf(node, "jobs.steps[]: expected exactly one action (cmd, write_file, fetch, dir, wait), got %d", actions)
	}

	return step
}

func (d *workflowDecoder) decodeWriteFile(node *yaml.Node) WriteFile {
	var wf WriteFile
	d.mapping(node, "jobs.steps[].write_file", map[string]func(*yaml.Node) error{
		"dest":     d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &wf.Dest, "jobs.steps[].write_file.dest") }),
		"content":  d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &wf.Content, "jobs.steps[].write_file.content") }),
		"template": d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &wf.Template, "jobs.steps[].write_file.template") }),
	})
	return wf
}

func (d *workflowDecoder) decodeFetch(node *yaml.Node) Fetch {
	var f Fetch
	d.mapping(node, "jobs.steps[].fetch", map[string]func(*yaml.Node) error{
		"url":  d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &f.URL, "jobs.steps[].fetch.url") }),
		"dest": d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &f.Dest, "jobs.steps[].fetch.dest") }),
	})
	return f
}

func (d *workflowDecoder) decodeWait(node *yaml.Node) Wait {
	var w Wait
	d.mapping(node, "jobs.steps[].wait", map[string]func(*yaml.Node) error{
		"http":    d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &w.HTTP, "jobs.steps[].wait.http") }),
		"timeout": d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &w.Timeout, "jobs.steps[].wait.timeout") }),
	})
	return w
}
