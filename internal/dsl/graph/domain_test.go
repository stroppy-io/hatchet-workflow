package graph_test

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/graph"
)

func dbGroup(diskType string, ext map[string]any) ast.MachineGroup {
	return ast.MachineGroup{
		Count: 3,
		Resources: ast.Resources{
			CPU: 8,
			RAM: 32 << 30,
			Disk: &ast.Disk{
				Size: 100 << 30,
				Type: diskType,
			},
		},
		Ext: ext,
	}
}

func yandexProvider(lowering map[string]map[string]string) *ast.ProviderManifest {
	return &ast.ProviderManifest{
		Name:     "yandex",
		Lowering: lowering,
	}
}

// --- brief's three required cases ---

func TestBuildLowersDiskType(t *testing.T) {
	cluster := &ast.ClusterDoc{
		Machines: map[string]ast.MachineGroup{
			"db": dbGroup("ssd", nil),
		},
	}
	provider := yandexProvider(map[string]map[string]string{
		"disk.type": {"ssd": "network-ssd"},
	})

	dom, diags := graph.Build(cluster, provider)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	got := dom.Groups["db"].LoweredDiskType
	if got != "network-ssd" {
		t.Fatalf("LoweredDiskType = %q, want %q", got, "network-ssd")
	}
}

func TestBuildExtOverrideWinsOverLowering(t *testing.T) {
	cluster := &ast.ClusterDoc{
		Machines: map[string]ast.MachineGroup{
			"db": dbGroup("ssd", map[string]any{"disk_type": "network-ssd-nonreplicated"}),
		},
	}
	provider := yandexProvider(map[string]map[string]string{
		"disk.type": {"ssd": "network-ssd"},
	})

	dom, diags := graph.Build(cluster, provider)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	got := dom.Groups["db"].LoweredDiskType
	if got != "network-ssd-nonreplicated" {
		t.Fatalf("LoweredDiskType = %q, want override %q", got, "network-ssd-nonreplicated")
	}
	if dom.Groups["db"].Ext["disk_type"] != "network-ssd-nonreplicated" {
		t.Fatalf("Ext[disk_type] not recorded: %+v", dom.Groups["db"].Ext)
	}
}

func TestBuildMissingLoweringEntryIsError(t *testing.T) {
	cluster := &ast.ClusterDoc{
		Machines: map[string]ast.MachineGroup{
			"db": dbGroup("nvme", nil),
		},
	}
	provider := yandexProvider(map[string]map[string]string{
		"disk.type": {"ssd": "network-ssd"},
	})

	_, diags := graph.Build(cluster, provider)
	if !diags.HasErrors() {
		t.Fatal("expected a lowering error diagnostic")
	}
	found := false
	for _, d := range diags {
		if d.Message == `provider yandex does not lower disk.type=nvme` {
			found = true
			if d.Module != "yandex" {
				t.Fatalf("diag.Module = %q, want %q", d.Module, "yandex")
			}
		}
	}
	if !found {
		t.Fatalf("expected error message naming provider and value, got: %+v", diags)
	}
}

// --- additional cases per task instructions ---

func TestBuildNoDiskGroupSkipsLowering(t *testing.T) {
	cluster := &ast.ClusterDoc{
		Machines: map[string]ast.MachineGroup{
			"app": {
				Count:     2,
				Resources: ast.Resources{CPU: 4, RAM: 8 << 30},
			},
		},
	}
	provider := yandexProvider(map[string]map[string]string{
		"disk.type": {"ssd": "network-ssd"},
	})

	dom, diags := graph.Build(cluster, provider)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	if dom.Groups["app"].LoweredDiskType != "" {
		t.Fatalf("LoweredDiskType = %q, want empty for diskless group", dom.Groups["app"].LoweredDiskType)
	}
}

func TestBuildNoLoweringTablePassesValueThrough(t *testing.T) {
	cluster := &ast.ClusterDoc{
		Machines: map[string]ast.MachineGroup{
			"db": dbGroup("local-ssd", nil),
		},
	}
	provider := &ast.ProviderManifest{Name: "docker"} // no Lowering table at all

	dom, diags := graph.Build(cluster, provider)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	if dom.Groups["db"].LoweredDiskType != "local-ssd" {
		t.Fatalf("LoweredDiskType = %q, want passthrough %q", dom.Groups["db"].LoweredDiskType, "local-ssd")
	}
}

func TestBuildNonStringExtDiskTypeIsError(t *testing.T) {
	cluster := &ast.ClusterDoc{
		Machines: map[string]ast.MachineGroup{
			"db": dbGroup("ssd", map[string]any{"disk_type": 42}),
		},
	}
	provider := yandexProvider(map[string]map[string]string{
		"disk.type": {"ssd": "network-ssd"},
	})

	_, diags := graph.Build(cluster, provider)
	if !diags.HasErrors() {
		t.Fatal("expected an error diagnostic for non-string ext disk_type")
	}
}

