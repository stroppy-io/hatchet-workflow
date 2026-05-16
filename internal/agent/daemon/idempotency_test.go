package daemon

import (
	"testing"

	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
)

func TestReportCache_StoreAndLookup(t *testing.T) {
	c := newReportCache(3)
	r := &agentpb.Report{CommandId: "c1", Status: agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED}
	c.Store("c1", r)

	got := c.Lookup("c1")
	if got == nil {
		t.Fatal("expected non-nil cached report")
	}
	if got.CommandId != "c1" {
		t.Fatalf("expected CommandId c1, got %q", got.CommandId)
	}
}

func TestReportCache_Eviction(t *testing.T) {
	c := newReportCache(2)
	c.Store("a", &agentpb.Report{CommandId: "a"})
	c.Store("b", &agentpb.Report{CommandId: "b"})
	c.Store("c", &agentpb.Report{CommandId: "c"}) // should evict "a"

	if c.Lookup("a") != nil {
		t.Fatal("expected 'a' to be evicted")
	}
	if c.Lookup("b") == nil {
		t.Fatal("expected 'b' to still be present")
	}
	if c.Lookup("c") == nil {
		t.Fatal("expected 'c' to be present")
	}
}
