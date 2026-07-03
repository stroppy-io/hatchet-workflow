// Package ast decodes stroppy YAML-DSL documents (cluster.yaml today;
// workflow/component documents in later tasks) into typed trees. Decoding
// walks the raw gopkg.in/yaml.v3 node tree instead of calling yaml.Unmarshal
// directly, so every problem in the user's YAML can be reported as a
// diag.Diagnostic carrying an exact line/column, and so a single
// provider-specific top-level key inside a machine group can be captured
// into MachineGroup.Ext instead of rejected as unknown.
package ast

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
)

// ClusterDoc is the decoded form of a cluster.yaml document.
type ClusterDoc struct {
	Version  int
	Provider ProviderUse
	Machines map[string]MachineGroup
	Services map[string]Service
}

// ProviderUse selects the infrastructure provider a cluster is built on and
// carries provider-specific top-level parameters.
type ProviderUse struct {
	Use    string
	Params map[string]any
}

// MachineGroup describes one named group of machines.
type MachineGroup struct {
	Count     int
	Resources Resources
	// Ext holds the contents of the top-level block whose key equals the
	// providerKey argument passed to DecodeCluster (e.g. a "yandex: {...}"
	// block when providerKey is "yandex"). Any other unrecognized top-level
	// key inside a machine group is a strict-decode error.
	Ext map[string]any
}

// Resources describes the compute resources requested for a machine group.
type Resources struct {
	CPU  int
	RAM  ByteSize
	Disk *Disk
}

// Disk describes a machine group's disk.
type Disk struct {
	Size ByteSize
	Type string
}

// Service describes one named service deployed onto a machine group.
type Service struct {
	On      string
	Image   string
	Network string
	Volumes []string
	Env     map[string]string
	Configs []ConfigFile
	Health  *Health
}

// ConfigFile describes one templated config file rendered onto a machine.
type ConfigFile struct {
	Template string
	Dest     string
}

// Health describes a service's health check.
type Health struct {
	HTTP    string
	Timeout Duration
}

// ByteSize is a size in bytes, parsed from strings such as "32g", "100m",
// "512k" or a plain byte count such as "512".
type ByteSize uint64

// ParseByteSize parses a size string with an optional binary-multiplier
// suffix: g (GiB), m (MiB), k (KiB). No suffix means the value is already a
// byte count.
func ParseByteSize(s string) (ByteSize, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("bad size %q: empty", s)
	}
	mult := uint64(1)
	suffix := strings.ToLower(s[len(s)-1:])
	switch suffix {
	case "g":
		mult, s = 1<<30, s[:len(s)-1]
	case "m":
		mult, s = 1<<20, s[:len(s)-1]
	case "k":
		mult, s = 1<<10, s[:len(s)-1]
	}
	n, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("bad size %q", s)
	}
	return ByteSize(n * mult), nil
}

// UnmarshalYAML implements yaml.Unmarshaler so a ByteSize field can be
// decoded directly via yaml.Node.Decode.
func (b *ByteSize) UnmarshalYAML(value *yaml.Node) error {
	parsed, err := ParseByteSize(value.Value)
	if err != nil {
		return err
	}
	*b = parsed
	return nil
}

// Duration wraps time.Duration, parsed from Go duration strings such as
// "120s".
type Duration time.Duration

// ParseDuration parses a Go duration string (e.g. "120s", "5m") into a
// Duration.
func ParseDuration(s string) (Duration, error) {
	d, err := time.ParseDuration(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("bad duration %q: %w", s, err)
	}
	return Duration(d), nil
}

// UnmarshalYAML implements yaml.Unmarshaler so a Duration field can be
// decoded directly via yaml.Node.Decode.
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	parsed, err := ParseDuration(value.Value)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// DecodeCluster strictly decodes a cluster.yaml document. Every problem
// found (parse errors, unknown keys, malformed scalars) is reported as a
// diagnostic in the returned diag.List rather than as a Go error, so a
// caller sees every problem in the document in one pass instead of just the
// first one. providerKey names the provider in use (e.g. "yandex"); inside
// each machine group, a top-level key equal to providerKey is captured into
// MachineGroup.Ext instead of being rejected as unknown.
func DecodeCluster(path string, src []byte, providerKey string) (*ClusterDoc, diag.List) {
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
		diags.Errorf(path, posOf(docNode), "cluster document must be a mapping, got %s", nodeKindName(docNode.Kind))
		return nil, diags
	}

	dec := &clusterDecoder{decoderBase: decoderBase{path: path, diags: &diags}, providerKey: providerKey}
	doc := &ClusterDoc{
		Machines: map[string]MachineGroup{},
		Services: map[string]Service{},
	}
	dec.decodeDoc(docNode, doc)

	return doc, diags
}

