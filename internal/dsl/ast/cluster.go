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

	dec := &clusterDecoder{path: path, providerKey: providerKey, diags: &diags}
	doc := &ClusterDoc{
		Machines: map[string]MachineGroup{},
		Services: map[string]Service{},
	}
	dec.decodeDoc(docNode, doc)

	return doc, diags
}

// clusterDecoder carries the state shared across all the nested decode
// helpers below: where diagnostics get attached (path), where they
// accumulate (diags), and which provider's ext-block is allowed
// (providerKey).
type clusterDecoder struct {
	path        string
	providerKey string
	diags       *diag.List
}

func (d *clusterDecoder) errorf(n *yaml.Node, format string, args ...any) {
	d.diags.Errorf(d.path, posOf(n), format, args...)
}

// decodeValue calls value.Decode(target) and, on failure, records a
// diagnostic pointing at value's position instead of returning an error.
// It reports whether decoding succeeded.
func (d *clusterDecoder) decodeValue(value *yaml.Node, target any, field string) bool {
	if err := value.Decode(target); err != nil {
		d.errorf(value, "%s: %v", field, err)
		return false
	}
	return true
}

// alwaysOK adapts a handler that reports its own problems through the
// shared *clusterDecoder (and so never fails the walk itself) to the
// func(*yaml.Node) error shape decodeMapping's handlers map requires.
func alwaysOK(fn func(*yaml.Node)) func(*yaml.Node) error {
	return func(v *yaml.Node) error {
		fn(v)
		return nil
	}
}

// mapping walks node as a strict mapping: any key not in handlers is
// reported as an unknown-key diagnostic. Structural mismatches (node isn't
// actually a mapping) are also turned into a diagnostic here so callers
// never need to check decodeMapping's own error return.
func (d *clusterDecoder) mapping(node *yaml.Node, context string, handlers map[string]func(*yaml.Node) error) {
	err := decodeMapping(node, handlers, func(key string, keyNode, _ *yaml.Node) {
		d.errorf(keyNode, "%s: unknown key %q", context, key)
	})
	if err != nil {
		d.errorf(node, "%s: %v", context, err)
	}
}

func (d *clusterDecoder) decodeDoc(node *yaml.Node, doc *ClusterDoc) {
	d.mapping(node, "cluster", map[string]func(*yaml.Node) error{
		"version":  alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &doc.Version, "version") }),
		"provider": alwaysOK(func(v *yaml.Node) { d.decodeProvider(v, &doc.Provider) }),
		"machines": alwaysOK(func(v *yaml.Node) { d.decodeMachines(v, doc.Machines) }),
		"services": alwaysOK(func(v *yaml.Node) { d.decodeServices(v, doc.Services) }),
	})
}

func (d *clusterDecoder) decodeProvider(node *yaml.Node, out *ProviderUse) {
	d.mapping(node, "provider", map[string]func(*yaml.Node) error{
		"use":    alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &out.Use, "provider.use") }),
		"params": alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &out.Params, "provider.params") }),
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
		"count":     alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &mg.Count, "machines.count") }),
		"resources": alwaysOK(func(v *yaml.Node) { mg.Resources = d.decodeResources(v) }),
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
		"cpu": alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &res.CPU, "resources.cpu") }),
		"ram": alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &res.RAM, "resources.ram") }),
		"disk": alwaysOK(func(v *yaml.Node) {
			disk := d.decodeDisk(v)
			res.Disk = &disk
		}),
	})
	return res
}

func (d *clusterDecoder) decodeDisk(node *yaml.Node) Disk {
	var disk Disk
	d.mapping(node, "disk", map[string]func(*yaml.Node) error{
		"size": alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &disk.Size, "disk.size") }),
		"type": alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &disk.Type, "disk.type") }),
	})
	return disk
}

// decodeServices walks the "services" mapping, whose keys are arbitrary
// service names, so — like decodeMachines — it iterates node.Content
// directly.
func (d *clusterDecoder) decodeServices(node *yaml.Node, out map[string]Service) {
	if node.Kind != yaml.MappingNode {
		d.errorf(node, "services: expected a mapping, got %s", nodeKindName(node.Kind))
		return
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		nameNode, svcNode := node.Content[i], node.Content[i+1]
		out[nameNode.Value] = d.decodeService(svcNode)
	}
}

func (d *clusterDecoder) decodeService(node *yaml.Node) Service {
	var svc Service
	d.mapping(node, "service", map[string]func(*yaml.Node) error{
		"on":      alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &svc.On, "service.on") }),
		"image":   alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &svc.Image, "service.image") }),
		"network": alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &svc.Network, "service.network") }),
		"volumes": alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &svc.Volumes, "service.volumes") }),
		"env":     alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &svc.Env, "service.env") }),
		"configs": alwaysOK(func(v *yaml.Node) { svc.Configs = d.decodeConfigs(v) }),
		"health": alwaysOK(func(v *yaml.Node) {
			health := d.decodeHealth(v)
			svc.Health = &health
		}),
	})
	return svc
}

func (d *clusterDecoder) decodeConfigs(node *yaml.Node) []ConfigFile {
	if node.Kind != yaml.SequenceNode {
		d.errorf(node, "service.configs: expected a sequence, got %s", nodeKindName(node.Kind))
		return nil
	}
	configs := make([]ConfigFile, 0, len(node.Content))
	for _, item := range node.Content {
		var cfg ConfigFile
		d.mapping(item, "service.configs[]", map[string]func(*yaml.Node) error{
			"template": alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &cfg.Template, "service.configs[].template") }),
			"dest":     alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &cfg.Dest, "service.configs[].dest") }),
		})
		configs = append(configs, cfg)
	}
	return configs
}

func (d *clusterDecoder) decodeHealth(node *yaml.Node) Health {
	var health Health
	d.mapping(node, "health", map[string]func(*yaml.Node) error{
		"http":    alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &health.HTTP, "health.http") }),
		"timeout": alwaysOK(func(v *yaml.Node) { d.decodeValue(v, &health.Timeout, "health.timeout") }),
	})
	return health
}
