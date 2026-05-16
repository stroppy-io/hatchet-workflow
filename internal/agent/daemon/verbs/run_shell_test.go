package verbs

import (
	"context"
	"testing"

	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
)

func TestRunShell_Echo(t *testing.T) {
	action := &agentpb.Action{
		Verb: &agentpb.Action_RunShell{
			RunShell: &agentpb.RunShell{Argv: []string{"echo", "hello"}},
		},
	}
	rep := RunShell(context.Background(), action)
	if rep.Status != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
		t.Fatalf("expected SUCCEEDED, got %v; error: %s", rep.Status, rep.Error)
	}
}

func TestRunShell_FailCommand(t *testing.T) {
	action := &agentpb.Action{
		Verb: &agentpb.Action_RunShell{
			RunShell: &agentpb.RunShell{Argv: []string{"false"}},
		},
	}
	rep := RunShell(context.Background(), action)
	if rep.Status != agentpb.ReportStatus_REPORT_STATUS_FAILED {
		t.Fatalf("expected FAILED, got %v", rep.Status)
	}
}

func TestRunShell_ExpectedExit(t *testing.T) {
	action := &agentpb.Action{
		Verb: &agentpb.Action_RunShell{
			RunShell: &agentpb.RunShell{
				Argv:          []string{"false"},
				ExpectedExits: []int32{1},
			},
		},
	}
	rep := RunShell(context.Background(), action)
	if rep.Status != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
		t.Fatalf("expected SUCCEEDED for exit=1 in expected_exits, got %v", rep.Status)
	}
}

func TestRunShell_EmptyArgv(t *testing.T) {
	action := &agentpb.Action{
		Verb: &agentpb.Action_RunShell{RunShell: &agentpb.RunShell{}},
	}
	rep := RunShell(context.Background(), action)
	if rep.Status != agentpb.ReportStatus_REPORT_STATUS_FAILED {
		t.Fatalf("expected FAILED for empty argv, got %v", rep.Status)
	}
}

func TestRunShell_ShellMode(t *testing.T) {
	action := &agentpb.Action{
		Verb: &agentpb.Action_RunShell{
			RunShell: &agentpb.RunShell{
				Argv:  []string{"echo hello world"},
				Shell: true,
			},
		},
	}
	rep := RunShell(context.Background(), action)
	if rep.Status != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
		t.Fatalf("expected SUCCEEDED in shell mode, got %v; err: %s", rep.Status, rep.Error)
	}
}
