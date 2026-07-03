package contract_test

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/contract"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/graph"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/include"
)

// --- fixtures ---

func buildDomain(t *testing.T, machines map[string]ast.MachineGroup, provider *ast.ProviderManifest) *graph.Domain {
	t.Helper()
	cluster := &ast.ClusterDoc{Machines: machines}
	dom, diags := graph.Build(cluster, provider)
	if diags.HasErrors() {
		t.Fatalf("graph.Build: %+v", diags)
	}
	return dom
}

func mgroup(count, cpu int, ram, disk ast.ByteSize, diskType string) ast.MachineGroup {
	mg := ast.MachineGroup{
		Count:     count,
		Resources: ast.Resources{CPU: cpu, RAM: ram},
	}
	if diskType != "" {
		mg.Resources.Disk = &ast.Disk{Size: disk, Type: diskType}
	}
	return mg
}

func plainProvider(name string, provides ...string) *ast.ProviderManifest {
	return &ast.ProviderManifest{Name: name, Provides: provides}
}

func hasMessage(diags diag.List, msg string) (diag.Diagnostic, bool) {
	for _, d := range diags {
		if d.Message == msg {
			return d, true
		}
	}
	return diag.Diagnostic{}, false
}

// --- 1. etcd count=2 -> quorum violation ---

func etcdDoc() *ast.ComponentDoc {
	return &ast.ComponentDoc{
		Inputs: map[string]ast.InputSpec{"nodes": {Type: "machine_group"}},
		Requires: []ast.Requirement{
			{
				Expr:    "inputs.nodes.count >= 3 && inputs.nodes.count % 2 == 1",
				Message: "etcd: нечётный кворум ≥ 3",
			},
		},
		Provides: []string{"kv.etcd"},
	}
}

func TestCheckEtcdQuorumViolation(t *testing.T) {
	dom := buildDomain(t, map[string]ast.MachineGroup{
		"db": mgroup(2, 4, 8<<30, 0, ""),
	}, plainProvider("docker"))

	comps := []include.BoundComponent{
		{Name: "etcd", Doc: etcdDoc(), Inputs: map[string]any{"nodes": "db"}},
	}

	diags := contract.Check(dom, comps)
	if !diags.HasErrors() {
		t.Fatalf("expected quorum violation, got: %+v", diags)
	}
	d, found := hasMessage(diags, "etcd: нечётный кворум ≥ 3")
	if !found {
		t.Fatalf("expected exact message, got: %+v", diags)
	}
	if d.Module != "etcd" {
		t.Fatalf("Module = %q, want %q", d.Module, "etcd")
	}
}

func TestCheckEtcdQuorumSatisfied(t *testing.T) {
	dom := buildDomain(t, map[string]ast.MachineGroup{
		"db": mgroup(3, 4, 8<<30, 0, ""),
	}, plainProvider("docker"))

	comps := []include.BoundComponent{
		{Name: "etcd", Doc: etcdDoc(), Inputs: map[string]any{"nodes": "db"}},
	}

	diags := contract.Check(dom, comps)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
}

// --- 2. patroni requires capability kv.etcd ---

func patroniDoc() *ast.ComponentDoc {
	return &ast.ComponentDoc{
		Requires: []ast.Requirement{
			{Capability: "kv.etcd"},
		},
	}
}

func TestCheckCapabilityResolvedByComponent(t *testing.T) {
	dom := buildDomain(t, map[string]ast.MachineGroup{
		"db": mgroup(3, 4, 8<<30, 0, ""),
	}, plainProvider("docker"))

	comps := []include.BoundComponent{
		{Name: "etcd", Doc: etcdDoc(), Inputs: map[string]any{"nodes": "db"}},
		{Name: "patroni", Doc: patroniDoc(), Inputs: map[string]any{}},
	}

	diags := contract.Check(dom, comps)
	if diags.HasErrors() {
		t.Fatalf("expected clean check, got: %+v", diags)
	}
}

