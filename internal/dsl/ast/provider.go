package ast

import (
	"gopkg.in/yaml.v3"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
)

// ProviderManifest is the decoded form of a providers/<name>/manifest.yaml
// document (spec §6): what the provider gives the domain (Provides), what
// it can build (Capabilities), and how domain enums map onto its own
// terms (Lowering, spec §5.2).
type ProviderManifest struct {
	Name         string
	Provides     []string
	Capabilities Capabilities
	// Lowering maps a domain field (e.g. "disk.type") to a table of domain
	// value -> provider value (e.g. {ssd: network-ssd}).
	Lowering map[string]map[string]string
}

// Capabilities describes what a provider can build: per-domain-field rules
// (Machine, e.g. "cpu"/"ram") and cross-field Constraints (spec §6).
type Capabilities struct {
	Machine     map[string]CapRule
	Constraints []Requirement
}

// CapRule constrains one domain field of a machine: either to an
// enumeration of allowed values (Enum) or to a CEL predicate (Expr).
type CapRule struct {
	Enum []any
	Expr string
}

// DecodeProviderManifest strictly decodes a providers/<name>/manifest.yaml
// document. Every problem found (parse errors, unknown keys, malformed
// scalars, constraints with the wrong number of Expr/Capability set) is
// reported as a diagnostic in the returned diag.List rather than as a Go
// error, so a caller sees every problem in the document in one pass instead
// of just the first one.
func DecodeProviderManifest(path string, src []byte) (*ProviderManifest, diag.List) {
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
		diags.Errorf(path, posOf(docNode), "provider manifest must be a mapping, got %s", nodeKindName(docNode.Kind))
		return nil, diags
	}

	dec := &providerDecoder{decoderBase{path: path, diags: &diags}}
	doc := &ProviderManifest{
		Capabilities: Capabilities{Machine: map[string]CapRule{}},
		Lowering:     map[string]map[string]string{},
	}
	dec.decodeDoc(docNode, doc)

	return doc, diags
}

// providerDecoder embeds decoderBase, inheriting errorf/decodeValue/
// alwaysOK/mapping plus the generic decodeRequirements helper (reused here
// for capabilities.constraints, which share the Requirement shape with
// component.go's requires:).
type providerDecoder struct {
	decoderBase
}

func (d *providerDecoder) decodeDoc(node *yaml.Node, doc *ProviderManifest) {
	d.mapping(node, "provider manifest", map[string]func(*yaml.Node) error{
		"name":         d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &doc.Name, "name") }),
		"provides":     d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &doc.Provides, "provides") }),
		"capabilities": d.alwaysOK(func(v *yaml.Node) { d.decodeCapabilities(v, &doc.Capabilities) }),
		"lowering":     d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &doc.Lowering, "lowering") }),
	})
}

func (d *providerDecoder) decodeCapabilities(node *yaml.Node, out *Capabilities) {
	d.mapping(node, "capabilities", map[string]func(*yaml.Node) error{
		"machine":     d.alwaysOK(func(v *yaml.Node) { d.decodeMachineCaps(v, out.Machine) }),
		"constraints": d.alwaysOK(func(v *yaml.Node) { out.Constraints = d.decodeRequirements(v, "capabilities.constraints") }),
	})
}

// decodeMachineCaps walks the "capabilities.machine" mapping, whose keys are
// arbitrary domain field names (e.g. "cpu", "ram"), so it iterates
// node.Content directly rather than going through decodeMapping's
// known-key dispatch. A duplicate field name is reported as a diagnostic
// (anchored to the second occurrence's key node) and the first entry is
// preserved.
func (d *providerDecoder) decodeMachineCaps(node *yaml.Node, out map[string]CapRule) {
	if node.Kind != yaml.MappingNode {
		d.errorf(node, "capabilities.machine: expected a mapping, got %s", nodeKindName(node.Kind))
		return
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		nameNode, ruleNode := node.Content[i], node.Content[i+1]
		if _, dup := out[nameNode.Value]; dup {
			d.errorf(nameNode, "capabilities.machine: duplicate field %q", nameNode.Value)
			continue
		}
		out[nameNode.Value] = d.decodeCapRule(ruleNode)
	}
}

func (d *providerDecoder) decodeCapRule(node *yaml.Node) CapRule {
	var rule CapRule
	d.mapping(node, "capabilities.machine[]", map[string]func(*yaml.Node) error{
		"enum": d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &rule.Enum, "capabilities.machine[].enum") }),
		"expr": d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &rule.Expr, "capabilities.machine[].expr") }),
	})
	return rule
}