// clusterDecoder embeds decoderBase (errorf/decodeValue/alwaysOK/mapping,
// and the generic decodeServices/decodeJobs helpers) and adds the one bit
// of state specific to cluster.yaml: which provider's ext-block is allowed
// inside a machine group (providerKey).
type clusterDecoder struct {
	decoderBase
	providerKey string
}

func (d *clusterDecoder) decodeDoc(node *yaml.Node, doc *ClusterDoc) {
	d.mapping(node, "cluster", map[string]func(*yaml.Node) error{
		"version":  d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &doc.Version, "version") }),
		"provider": d.alwaysOK(func(v *yaml.Node) { d.decodeProvider(v, &doc.Provider) }),
		"machines": d.alwaysOK(func(v *yaml.Node) { d.decodeMachines(v, doc.Machines) }),
		"services": d.alwaysOK(func(v *yaml.Node) { d.decodeServices(v, doc.Services) }),
	})
}

func (d *clusterDecoder) decodeProvider(node *yaml.Node, out *ProviderUse) {
	d.mapping(node, "provider", map[string]func(*yaml.Node) error{
		"use":    d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &out.Use, "provider.use") }),
		"params": d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &out.Params, "provider.params") }),
	})
}

// decodeMachines walks the "machines" mapping, whose keys are arbitrary
// machine-group names (not a fixed schema), so it iterates node.Content
// directly rather than going through decodeMapping's known-key dispatch.
func (d *clusterDecoder) decodeMachines(node *yaml.Node, out map[string]MachineGroup) {
	if node.Kind != yaml.MappingNode {
		d.errorf(node, "machines: expected a mapping, got %s", nodeKindName(node.Kind))
		return
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		nameNode, groupNode := node.Content[i], node.Content[i+1]
		out[nameNode.Value] = d.decodeMachineGroup(groupNode)
	}
}

func (d *clusterDecoder) decodeMachineGroup(node *yaml.Node) MachineGroup {
	var mg MachineGroup

	err := decodeMapping(node, map[string]func(*yaml.Node) error{
		"count":     d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &mg.Count, "machines.count") }),
		"resources": d.alwaysOK(func(v *yaml.Node) { mg.Resources = d.decodeResources(v) }),
	}, func(key string, keyNode, valNode *yaml.Node) {
		if key == d.providerKey {
			var ext map[string]any
			if d.decodeValue(valNode, &ext, "machines."+key) {
				mg.Ext = ext
			}
			return
		}
		d.errorf(keyNode, "machines: unknown key %q", key)
	})
	if err != nil {
		d.errorf(node, "machines: %v", err)
	}

	return mg
}

func (d *clusterDecoder) decodeResources(node *yaml.Node) Resources {
	var res Resources
	d.mapping(node, "resources", map[string]func(*yaml.Node) error{
		"cpu": d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &res.CPU, "resources.cpu") }),
		"ram": d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &res.RAM, "resources.ram") }),
		"disk": d.alwaysOK(func(v *yaml.Node) {
			disk := d.decodeDisk(v)
			res.Disk = &disk
		}),
	})
	return res
}

func (d *clusterDecoder) decodeDisk(node *yaml.Node) Disk {
	var disk Disk
	d.mapping(node, "disk", map[string]func(*yaml.Node) error{
		"size": d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &disk.Size, "disk.size") }),
		"type": d.alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &disk.Type, "disk.type") }),
	})
	return disk
}

// decodeServices, decodeService, decodeConfigs and decodeHealth are
// generic (not tied to cluster.yaml specifically — a component.yaml can
// nest the same shapes under `cluster:`) and live on decoderBase in
// yamlwalk.go; clusterDecoder inherits them through embedding.
