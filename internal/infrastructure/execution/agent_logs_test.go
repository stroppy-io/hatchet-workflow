package execution

import (
	"testing"

	agentdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
)

func TestNormalizeAgentLogLinesStampsClaims(t *testing.T) {
	lines, err := normalizeAgentLogLines([]*monitor.LogLine{
		{Line: "ok"},
	}, &agentdomain.TokenClaims{
		RunID:     "run-1",
		MachineID: "node-1",
	})
	if err != nil {
		t.Fatalf("normalize agent logs: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("lines = %d, want 1", len(lines))
	}
	if got, want := lines[0].GetRunId(), "run-1"; got != want {
		t.Fatalf("run_id = %q, want %q", got, want)
	}
	if got, want := lines[0].GetMachineId(), "node-1"; got != want {
		t.Fatalf("machine_id = %q, want %q", got, want)
	}
}

func TestNormalizeAgentLogLinesRejectsMismatchedClaims(t *testing.T) {
	_, err := normalizeAgentLogLines([]*monitor.LogLine{
		{RunId: "other-run", MachineId: "node-1", Line: "bad"},
	}, &agentdomain.TokenClaims{
		RunID:     "run-1",
		MachineID: "node-1",
	})
	if err == nil {
		t.Fatal("mismatched run_id was accepted")
	}
}
