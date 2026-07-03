package ast

import (
	"gopkg.in/yaml.v3"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
)

// ComponentDoc is the decoded form of a component.yaml document (also used,
// with the same shape, for workload fragments — see spec §6).
type ComponentDoc struct {
	Inputs   map[string]InputSpec
	Requires []Requirement
	Provides []string
	// Cluster holds the "cluster:" fragment (v1: services only) merged into
	// the owning cluster.yaml when the component is attached.
	Cluster *ClusterFragment
	Jobs    map[string]Job
}

// InputSpec describes one named input a component/workload accepts. It may
// be written as a bare scalar type name ("iterations: int") or as a mapping
// with an optional default ("row_bytes: { type: int, default: 512 }").
type InputSpec struct {
	Type    string
	Default any
}

// Requirement is one entry of a "requires:" or "capabilities.constraints:"
// list. Exactly one of Expr and Capability must be set; DecodeComponent and
// DecodeProviderManifest report a diagnostic if a requirement sets both or
// neither.
type Requirement struct {
	Expr       string
	Capability string
	Message    string
}

// ClusterFragment is the "cluster:" block a component/workload may carry to
// merge into the owning cluster.yaml. v1 only supports merging services.
type ClusterFragment struct {
	Services map[string]Service
}

// DecodeComponent strictly decodes a component.yaml document (or a
// workload's component.yaml — same shape). Every problem found (parse
// errors, unknown keys, malformed scalars, requirements with the wrong
// number of Expr/Capability set) is reported as a diagnostic in the
// returned diag.List rather than as a Go error, so a caller sees every
// problem in the document in one pass instead of just the first one.
func DecodeComponent(path string, src []byte) (*ComponentDoc, diag.List) {
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
		diags.Errorf(path, posOf(docNode), "component document must be a mapping, got %s", nodeKindName(docNode.Kind))
		return nil, diags
	}

	dec := &componentDecoder{decoderBase{path: path, diags: &diags}}
	doc := &ComponentDoc{
		Inputs: map[string]InputSpec{},
		Jobs:   map[string]Job{},
	}
	dec.decodeDoc(docNode, doc)

	return doc, diags
}

// componentDecoder embeds decoderBase, inheriting errorf/decodeValue/
// alwaysOK/mapping plus the generic decodeServices (for Cluster) and
// decodeJobs (for Jobs) helpers shared with cluster.go/workflow.go.
type componentDecoder struct {
	decoderBase
}

func (d *componentDecoder) decodeDoc(node *yaml.Node, doc *ComponentDoc) {
	d.mapping(node, "component", map[string]func(*yaml.Node) error{
		"inputs":   d.alwaysOK(func(v *yaml.Node) { d.decodeInputs(v, doc.Inputs) }),
		"requires": d.alwaysOK(func(v *yaml.Node) { doc.Requires = d.decodeRequirements(v, "requires") }),
		"provides": d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &doc.Provides, "provides") }),
		"cluster": d.alwaysOK(func(v *yaml.Node) {
			cf := d.decodeClusterFragment(v)
			doc.Cluster = &cf
		}),
		"jobs": d.alwaysOK(func(v *yaml.Node) { d.decodeJobs(v, doc.Jobs) }),
	})
}

// decodeInputs walks the "inputs" mapping, whose keys are arbitrary input
// names (not a fixed schema), so it iterates node.Content directly rather
// than going through decodeMapping's known-key dispatch. A duplicate input
// name is reported as a diagnostic (anchored to the second occurrence's key
// node) and the first entry is preserved.
func (d *componentDecoder) decodeInputs(node *yaml.Node, out map[string]InputSpec) {
	if node.Kind != yaml.MappingNode {
		d.errorf(node, "inputs: expected a mapping, got %s", nodeKindName(node.Kind))
		return
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		nameNode, specNode := node.Content[i], node.Content[i+1]
		if _, dup := out[nameNode.Value]; dup {
			d.errorf(nameNode, "inputs: duplicate input name %q", nameNode.Value)
			continue
		}
		out[nameNode.Value] = d.decodeInputSpec(specNode)
	}
}

// decodeInputSpec accepts either a bare scalar type name ("int") or a
// mapping with type/default fields ("{ type: int, default: 512 }").
func (d *componentDecoder) decodeInputSpec(node *yaml.Node) InputSpec {
	var spec InputSpec
	if node.Kind == yaml.ScalarNode {
		d.decodeValue(node, &spec.Type, "inputs[]")
		return spec
	}
	d.mapping(node, "inputs[]", map[string]func(*yaml.Node) error{
		"type":    d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &spec.Type, "inputs[].type") }),
		"default": d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &spec.Default, "inputs[].default") }),
	})
	return spec
}

func (d *componentDecoder) decodeClusterFragment(node *yaml.Node) ClusterFragment {
	var cf ClusterFragment
	cf.Services = map[string]Service{}
	d.mapping(node, "cluster", map[string]func(*yaml.Node) error{
		"services": d.alwaysOK(func(v *yaml.Node) { d.decodeServices(v, cf.Services) }),
	})
	return cf
}

// decodeRequirements walks a "requires:"/"capabilities.constraints:"
// sequence of Requirement mappings. Defined on decoderBase (rather than
// componentDecoder) so provider.go's providerDecoder gets it too through
// embedding: both requires: and capabilities.constraints: are the same
// {expr|capability, message} shape (spec §6).
func (d *decoderBase) decodeRequirements(node *yaml.Node, context string) []Requirement {
	if node.Kind != yaml.SequenceNode {
		d.errorf(node, "%s: expected a sequence, got %s", context, nodeKindName(node.Kind))
		return nil
	}
	reqs := make([]Requirement, 0, len(node.Content))
	for _, item := range node.Content {
		reqs = append(reqs, d.decodeRequirement(item, context))
	}
	return reqs
}

func (d *decoderBase) decodeRequirement(node *yaml.Node, context string) Requirement {
	var req Requirement
	d.mapping(node, context+"[]", map[string]func(*yaml.Node) error{
		"expr":       d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &req.Expr, context+"[].expr") }),
		"capability": d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &req.Capability, context+"[].capability") }),
		"message":    d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &req.Message, context+"[].message") }),
	})
	if (req.Expr != "") == (req.Capability != "") {
		d.errorf(node, "%s[]: exactly one of expr or capability must be set", context)
	}
	return req
}
