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
