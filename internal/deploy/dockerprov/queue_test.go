package dockerprov

import "testing"

// TestAgentQueue locks the per-machine queue naming: deterministic, unique per
// (run, instance). The deployer and the workflow must agree on this exactly, or
// a session would target a queue no agent listens on.
func TestAgentQueue(t *testing.T) {
	q := AgentQueue("run1", "db")
	if q != "stroppy-agent-run1-db" {
		t.Fatalf("AgentQueue = %q", q)
	}
	// Deterministic.
	if AgentQueue("run1", "db") != q {
		t.Fatal("AgentQueue not deterministic")
	}
	// Distinct per instance and per run.
	if AgentQueue("run1", "db") == AgentQueue("run1", "wl") {
		t.Fatal("queues must differ per instance")
	}
	if AgentQueue("run1", "db") == AgentQueue("run2", "db") {
		t.Fatal("queues must differ per run")
	}
}
