package types

import (
	"fmt"
	"testing"
)

func TestYDBMirror3DCTargetPresets(t *testing.T) {
	cases := []struct {
		name         YDBPreset
		computeCount int
		computeCPUs  int
		computeMemMB int
	}{
		{YDBMirror3DC3x32, 3, 32, 65536},
		{YDBMirror3DC9x32, 9, 32, 65536},
		{YDBMirror3DC3x64, 3, 64, 131072},
	}
	for _, c := range cases {
		t.Run(string(c.name), func(t *testing.T) {
			p := YDBPresets[c.name]
			if p.FaultTolerance != "mirror-3-dc" || p.FailureDomainType != "disk" {
				t.Fatalf("unexpected topology mode: erasure=%q failure_domain=%q", p.FaultTolerance, p.FailureDomainType)
			}
			if p.StorageGroups != 8 {
				t.Fatalf("storage groups = %d, want 8", p.StorageGroups)
			}
			if p.Storage.Count != 3 || p.Storage.CPUs != 16 || p.Storage.MemoryMB != 32768 {
				t.Fatalf("storage spec = %+v, want 3x 16CPU/32768MB", p.Storage)
			}
			if len(p.Storage.SecondaryDisks) != 3 {
				t.Fatalf("secondary disks = %d, want 3", len(p.Storage.SecondaryDisks))
			}
			for i, d := range p.Storage.SecondaryDisks {
				if d.DeviceName != fmt.Sprintf("ydb-data-%d", i) || d.SizeGB != 930 || d.Type != ydbStoragePdiskType {
					t.Fatalf("disk %d = %+v", i, d)
				}
			}
			if p.Database == nil {
				t.Fatal("database spec is nil")
			}
			if p.Database.Count != c.computeCount || p.Database.CPUs != c.computeCPUs || p.Database.MemoryMB != c.computeMemMB {
				t.Fatalf("compute spec = %+v", *p.Database)
			}
			if p.Database.Placement == nil || p.Database.Placement.Strategy != "round-robin" {
				t.Fatalf("compute placement = %+v, want round-robin", p.Database.Placement)
			}
			if p.Storage.Placement == nil || p.Storage.Placement.Strategy != "round-robin" {
				t.Fatalf("storage placement = %+v, want round-robin", p.Storage.Placement)
			}
		})
	}
}