func TestCheckCapabilityUnresolved(t *testing.T) {
	dom := buildDomain(t, map[string]ast.MachineGroup{
		"db": mgroup(3, 4, 8<<30, 0, ""),
	}, plainProvider("docker"))

	comps := []include.BoundComponent{
		{Name: "patroni", Doc: patroniDoc(), Inputs: map[string]any{}},
	}

	diags := contract.Check(dom, comps)
	if !diags.HasErrors() {
		t.Fatal("expected unresolved capability error")
	}
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "kv.etcd") {
			found = true
			if d.Module != "patroni" {
				t.Fatalf("Module = %q, want %q", d.Module, "patroni")
			}
		}
	}
	if !found {
		t.Fatalf("expected error naming kv.etcd, got: %+v", diags)
	}
}

// --- 3. provider cpu enum ---

func TestCheckProviderCPUEnumViolation(t *testing.T) {
	provider := &ast.ProviderManifest{
		Name: "yandex",
		Capabilities: ast.Capabilities{
			Machine: map[string]ast.CapRule{
				"cpu": {Enum: []any{2, 4, 8}},
			},
		},
	}
	dom := buildDomain(t, map[string]ast.MachineGroup{
		"db": mgroup(1, 6, 8<<30, 0, ""),
	}, provider)

	diags := contract.Check(dom, nil)
	if !diags.HasErrors() {
		t.Fatalf("expected cpu enum violation, got: %+v", diags)
	}
	found := false
	for _, d := range diags {
		if d.Module == "yandex" && strings.Contains(d.Message, "cpu") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error naming provider+cpu, got: %+v", diags)
	}
}

func TestCheckProviderCPUEnumSatisfied(t *testing.T) {
	provider := &ast.ProviderManifest{
		Name: "yandex",
		Capabilities: ast.Capabilities{
			Machine: map[string]ast.CapRule{
				"cpu": {Enum: []any{2, 4, 8}},
			},
		},
	}
	dom := buildDomain(t, map[string]ast.MachineGroup{
		"db": mgroup(1, 8, 8<<30, 0, ""),
	}, provider)

	diags := contract.Check(dom, nil)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
}

// --- 4. local-ssd constraint via machine.ext.disk_type ---

func localSSDProvider() *ast.ProviderManifest {
	return &ast.ProviderManifest{
		Name: "yandex",
		Capabilities: ast.Capabilities{
			Constraints: []ast.Requirement{
				{
					Expr:    "machine.ext.disk_type == 'local-ssd' ? machine.cpu >= 8 : true",
					Message: "local-ssd только от 8 vCPU",
				},
			},
		},
	}
}

func TestCheckLocalSSDConstraintViolation(t *testing.T) {
	dom := buildDomain(t, map[string]ast.MachineGroup{
		"db": mgroup(1, 4, 8<<30, 100<<30, "local-ssd"),
	}, localSSDProvider())

	diags := contract.Check(dom, nil)
	if !diags.HasErrors() {
		t.Fatalf("expected local-ssd constraint violation, got: %+v", diags)
	}
	d, found := hasMessage(diags, "local-ssd только от 8 vCPU")
	if !found {
		t.Fatalf("expected exact message, got: %+v", diags)
	}
	if d.Module != "yandex" {
		t.Fatalf("Module = %q, want %q", d.Module, "yandex")
	}
}

func TestCheckLocalSSDConstraintSatisfiedByHighCPU(t *testing.T) {
	dom := buildDomain(t, map[string]ast.MachineGroup{
		"db": mgroup(1, 8, 8<<30, 100<<30, "local-ssd"),
	}, localSSDProvider())

	diags := contract.Check(dom, nil)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
}

func TestCheckLocalSSDConstraintIrrelevantForOtherDiskType(t *testing.T) {
	dom := buildDomain(t, map[string]ast.MachineGroup{
		"db": mgroup(1, 4, 8<<30, 100<<30, "network-ssd"),
	}, localSSDProvider())

	diags := contract.Check(dom, nil)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
}

