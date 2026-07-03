// Package graph builds the domain graph (spec §5): the compile-time view of
// a cluster.yaml document merged with the selected provider's manifest, with
// domain values lowered to provider-specific terms and CEL views populated
// for later stages (contract-check, codegen).
package graph

import (
	"sort"
	"strconv"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/expr"
)

// Domain is the compile-time domain graph for one cluster.yaml document: one
// GroupState per named machine group, plus the provider manifest the graph
// was built against.
type Domain struct {
	Groups   map[string]GroupState
	Provider *ast.ProviderManifest
}

// GroupState is the domain-graph view of one named machine group: the
// decoded AST spec, its CEL-visible view (expr.MachineGroupView), the
// group's disk type after lowering (LoweredDiskType, empty if the group has
// no disk), and a shallow copy of the group's ext block (Ext) with any
// lowering-relevant override already applied.
type GroupState struct {
	Spec            ast.MachineGroup
	View            expr.MachineGroupView
	LoweredDiskType string
	Ext             map[string]any
}

const diskTypeField = "disk.type"

// Build assembles the domain graph for cluster against provider, applying
// the lowering precedence from spec §5.2 to each machine group's disk type:
//
//  1. domain value — the group's resources.disk.type as written in
//     cluster.yaml (e.g. "ssd").
//  2. provider.Lowering["disk.type"][<domain value>] — the provider's own
//     term for that domain value (e.g. "network-ssd"), if the provider
//     declares a lowering table for "disk.type" at all.
//  3. ext override — group.Ext["disk_type"], if present, always wins over
//     both of the above.
//
// A provider that declares no "disk.type" lowering table at all (Lowering is
// nil, or has no "disk.type" key) is treated as doing no lowering for that
// field: the domain value passes through unlowered rather than erroring —
// this lets a provider such as the docker builtin, which has no lowering
// tables, use the domain vocabulary directly. It is an error, however, for a
// "disk.type" lowering table to exist but lack an entry for the group's
// domain value; Build reports this as an Error diagnostic naming the
// provider and the offending value, with Module set to provider.Name.
//
// Lowering (and its associated error) only applies to groups that declare a
// disk with a non-empty type; a group with no disk is left with an empty
// LoweredDiskType and produces no diagnostic.
//
// Build also populates each group's View for CEL: Count and per-machine
// CPU/RAMGb/DiskGb/Disks are filled in from the group's spec, but every
// machine's IP is left empty, since IPs are only known once the topology is
// actually provisioned at runtime, not at compile time.
//
// provider must be non-nil; a nil provider is reported as an Error
// diagnostic rather than a panic, since a caller may reach Build with an
// as-yet-unresolved provider reference.
func Build(cluster *ast.ClusterDoc, provider *ast.ProviderManifest) (*Domain, diag.List) {
	var diags diag.List

	if provider == nil {
		diags.Errorf("", diag.Pos{}, "no provider manifest")
		return nil, diags
	}

	dom := &Domain{
		Groups:   make(map[string]GroupState, len(cluster.Machines)),
		Provider: provider,
	}

	for _, name := range sortedGroupNames(cluster.Machines) {
		spec := cluster.Machines[name]
		state, groupDiags := buildGroup(name, spec, provider)
		diags = append(diags, groupDiags...)
		dom.Groups[name] = state
	}

	return dom, diags
}

func buildGroup(name string, spec ast.MachineGroup, provider *ast.ProviderManifest) (GroupState, diag.List) {
	var diags diag.List

	if spec.Count < 0 {
		diags.Add(diag.Diagnostic{
			Severity: diag.Error,
			Message:  "machine group " + name + ": count must be non-negative (got " + strconv.FormatInt(int64(spec.Count), 10) + ")",
		})
		return GroupState{
			Spec: spec,
			View: expr.MachineGroupView{Count: int64(spec.Count), Machines: nil},
			Ext:  make(map[string]any),
		}, diags
	}

	ext := make(map[string]any, len(spec.Ext))
	for k, v := range spec.Ext {
		ext[k] = v
	}

	state := GroupState{
		Spec: spec,
		View: buildView(spec),
		Ext:  ext,
	}

	if spec.Resources.Disk == nil || spec.Resources.Disk.Type == "" {
		return state, diags
	}

	domainValue := spec.Resources.Disk.Type
	lowered := domainValue
	if table, ok := provider.Lowering[diskTypeField]; ok {
		provValue, ok := table[domainValue]
		if !ok {
			diags.Add(diag.Diagnostic{
				Severity: diag.Error,
				Message:  "provider " + provider.Name + " does not lower disk.type=" + domainValue,
				Module:   provider.Name,
			})
		} else {
			lowered = provValue
		}
	}

	if override, present := ext["disk_type"]; present {
		s, ok := override.(string)
		if !ok {
			diags.Add(diag.Diagnostic{
				Severity: diag.Error,
				Message:  "ext.disk_type must be a string",
				Module:   provider.Name,
			})
		} else {
			lowered = s
		}
	}

	state.LoweredDiskType = lowered
	return state, diags
}

func buildView(spec ast.MachineGroup) expr.MachineGroupView {
	ramGb := int64(0)
	diskGb := int64(0)
	var disks []expr.DiskView
	if spec.Resources.RAM > 0 {
		ramGb = int64(spec.Resources.RAM / (1 << 30)) //nolint:gosec // RAM sizes are far below MaxInt64 GiB.
	}
	if spec.Resources.Disk != nil {
		diskGb = int64(spec.Resources.Disk.Size / (1 << 30)) //nolint:gosec // disk sizes are far below MaxInt64 GiB.
		disks = []expr.DiskView{{Path: "", SizeGb: diskGb}}
	}

	machines := make([]expr.MachineView, spec.Count)
	for i := range machines {
		machines[i] = expr.MachineView{
			IP:     "",
			RAMGb:  ramGb,
			CPU:    int64(spec.Resources.CPU),
			DiskGb: diskGb,
			Disks:  disks,
		}
	}

	return expr.MachineGroupView{
		Count:    int64(spec.Count),
		Machines: machines,
	}
}

func sortedGroupNames(groups map[string]ast.MachineGroup) []string {
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
