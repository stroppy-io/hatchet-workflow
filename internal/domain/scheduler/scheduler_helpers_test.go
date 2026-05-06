package scheduler

import (
	"encoding/json"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
)

// preFailNonTeardown is the recovery escape hatch for unreachable yandex
// runs — flip every non-terminal node to failed EXCEPT teardown, so the
// AlwaysRun teardown still gets a chance to fire.

func TestPreFailNonTeardown_KeepsTeardownPending(t *testing.T) {
	snap := &dag.Snapshot{Nodes: []dag.NodeStatus{
		{ID: "network", Status: dag.StatusDone},
		{ID: "machines", Status: dag.StatusRunning},
		{ID: "install_db", Status: dag.StatusPending},
		{ID: "run_stroppy", Status: dag.StatusPending},
		{ID: "teardown", Status: dag.StatusPending},
	}}
	preFailNonTeardown(snap, "agents unreachable")

	for _, n := range snap.Nodes {
		switch n.ID {
		case "network":
			if n.Status != dag.StatusDone {
				t.Errorf("done node mutated: %s", n.Status)
			}
		case "teardown":
			if n.Status != dag.StatusPending {
				t.Errorf("teardown should stay pending so AlwaysRun fires; got %s", n.Status)
			}
		case "machines", "install_db", "run_stroppy":
			if n.Status != dag.StatusFailed {
				t.Errorf("%s should be failed, got %s", n.ID, n.Status)
			}
			if n.Error == "" {
				t.Errorf("%s missing error message", n.ID)
			}
		}
	}
}

func TestStepTimeoutMinutes_DefaultsToZero(t *testing.T) {
	if got := stepTimeoutMinutes(nil); got != 0 {
		t.Errorf("nil policy = %d, want 0", got)
	}
	if got := stepTimeoutMinutes([]byte(`{}`)); got != 0 {
		t.Errorf("empty policy = %d, want 0", got)
	}
	if got := stepTimeoutMinutes([]byte(`{"step_timeout_min": -5}`)); got != 0 {
		t.Errorf("negative policy clamped to %d, want 0", got)
	}
}

func TestStepTimeoutMinutes_ReadsValue(t *testing.T) {
	policy, _ := json.Marshal(map[string]any{
		"mode":             "parallel",
		"max_parallel":     2,
		"step_timeout_min": 15,
	})
	if got := stepTimeoutMinutes(policy); got != 15 {
		t.Errorf("got %d, want 15", got)
	}
}