// --- 5. workload formula: target.machines[0].disk_gb vs inputs.iterations*row_bytes ---
//
// Two semantic corrections to the brief's literal example, both documented
// in task-10-report.md:
//
//   - `target.disk_gb`, as the brief's example expr literally writes it,
//     cannot compile: ComponentEnv (a frozen task-7 deliverable) declares
//     `target` as expr.MachineGroupView, which has only `count`/`machines`
//     — `disk_gb` lives on the per-machine expr.MachineView nested inside
//     `machines`. This test uses `target.machines[0].disk_gb` instead (the
//     designated group's representative machine — the fixture group has
//     Count 1, so machines[0] is its only machine).
//   - the brief's literal numbers (iterations=10000, row_bytes=512) don't
//     actually cross the stated 1g/100g boundary under the given formula
//     (10000*512*2.5 =~ 12.2 MiB, comfortably under even a 1 GiB disk) —
//     this test uses iterations=10_000_000 instead, the smallest round
//     number that reproduces the brief's documented "1g fails / 100g
//     passes" behavior while keeping the exact multiplier, input names, and
//     author message from the brief.

func workloadDoc() *ast.ComponentDoc {
	return &ast.ComponentDoc{
		Inputs: map[string]ast.InputSpec{
			"iterations": {Type: "int"},
			"row_bytes":  {Type: "int", Default: 512},
			"target":     {Type: "machine_group"},
		},
		Requires: []ast.Requirement{
			{
				Expr:    "double(target.machines[0].disk_gb) * 1000000000.0 >= double(inputs.iterations) * double(inputs.row_bytes) * 2.5",
				Message: "диск мал для заданного числа итераций",
			},
		},
	}
}

func TestCheckWorkloadFormulaDiskTooSmall(t *testing.T) {
	dom := buildDomain(t, map[string]ast.MachineGroup{
		"wl": mgroup(1, 4, 8<<30, 1<<30, "ssd"),
	}, plainProvider("docker"))

	comps := []include.BoundComponent{
		{
			Name: "insert",
			Doc:  workloadDoc(),
			Inputs: map[string]any{
				"iterations": 10_000_000,
				"row_bytes":  512,
				"target":     "wl",
			},
		},
	}

	diags := contract.Check(dom, comps)
	if !diags.HasErrors() {
		t.Fatalf("expected disk-too-small error, got: %+v", diags)
	}
	d, found := hasMessage(diags, "диск мал для заданного числа итераций")
	if !found {
		t.Fatalf("expected exact message, got: %+v", diags)
	}
	if d.Module != "insert" {
		t.Fatalf("Module = %q, want %q", d.Module, "insert")
	}
}

func TestCheckWorkloadFormulaDiskSufficient(t *testing.T) {
	dom := buildDomain(t, map[string]ast.MachineGroup{
		"wl": mgroup(1, 4, 8<<30, 100<<30, "ssd"),
	}, plainProvider("docker"))

	comps := []include.BoundComponent{
		{
			Name: "insert",
			Doc:  workloadDoc(),
			Inputs: map[string]any{
				"iterations": 10_000_000,
				"row_bytes":  512,
				"target":     "wl",
			},
		},
	}

	diags := contract.Check(dom, comps)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
}

// --- edge cases ---

func TestCheckEmptyRequiresIsClean(t *testing.T) {
	dom := buildDomain(t, map[string]ast.MachineGroup{
		"db": mgroup(1, 4, 8<<30, 0, ""),
	}, plainProvider("docker"))

	comps := []include.BoundComponent{
		{Name: "noop", Doc: &ast.ComponentDoc{}, Inputs: map[string]any{}},
	}

	diags := contract.Check(dom, comps)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
}

func TestCheckNoComponentsIsClean(t *testing.T) {
	dom := buildDomain(t, map[string]ast.MachineGroup{
		"db": mgroup(1, 4, 8<<30, 0, ""),
	}, plainProvider("docker"))

	diags := contract.Check(dom, nil)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
}

// TestCheckEvalTypeErrorProducesDiagNotPanic locks that a requires:expr
// which fails to typecheck/compile (rather than evaluating to false) is
// reported as a diagnostic carrying the CEL error, not a panic.
func TestCheckEvalTypeErrorProducesDiagNotPanic(t *testing.T) {
	dom := buildDomain(t, map[string]ast.MachineGroup{
		"db": mgroup(1, 4, 8<<30, 0, ""),
	}, plainProvider("docker"))

	comps := []include.BoundComponent{
		{
			Name: "broken",
			Doc: &ast.ComponentDoc{
				Requires: []ast.Requirement{
					{Expr: "inputs.nodes +++ bad syntax", Message: "should never show"},
				},
			},
			Inputs: map[string]any{},
		},
	}

	var diags diag.List
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Check panicked: %v", r)
			}
		}()
		diags = contract.Check(dom, comps)
	}()

	if !diags.HasErrors() {
		t.Fatal("expected a diagnostic for the malformed expression")
	}
	d, found := hasMessage(diags, "should never show")
	if found {
		t.Fatalf("author message must not be used for a compile error: %+v", d)
	}
}

