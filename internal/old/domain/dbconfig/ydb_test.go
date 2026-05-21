package dbconfig

import (
	"strings"
	"testing"
)

func TestRenderYDBStorageConf_HostPlaceholders(t *testing.T) {
	out := RenderYDBStorageConf(RenderYDBConfOpts{HostCount: 3, MemoryMB: 8192, CPUs: 4})
	for i := 0; i < 3; i++ {
		want := "host: " + ydbHostPlaceholder(i)
		if !strings.Contains(out, want) {
			t.Errorf("host placeholder %d missing:\n%s", i, out)
		}
	}
}

func TestRenderYDBStorageConf_HardLimitFromMemory(t *testing.T) {
	// 85% of 8192 MB = 6963 MB → 6963 * 1024 * 1024 = 7301234688 bytes.
	out := RenderYDBStorageConf(RenderYDBConfOpts{HostCount: 3, MemoryMB: 8192})
	if !strings.Contains(out, "hard_limit_bytes: 7301234688") {
		t.Errorf("hard_limit_bytes wrong; got:\n%s", out)
	}
}

func TestRenderYDBStorageConf_NoMemoryControllerWhenMemZero(t *testing.T) {
	out := RenderYDBStorageConf(RenderYDBConfOpts{HostCount: 1, MemoryMB: 0})
	if strings.Contains(out, "memory_controller_config:") {
		t.Errorf("memory_controller_config should be omitted when MemoryMB=0:\n%s", out)
	}
}

func TestRenderYDBStorageConf_Mirror3DCGeometry(t *testing.T) {
	out := RenderYDBStorageConf(RenderYDBConfOpts{HostCount: 3, MemoryMB: 8192, FaultTolerance: "mirror-3-dc"})
	if !strings.Contains(out, "realm_level_begin: 10") {
		t.Errorf("mirror-3-dc geometry missing:\n%s", out)
	}
	if !strings.Contains(out, "static_erasure: mirror-3-dc") {
		t.Errorf("static_erasure should be mirror-3-dc:\n%s", out)
	}
}

func TestRenderYDBStorageConf_NodeTypeStorage(t *testing.T) {
	out := RenderYDBStorageConf(RenderYDBConfOpts{HostCount: 1, MemoryMB: 1024})
	if !strings.Contains(out, "node_type: STORAGE") {
		t.Errorf("storage role missing STORAGE node_type:\n%s", out)
	}
	if strings.Contains(out, "node_type: COMPUTE") {
		t.Errorf("storage role should not emit COMPUTE node_type:\n%s", out)
	}
}

func TestRenderYDBDatabaseConf_NodeTypeCompute(t *testing.T) {
	out := RenderYDBDatabaseConf(RenderYDBDatabaseConfOpts{HostCount: 3, MemoryMB: 8192, CPUs: 4})
	if !strings.Contains(out, "node_type: COMPUTE") {
		t.Errorf("database role missing COMPUTE node_type:\n%s", out)
	}
	if strings.Contains(out, "node_type: STORAGE") {
		t.Errorf("database role should not emit STORAGE node_type:\n%s", out)
	}
	// Same placeholder shape as storage so the SPA template stays consistent.
	if !strings.Contains(out, "host: "+ydbHostPlaceholder(2)) {
		t.Errorf("database role missing host placeholder:\n%s", out)
	}
}

func TestRenderYDBStorageConf_RawBlockDevicePath(t *testing.T) {
	out := RenderYDBStorageConf(RenderYDBConfOpts{
		HostCount:        3,
		MemoryMB:         4096,
		BlockDevicePaths: []string{"/dev/disk/by-id/virtio-ydb-data"},
	})
	// host_configs entry should reference the raw device, not the file-backed
	// pdisk.data fallback.
	if !strings.Contains(out, "path: /dev/disk/by-id/virtio-ydb-data\n") {
		t.Errorf("expected raw-device path in host_configs:\n%s", out)
	}
	if strings.Contains(out, "pdisk.data") {
		t.Errorf("file-backed pdisk path should not appear when BlockDevicePaths is set:\n%s", out)
	}
}

func TestRenderYDBStorageConf_MultiDiskPerHost(t *testing.T) {
	paths := []string{
		"/dev/disk/by-id/virtio-ydb-data-0",
		"/dev/disk/by-id/virtio-ydb-data-1",
		"/dev/disk/by-id/virtio-ydb-data-2",
	}
	out := RenderYDBStorageConf(RenderYDBConfOpts{
		HostCount:         3,
		MemoryMB:          4096,
		BlockDevicePaths:  paths,
		FailureDomainType: "disk",
	})
	// host_configs should list one drive entry per pdisk (3 paths × "type: SSD").
	for _, p := range paths {
		if !strings.Contains(out, "  - path: "+p+"\n") {
			t.Errorf("host_configs missing drive %q:\n%s", p, out)
		}
	}
	// blob_storage_config: 3 hosts × 3 pdisks = 9 fail_domains × 1 vdisk_location each.
	if c := strings.Count(out, "vdisk_locations:"); c != 9 {
		t.Errorf("expected 9 vdisk_locations (3 hosts × 3 pdisks), got %d", c)
	}
}

func TestRenderYDBStorageConf_Mirror3DCDiskLocations(t *testing.T) {
	out := RenderYDBStorageConf(RenderYDBConfOpts{
		HostCount:         3,
		HostLocations:     []string{"ru-central1-a", "ru-central1-b", "ru-central1-c"},
		MemoryMB:          4096,
		FaultTolerance:    "mirror-3-dc",
		FailureDomainType: "disk",
		DefaultDiskType:   "SSD",
		BlockDevicePaths: []string{
			"/dev/disk/by-id/virtio-ydb-data-0",
			"/dev/disk/by-id/virtio-ydb-data-1",
			"/dev/disk/by-id/virtio-ydb-data-2",
		},
	})
	for _, loc := range []string{"data_center: 'ru-central1-a'", "data_center: 'ru-central1-b'", "data_center: 'ru-central1-c'"} {
		if !strings.Contains(out, loc) {
			t.Fatalf("missing location %q:\n%s", loc, out)
		}
	}
	if strings.Count(out, "      - fail_domains:") != 3 {
		t.Fatalf("expected 3 rings for mirror-3-dc, got:\n%s", out)
	}
	if c := strings.Count(out, "vdisk_locations:"); c != 9 {
		t.Fatalf("expected 9 vdisk locations, got %d:\n%s", c, out)
	}
}

func TestRenderYDBStorageConf_FileBackedFallback(t *testing.T) {
	out := RenderYDBStorageConf(RenderYDBConfOpts{HostCount: 1, MemoryMB: 1024})
	if !strings.Contains(out, "path: /ydb_data/pdisk.data\n") {
		t.Errorf("file-backed pdisk path expected when BlockDevicePath is empty:\n%s", out)
	}
}

func TestSubstituteYDBHostPlaceholders(t *testing.T) {
	body := RenderYDBStorageConf(RenderYDBConfOpts{HostCount: 2, MemoryMB: 1024})
	got := SubstituteYDBHostPlaceholders(body, []string{"node-a.local", "node-b.local"})
	if !strings.Contains(got, "host: node-a.local") || !strings.Contains(got, "host: node-b.local") {
		t.Errorf("substitution failed:\n%s", got)
	}
	if strings.Contains(got, ydbHostPlaceholder(0)) || strings.Contains(got, ydbHostPlaceholder(1)) {
		t.Errorf("placeholders still present after substitution:\n%s", got)
	}
}
