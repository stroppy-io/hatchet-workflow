package handlers

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	taskspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks"
)

// WaitAgentsHandler polls the AgentHub until every machine_id in the task
// has a registered agent for this DagRun, or the timeout elapses.
type WaitAgentsHandler struct {
	hub      HubPort
	pollTick time.Duration
}

func NewWaitAgentsHandler(hub HubPort) *WaitAgentsHandler {
	return &WaitAgentsHandler{hub: hub, pollTick: 2 * time.Second}
}

func (h *WaitAgentsHandler) Kind() string { return "WaitAgentsTask" }

func (h *WaitAgentsHandler) Execute(ctx context.Context, node *systempb.NodeRun, spec *anypb.Any, state nodeworker.StateStore) (*anypb.Any, error) {
	var task taskspb.WaitAgentsTask
	if err := anypb.UnmarshalTo(spec, &task, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("WaitAgentsHandler: unmarshal: %w", err)
	}
	machineIDs := task.GetMachineIds()
	if len(machineIDs) == 0 {
		return nil, fmt.Errorf("WaitAgentsHandler: machine_ids required")
	}
	dagRunID := node.GetDagRunId().GetValue()
	timeout := time.Duration(task.GetTimeoutSeconds()) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}

	deadline := time.Now().Add(timeout)
	for {
		pending := make([]string, 0, len(machineIDs))
		for _, id := range machineIDs {
			if _, ok := h.hub.ResolveByMachine(dagRunID, id); !ok {
				pending = append(pending, id)
			}
		}
		if len(pending) == 0 {
			return nil, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("WaitAgentsHandler: timeout after %s; still missing: %v", timeout, pending)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(h.pollTick):
		}
	}
}
