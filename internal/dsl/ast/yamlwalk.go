package ast

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
)

// posOf converts a yaml.Node's Line/Column into a diag.Pos. A nil node
// yields the zero Pos.
func posOf(n *yaml.Node) diag.Pos {
	if n == nil {
		return diag.Pos{}
	}
	return diag.Pos{Line: n.Line, Col: n.Column}
}

// nodeKindName renders a yaml.Kind as a short human-readable word, used in
// diagnostic and error messages.
func nodeKindName(k yaml.Kind) string {
	switch k {
	case yaml.DocumentNode:
		return "document"
	case yaml.MappingNode:
		return "mapping"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.AliasNode:
		return "alias"
	default:
		return "unknown"
	}
}

// decodeMapping walks a YAML mapping node key by key. For every key present
// in handlers, the matching function is called with that key's value node;
// a non-nil error returned by a handler aborts the walk (handlers that want
// to keep collecting sibling diagnostics should record them directly, e.g.
// via a closed-over diag.List, and return nil).
//
// Any key absent from handlers is reported through onUnknown, which
// receives both the key node (for its exact source position) and the value
// node (so callers can, for example, capture the block into an ext/extra
// map instead of erroring). onUnknown may be nil, in which case unknown
// keys are silently skipped by the caller's own contract — callers wanting
// strict decoding must pass one that records a diagnostic.
//
// decodeMapping is intentionally generic (not tied to the cluster.yaml
// schema) so Tasks 3/4 can reuse it for the workflow/component documents.
func decodeMapping(
	node *yaml.Node,
	handlers map[string]func(*yaml.Node) error,
	onUnknown func(key string, keyNode, valNode *yaml.Node),
) error {
	if node == nil {
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected a mapping, got %s", nodeKindName(node.Kind))
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]

		handler, ok := handlers[keyNode.Value]
		if !ok {
			if onUnknown != nil {
				onUnknown(keyNode.Value, keyNode, valNode)
			}
			continue
		}
		if err := handler(valNode); err != nil {
			return fmt.Errorf("%s: %w", keyNode.Value, err)
		}
	}
	return nil
}

// decoderBase carries the state shared by every document decoder in this
// package (clusterDecoder, workflowDecoder, componentDecoder,
// providerDecoder): where diagnostics are anchored (path) and where they
// accumulate (diags). Document-specific decoders embed it by value and get
// errorf/decodeValue/alwaysOK/mapping for free instead of redefining this
// ~40-line wrapper boilerplate per document type.
type decoderBase struct {
	path  string
	diags *diag.List
}

func (d *decoderBase) errorf(n *yaml.Node, format string, args ...any) {
	d.diags.Errorf(d.path, posOf(n), format, args...)
}

// decodeValue calls value.Decode(target) and, on failure, records a
// diagnostic pointing at value's position instead of returning an error.
// It reports whether decoding succeeded.
func (d *decoderBase) decodeValue(value *yaml.Node, target any, field string) bool {
	if err := value.Decode(target); err != nil {
		d.errorf(value, "%s: %v", field, err)
		return false
	}
	return true
}

// alwaysOK adapts a handler that reports its own problems through the
// shared decoderBase (and so never fails the walk itself) to the
// func(*yaml.Node) error shape decodeMapping's handlers map requires.
func (d *decoderBase) alwaysOK(fn func(*yaml.Node)) func(*yaml.Node) error {
	return func(v *yaml.Node) error {
		fn(v)
		return nil
	}
}

// mapping walks node as a strict mapping: any key not in handlers is
// reported as an unknown-key diagnostic. Structural mismatches (node isn't
// actually a mapping) are also turned into a diagnostic here so callers
// never need to check decodeMapping's own error return.
func (d *decoderBase) mapping(node *yaml.Node, context string, handlers map[string]func(*yaml.Node) error) {
	err := decodeMapping(node, handlers, func(key string, keyNode, _ *yaml.Node) {
		d.errorf(keyNode, "%s: unknown key %q", context, key)
	})
	if err != nil {
		d.errorf(node, "%s: %v", context, err)
	}
}

// --- generic sub-document decoders reused across document types ---
//
// The methods below decode structures that are not tied to any single
// top-level document schema (cluster.yaml vs workflow.yaml vs
// component.yaml): a Service can appear standalone in cluster.yaml or
// nested under a component's `cluster:` fragment; a Job is identical in
// workflow.yaml and in a component's `jobs:` fragment. Defining them on
// decoderBase means every embedding decoder (clusterDecoder,
// workflowDecoder, componentDecoder, ...) gets the same decode logic for
// free through method promotion, instead of each document type
// re-implementing it.

