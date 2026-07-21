package workflows

import (
	"testing"
	"time"
)

// A dead agent must not hang a run forever. agentActivityOptions is the safety
// net: if any of these fields is dropped, a retry re-queued onto a dead agent's
// task queue waits indefinitely, the workflow never returns, and its deferred
// infrastructure teardown never runs — leaking cloud machines (exactly the
// stuck-run incident). This test pins the invariant so a future edit can't
// silently reopen the hole.
func TestAgentActivityOptionsBoundDeadAgent(t *testing.T) {
	const startToClose = 30 * 24 * time.Hour // a workload run may last a week
	opts := agentActivityOptions("stroppy-agent-node-1", startToClose)

	if opts.TaskQueue != "stroppy-agent-node-1" {
		t.Fatalf("TaskQueue = %q, want the agent's queue", opts.TaskQueue)
	}
	// StartToClose passes through: it supports long runs and is NOT the safety net.
	if opts.StartToCloseTimeout != startToClose {
		t.Fatalf("StartToCloseTimeout = %s, want %s (passthrough)", opts.StartToCloseTimeout, startToClose)
	}

	// The two fields that actually bound a dead agent.
	if opts.ScheduleToStartTimeout <= 0 {
		t.Fatal("ScheduleToStartTimeout must be set: a retry to a dead agent's queue would otherwise wait forever")
	}
	if opts.RetryPolicy == nil || opts.RetryPolicy.MaximumAttempts <= 0 {
		t.Fatalf("RetryPolicy.MaximumAttempts must be finite, got %+v: unbounded retries hang the workflow", opts.RetryPolicy)
	}
	// Heartbeat still catches an agent that dies mid-activity.
	if opts.HeartbeatTimeout <= 0 {
		t.Fatal("HeartbeatTimeout must be set so a mid-run agent death is detected")
	}

	// Worst-case time to give up on a dead agent stays well under an hour, so a
	// crashed run converges to teardown promptly rather than lingering for days.
	worst := time.Duration(opts.RetryPolicy.MaximumAttempts) * (opts.ScheduleToStartTimeout + opts.RetryPolicy.MaximumInterval)
	if worst > time.Hour {
		t.Fatalf("dead-agent give-up bound = %s, want <= 1h", worst)
	}
}