func TestBuildViewPopulation(t *testing.T) {
	cluster := &ast.ClusterDoc{
		Machines: map[string]ast.MachineGroup{
			"db": {
				Count: 3,
				Resources: ast.Resources{
					CPU: 8,
					RAM: 32 << 30,
					Disk: &ast.Disk{
						Size: 100 << 30,
						Type: "ssd",
					},
				},
			},
		},
	}
	provider := yandexProvider(map[string]map[string]string{
		"disk.type": {"ssd": "network-ssd"},
	})

	dom, diags := graph.Build(cluster, provider)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	view := dom.Groups["db"].View
	if view.Count != 3 {
		t.Fatalf("view.Count = %d, want 3", view.Count)
	}
	if len(view.Machines) != 3 {
		t.Fatalf("len(view.Machines) = %d, want 3", len(view.Machines))
	}
	for i, m := range view.Machines {
		if m.IP != "" {
			t.Fatalf("machines[%d].IP = %q, want empty at compile time", i, m.IP)
		}
		if m.CPU != 8 {
			t.Fatalf("machines[%d].CPU = %d, want 8", i, m.CPU)
		}
		if m.RAMGb != 32 {
			t.Fatalf("machines[%d].RAMGb = %d, want 32", i, m.RAMGb)
		}
		if m.DiskGb != 100 {
			t.Fatalf("machines[%d].DiskGb = %d, want 100", i, m.DiskGb)
		}
		if len(m.Disks) != 1 || m.Disks[0].SizeGb != 100 {
			t.Fatalf("machines[%d].Disks = %+v, want one 100gb disk", i, m.Disks)
		}
	}
}

func TestBuildViewSubGbRoundsDownToZero(t *testing.T) {
	cluster := &ast.ClusterDoc{
		Machines: map[string]ast.MachineGroup{
			"tiny": {
				Count: 1,
				Resources: ast.Resources{
					CPU: 1,
					RAM: 512 << 20, // half a GiB
					Disk: &ast.Disk{
						Size: 256 << 20, // quarter GiB
						Type: "ssd",
					},
				},
			},
		},
	}
	provider := yandexProvider(map[string]map[string]string{
		"disk.type": {"ssd": "network-ssd"},
	})

	dom, diags := graph.Build(cluster, provider)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	m := dom.Groups["tiny"].View.Machines[0]
	if m.RAMGb != 0 {
		t.Fatalf("RAMGb = %d, want 0 (sub-GB rounds down)", m.RAMGb)
	}
	if m.DiskGb != 0 {
		t.Fatalf("DiskGb = %d, want 0 (sub-GB rounds down)", m.DiskGb)
	}
}

func TestBuildDisklessGroupHasEmptyDisksView(t *testing.T) {
	cluster := &ast.ClusterDoc{
		Machines: map[string]ast.MachineGroup{
			"app": {
				Count:     1,
				Resources: ast.Resources{CPU: 2, RAM: 4 << 30},
			},
		},
	}
	provider := yandexProvider(nil)

	dom, diags := graph.Build(cluster, provider)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	m := dom.Groups["app"].View.Machines[0]
	if m.DiskGb != 0 {
		t.Fatalf("DiskGb = %d, want 0", m.DiskGb)
	}
	if len(m.Disks) != 0 {
		t.Fatalf("Disks = %+v, want empty", m.Disks)
	}
}

func TestBuildExtIsShallowCopyNotAliased(t *testing.T) {
	specExt := map[string]any{"platform_id": "standard-v3"}
	cluster := &ast.ClusterDoc{
		Machines: map[string]ast.MachineGroup{
			"db": {
				Count:     1,
				Resources: ast.Resources{CPU: 2, RAM: 4 << 30},
				Ext:       specExt,
			},
		},
	}
	provider := yandexProvider(nil)

	dom, diags := graph.Build(cluster, provider)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	dom.Groups["db"].Ext["platform_id"] = "mutated"
	if specExt["platform_id"] != "standard-v3" {
		t.Fatalf("mutating GroupState.Ext leaked into ast.MachineGroup.Ext: %+v", specExt)
	}
}

func TestBuildNilProviderIsError(t *testing.T) {
	cluster := &ast.ClusterDoc{
		Machines: map[string]ast.MachineGroup{
			"db": dbGroup("ssd", nil),
		},
	}
	_, diags := graph.Build(cluster, nil)
	if !diags.HasErrors() {
		t.Fatal("expected an error diagnostic for nil provider manifest")
	}
}

func TestBuildNegativeCountIsError(t *testing.T) {
	cluster := &ast.ClusterDoc{
		Machines: map[string]ast.MachineGroup{
			"bad": {
				Count:     -1,
				Resources: ast.Resources{CPU: 2, RAM: 4 << 30},
			},
		},
	}
	provider := yandexProvider(nil)

	_, diags := graph.Build(cluster, provider)
	if !diags.HasErrors() {
		t.Fatal("expected an error diagnostic for negative count")
	}

	found := false
	for _, d := range diags {
		if d.Message == `machine group bad: count must be non-negative (got -1)` {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected error message for group 'bad' with count, got: %+v", diags)
	}
}
