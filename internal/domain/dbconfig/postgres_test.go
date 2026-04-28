package dbconfig

import (
	"strings"
	"testing"
)

func TestRenderPostgresConf_DeterministicOrdering(t *testing.T) {
	opts := RenderPostgresConfOpts{Version: "16", Role: "master", TotalMemoryMB: 4096}
	a := RenderPostgresConf(opts)
	b := RenderPostgresConf(opts)
	if a != b {
		t.Fatalf("renderer not deterministic across calls")
	}
}

func TestRenderPostgresConf_OptionsOverrideDefaults(t *testing.T) {
	out := RenderPostgresConf(RenderPostgresConfOpts{
		Version:       "16",
		Role:          "master",
		TotalMemoryMB: 4096,
		Options:       map[string]string{"shared_buffers": "1234MB", "max_connections": "777"},
	})
	if !strings.Contains(out, "shared_buffers = 1234MB\n") {
		t.Errorf("user shared_buffers not honoured:\n%s", out)
	}
	if !strings.Contains(out, "max_connections = 777\n") {
		t.Errorf("user max_connections not honoured:\n%s", out)
	}
}

func TestRenderPostgresConf_PercentResolved(t *testing.T) {
	out := RenderPostgresConf(RenderPostgresConfOpts{
		Version:       "16",
		Role:          "master",
		TotalMemoryMB: 4096,
		Options:       map[string]string{"shared_buffers": "25%"},
	})
	// 25% of 4096 = 1024, capped at 2048; floor of 32 doesn't apply.
	if !strings.Contains(out, "shared_buffers = 1024MB\n") {
		t.Errorf("expected resolved 25%% of 4096MB to be 1024MB, got:\n%s", out)
	}
}

func TestRenderPostgresConf_PatroniSkipsReplicationDefaults(t *testing.T) {
	withPatroni := RenderPostgresConf(RenderPostgresConfOpts{Version: "16", Role: "master", Patroni: true, TotalMemoryMB: 4096})
	withoutPatroni := RenderPostgresConf(RenderPostgresConfOpts{Version: "16", Role: "master", Patroni: false, TotalMemoryMB: 4096})
	if strings.Contains(withPatroni, "wal_level = replica") {
		t.Errorf("Patroni-managed config should not force wal_level itself")
	}
	if !strings.Contains(withoutPatroni, "wal_level = replica") {
		t.Errorf("non-Patroni master should set wal_level = replica")
	}
}

func TestRenderPostgresConf_HotStandbyOnReplicaWithoutPatroni(t *testing.T) {
	r := RenderPostgresConf(RenderPostgresConfOpts{Version: "16", Role: "replica", Patroni: false, TotalMemoryMB: 4096})
	if !strings.Contains(r, "hot_standby = on") {
		t.Errorf("non-Patroni replica missing hot_standby = on:\n%s", r)
	}
}

func TestResolveMemoryPercents_FloorsAndCaps(t *testing.T) {
	m := map[string]string{
		"low":  "1%",    // 1% of 100 = 1, floor at 32
		"high": "5000%", // 5000% of 100 = 5000, cap at 2048
		"abs":  "64MB",  // not a percent — leave alone
	}
	ResolveMemoryPercents(m, 100)
	if m["low"] != "32MB" {
		t.Errorf("low: want 32MB, got %s", m["low"])
	}
	if m["high"] != "2048MB" {
		t.Errorf("high: want 2048MB, got %s", m["high"])
	}
	if m["abs"] != "64MB" {
		t.Errorf("abs: should be untouched, got %s", m["abs"])
	}
}