// decodeServices walks a "services" mapping, whose keys are arbitrary
// service names, so it iterates node.Content directly rather than going
// through decodeMapping's known-key dispatch. A duplicate service name is
// reported as a diagnostic (anchored to the second occurrence's key node)
// and the first entry is preserved.
func (d *decoderBase) decodeServices(node *yaml.Node, out map[string]Service) {
	if node.Kind != yaml.MappingNode {
		d.errorf(node, "services: expected a mapping, got %s", nodeKindName(node.Kind))
		return
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		nameNode, svcNode := node.Content[i], node.Content[i+1]
		if _, dup := out[nameNode.Value]; dup {
			d.errorf(nameNode, "services: duplicate service name %q", nameNode.Value)
			continue
		}
		out[nameNode.Value] = d.decodeService(svcNode)
	}
}

func (d *decoderBase) decodeService(node *yaml.Node) Service {
	var svc Service
	d.mapping(node, "service", map[string]func(*yaml.Node) error{
		"on":      d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &svc.On, "service.on") }),
		"image":   d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &svc.Image, "service.image") }),
		"network": d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &svc.Network, "service.network") }),
		"volumes": d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &svc.Volumes, "service.volumes") }),
		"env":     d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &svc.Env, "service.env") }),
		"configs": d.alwaysOK(func(v *yaml.Node) { svc.Configs = d.decodeConfigs(v) }),
		"health": d.alwaysOK(func(v *yaml.Node) {
			health := d.decodeHealth(v)
			svc.Health = &health
		}),
	})
	return svc
}

func (d *decoderBase) decodeConfigs(node *yaml.Node) []ConfigFile {
	if node.Kind != yaml.SequenceNode {
		d.errorf(node, "service.configs: expected a sequence, got %s", nodeKindName(node.Kind))
		return nil
	}
	configs := make([]ConfigFile, 0, len(node.Content))
	for _, item := range node.Content {
		var cfg ConfigFile
		d.mapping(item, "service.configs[]", map[string]func(*yaml.Node) error{
			"template": d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &cfg.Template, "service.configs[].template") }),
			"dest":     d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &cfg.Dest, "service.configs[].dest") }),
		})
		configs = append(configs, cfg)
	}
	return configs
}

func (d *decoderBase) decodeHealth(node *yaml.Node) Health {
	var health Health
	d.mapping(node, "health", map[string]func(*yaml.Node) error{
		"http":    d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &health.HTTP, "health.http") }),
		"timeout": d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &health.Timeout, "health.timeout") }),
	})
	return health
}

// decodeJobs walks a "jobs" mapping, whose keys are arbitrary job names (not
// a fixed schema), so it iterates node.Content directly rather than going
// through decodeMapping's known-key dispatch. A duplicate job name is
// reported as a diagnostic (anchored to the second occurrence's key node)
// instead of silently overwriting the earlier entry.
func (d *decoderBase) decodeJobs(node *yaml.Node, out map[string]Job) {
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

func (d *decoderBase) decodeJob(node *yaml.Node) Job {
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

func (d *decoderBase) decodeSteps(node *yaml.Node) []Step {
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

func (d *decoderBase) decodeStep(node *yaml.Node) Step {
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

func (d *decoderBase) decodeWriteFile(node *yaml.Node) WriteFile {
	var wf WriteFile
	d.mapping(node, "jobs.steps[].write_file", map[string]func(*yaml.Node) error{
		"dest":     d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &wf.Dest, "jobs.steps[].write_file.dest") }),
		"content":  d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &wf.Content, "jobs.steps[].write_file.content") }),
		"template": d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &wf.Template, "jobs.steps[].write_file.template") }),
	})
	return wf
}

func (d *decoderBase) decodeFetch(node *yaml.Node) Fetch {
	var f Fetch
	d.mapping(node, "jobs.steps[].fetch", map[string]func(*yaml.Node) error{
		"url":  d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &f.URL, "jobs.steps[].fetch.url") }),
		"dest": d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &f.Dest, "jobs.steps[].fetch.dest") }),
	})
	return f
}

func (d *decoderBase) decodeWait(node *yaml.Node) Wait {
	var w Wait
	d.mapping(node, "jobs.steps[].wait", map[string]func(*yaml.Node) error{
		"http":    d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &w.HTTP, "jobs.steps[].wait.http") }),
		"timeout": d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &w.Timeout, "jobs.steps[].wait.timeout") }),
	})
	return w
}
