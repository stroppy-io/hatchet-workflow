package daemon

import (
	"context"
	"testing"

	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
)

func TestDispatcher_RegisterAndDispatch(t *testing.T) {
	d := NewDispatcher()
	d.Register("RunShell", func(ctx context.Context, action *agentpb.Action) *agentpb.Report {
		return &agentpb.Report{
			Status: agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED,
			Output: []byte("hello"),
		}
	})

	cmd := &agentpb.Command{
		Id:        "cmd-001",
		MachineId: "machine-1",
		Action:    &agentpb.Action{Verb: &agentpb.Action_RunShell{RunShell: &agentpb.RunShell{Argv: []string{"echo", "hello"}}}},
	}

	rep := d.Dispatch(context.Background(), cmd)
	if rep.CommandId != "cmd-001" {
		t.Fatalf("expected CommandId cmd-001, got %q", rep.CommandId)
	}
	if rep.MachineId != "machine-1" {
		t.Fatalf("expected MachineId machine-1, got %q", rep.MachineId)
	}
	if rep.Status != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
		t.Fatalf("expected SUCCEEDED, got %v", rep.Status)
	}
}

func TestDispatcher_UnknownVerb(t *testing.T) {
	d := NewDispatcher()
	cmd := &agentpb.Command{
		Id:     "cmd-002",
		Action: &agentpb.Action{Verb: &agentpb.Action_RunShell{RunShell: &agentpb.RunShell{Argv: []string{"ls"}}}},
	}
	rep := d.Dispatch(context.Background(), cmd)
	if rep.Status != agentpb.ReportStatus_REPORT_STATUS_FAILED {
		t.Fatalf("expected FAILED for unknown verb, got %v", rep.Status)
	}
}

func TestDispatcher_DrainReports(t *testing.T) {
	d := NewDispatcher()
	d.QueueReport(&agentpb.Report{CommandId: "c1"})
	d.QueueReport(&agentpb.Report{CommandId: "c2"})

	ar := d.DrainReports()
	if len(ar.Reports) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(ar.Reports))
	}
	// Second drain should be empty.
	ar2 := d.DrainReports()
	if len(ar2.Reports) != 0 {
		t.Fatalf("expected 0 reports after drain, got %d", len(ar2.Reports))
	}
}