// TestCheckTargetUnboundWhenZeroMachineGroupInputs locks the documented
// convention: a component with no machine_group input has no `target`
// binding, so a requires:expr referencing target surfaces as an Eval
// error diagnostic rather than silently resolving to some default.
func TestCheckTargetUnboundWhenZeroMachineGroupInputs(t *testing.T) {
	dom := buildDomain(t, map[string]ast.MachineGroup{
		"db": mgroup(1, 4, 8<<30, 0, ""),
	}, plainProvider("docker"))

	comps := []include.BoundComponent{
		{
			Name: "no-target",
			Doc: &ast.ComponentDoc{
				Inputs: map[string]ast.InputSpec{"iterations": {Type: "int"}},
				Requires: []ast.Requirement{
					{Expr: "target.disk_gb >= 1", Message: "needs a target"},
				},
			},
			Inputs: map[string]any{"iterations": 1},
		},
	}

	diags := contract.Check(dom, comps)
	if !diags.HasErrors() {
		t.Fatal("expected an eval error diagnostic for unbound target")
	}
}

// TestCheckEnumMatchesNumericTypesAcrossIntFloat64 locks that enum
// membership is compared numerically (int/int64/float64), not via
// reflect.DeepEqual — a YAML-decoded enum list holds plain `int` values,
// while the domain CPU value flowing through expr.MachineGroupView is
// int64.
func TestCheckEnumMatchesNumericTypesAcrossIntFloat64(t *testing.T) {
	provider := &ast.ProviderManifest{
		Name: "yandex",
		Capabilities: ast.Capabilities{
			Machine: map[string]ast.CapRule{
				"cpu": {Enum: []any{2, 4.0, int64(8)}},
			},
		},
	}
	dom := buildDomain(t, map[string]ast.MachineGroup{
		"db": mgroup(1, 4, 8<<30, 0, ""),
	}, provider)

	diags := contract.Check(dom, nil)
	if diags.HasErrors() {
		t.Fatalf("expected numeric enum match (int64 4 == float64 4.0), got: %+v", diags)
	}
}

// TestCheckUnknownCapabilityKeyWarnsAndSkips locks that an enum rule under
// an unrecognized capabilities.machine key produces a warning, not an
// error, and does not block the check.
func TestCheckUnknownCapabilityKeyWarnsAndSkips(t *testing.T) {
	provider := &ast.ProviderManifest{
		Name: "yandex",
		Capabilities: ast.Capabilities{
			Machine: map[string]ast.CapRule{
				"mystery": {Enum: []any{1, 2, 3}},
			},
		},
	}
	dom := buildDomain(t, map[string]ast.MachineGroup{
		"db": mgroup(1, 4, 8<<30, 0, ""),
	}, provider)

	diags := contract.Check(dom, nil)
	if diags.HasErrors() {
		t.Fatalf("unknown capability key must not error, got: %+v", diags)
	}
	found := false
	for _, d := range diags {
		if d.Severity == diag.Warning && strings.Contains(d.Message, "mystery") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a warning diagnostic naming the unknown key, got: %+v", diags)
	}
}

// TestCheckProviderProvidesAlsoCoveredByComponentIsHarmless locks that a
// capability provided by both the provider manifest and a bundled
// component dedups harmlessly (no duplicate-provide error, no panic).
func TestCheckProviderProvidesAlsoCoveredByComponentIsHarmless(t *testing.T) {
	dom := buildDomain(t, map[string]ast.MachineGroup{
		"db": mgroup(3, 4, 8<<30, 0, ""),
	}, plainProvider("docker", "kv.etcd"))

	comps := []include.BoundComponent{
		{Name: "etcd", Doc: etcdDoc(), Inputs: map[string]any{"nodes": "db"}},
		{Name: "patroni", Doc: patroniDoc(), Inputs: map[string]any{}},
	}

	diags := contract.Check(dom, comps)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
}
