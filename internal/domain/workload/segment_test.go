package workload

import "testing"

func TestSegmentConfigFileNameKeepsLegacyNameForFirstSegment(t *testing.T) {
	if got := SegmentConfigFileName(0); got != "stroppy-config.json" {
		t.Fatalf("segment 0 must keep legacy config name, got %q", got)
	}
	if got := SegmentConfigFileName(1); got != "stroppy-config-1.json" {
		t.Fatalf("segment 1 config name = %q, want stroppy-config-1.json", got)
	}
	if got := SegmentConfigFileName(2); got != "stroppy-config-2.json" {
		t.Fatalf("segment 2 config name = %q, want stroppy-config-2.json", got)
	}
}

func TestSegmentConfigPathUnderComponentConfigDir(t *testing.T) {
	got := SegmentConfigPath("workload-runner-1", 1)
	if want := "/etc/stroppy-cloud/workload-runner-1/stroppy-config-1.json"; got != want {
		t.Fatalf("segment config path = %q, want %q", got, want)
	}
}
