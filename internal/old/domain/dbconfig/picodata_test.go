package dbconfig

import (
	"strings"
	"testing"
)

func TestRenderPicodataConf_AdvertisePlaceholder(t *testing.T) {
	out := RenderPicodataConf(RenderPicodataConfOpts{Replication: 2, TotalMemoryMB: 4096})
	if !strings.Contains(out, "advertise: "+PicodataAdvertiseHostPlaceholder+":3301") {
		t.Errorf("iproto advertise placeholder missing:\n%s", out)
	}
	if !strings.Contains(out, "advertise: "+PicodataAdvertiseHostPlaceholder+":5432") {
		t.Errorf("pg advertise placeholder missing:\n%s", out)
	}
}

func TestRenderPicodataConf_ReplicationFactor(t *testing.T) {
	out := RenderPicodataConf(RenderPicodataConfOpts{Replication: 3, TotalMemoryMB: 4096})
	if !strings.Contains(out, "replication_factor: 3") {
		t.Errorf("replication factor missing:\n%s", out)
	}
}

func TestRenderPicodataConf_DefaultReplicationOnZero(t *testing.T) {
	out := RenderPicodataConf(RenderPicodataConfOpts{Replication: 0, TotalMemoryMB: 4096})
	if !strings.Contains(out, "replication_factor: 2") {
		t.Errorf("zero replication should default to 2:\n%s", out)
	}
}

func TestRenderPicodataConf_MemtxFromOptions(t *testing.T) {
	out := RenderPicodataConf(RenderPicodataConfOpts{
		Replication: 2, TotalMemoryMB: 4096,
		Options: map[string]string{"memtx_memory": "1024MB"},
	})
	wantBytes := 1024 * 1024 * 1024
	if !strings.Contains(out, "memory: ") || !strings.Contains(out, "1073741824") {
		t.Errorf("memtx memory mismatch — want %d:\n%s", wantBytes, out)
	}
}

func TestSubstitutePicodataPlaceholders(t *testing.T) {
	body := "advertise: " + PicodataAdvertiseHostPlaceholder + ":3301\n"
	got := SubstitutePicodataPlaceholders(body, "10.0.0.7")
	if !strings.Contains(got, "advertise: 10.0.0.7:3301") {
		t.Errorf("substitution failed:\n%s", got)
	}
}
