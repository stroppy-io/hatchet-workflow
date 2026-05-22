package runtime

import (
	"context"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
)

// TestExecutorParksSubDagAgentNode reproduces the full-run blocker: an agent-locus
// node inside a sub-dag must be parked RUNNING (so the agent can lease it), not left
// PENDING.
func TestExecutorParksSubDagAgentNode(t *testing.T) {
	reg := NewTaskRegistry()

	agentNode := testNode(t, "comp.install", "agent.command", nil)
	agentNode.ExecutionId = "comp.install"
	agentNode.GetTaskState().Locus = primitive.Dag_Node_TaskState_EXECUTION_LOCUS_AGENT

	sub := testDag("sub", agentNode)

	subNode := &primitive.Dag_Node{
		Id:          "installAndRun",
		ExecutionId: "installAndRun",
		Status:      primitive.Status_STATUS_PENDING,
		Scheduling:  &primitive.Dag_Node_Scheduling{},
		Variant:     &primitive.Dag_Node_SubDag{SubDag: sub},
	}
	top := testDag("top", subNode)

	if err := NewExecutor(reg).Run(context.Background(), top); err != nil {
		t.Fatalf("run: %v", err)
	}

	got := top.GetNodes()[0].GetSubDag().GetNodes()[0].GetStatus()
	t.Logf("top=%s installAndRun=%s agentNode=%s",
		top.GetStatus(), top.GetNodes()[0].GetStatus(), got)
	if got != primitive.Status_STATUS_RUNNING {
		t.Fatalf("sub-dag agent node = %s, want RUNNING (parked)", got)
	}
}
