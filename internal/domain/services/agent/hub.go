package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
)

// Hub bridges DAG node handlers (server side) and connected agents.
// Commands are enqueued per agent and drained on each Poll call.
// Dispatch blocks until the agent posts a Report for the command or the
// context/timeout fires.
//
// Machine bindings: each (dag_run_id, machine_id) → agent_id mapping is
// established by agent.Service.Register when the bootstrap JWT verifies.
// Handlers call ResolveByMachine at execute time to find the live agent.
type Hub struct {
	mu              sync.Mutex
	queues          map[string][]*agentpb.Command   // agent_id → pending Commands FIFO
	pending         map[string]chan *agentpb.Report // command_id → reply waiter
	machineBindings sync.Map                        // "<dag_run_id>|<machine_id>" → agent_id
}

func NewHub() *Hub {
	return &Hub{
		queues:  map[string][]*agentpb.Command{},
		pending: map[string]chan *agentpb.Report{},
	}
}

// Bind associates a (dag_run_id, machine_id) with a freshly-registered agent.
// Called by agent.Service.Register after the bootstrap token verifies.
func (h *Hub) Bind(dagRunID, machineID, agentID string) {
	if dagRunID == "" || machineID == "" || agentID == "" {
		return
	}
	h.machineBindings.Store(dagRunID+"|"+machineID, agentID)
}

// ResolveByMachine returns the agent_id bound to (dag_run_id, machine_id).
// Returns ("", false) when no agent has registered for that pair yet.
func (h *Hub) ResolveByMachine(dagRunID, machineID string) (string, bool) {
	v, ok := h.machineBindings.Load(dagRunID + "|" + machineID)
	if !ok {
		return "", false
	}
	id, _ := v.(string)
	return id, id != ""
}

// ClearByAgent removes every (dag_run_id, machine_id) → agent_id binding for
// the given agent. Called by agent.Service.Deregister so subsequent handler
// resolves return ("", false) instead of dispatching to a dead agent.
func (h *Hub) ClearByAgent(agentID string) {
	if agentID == "" {
		return
	}
	h.machineBindings.Range(func(k, v any) bool {
		if id, ok := v.(string); ok && id == agentID {
			h.machineBindings.Delete(k)
		}
		return true
	})
}

// Dispatch enqueues a command for the given agent and blocks until the
// agent's next Poll posts a Report for the resulting command_id.
// timeout caps the wait; returns ctx.Err() on cancellation.
func (h *Hub) Dispatch(ctx context.Context, agentID string, machineID string, action *agentpb.Action, timeout time.Duration) (*agentpb.Report, error) {
	cmd := &agentpb.Command{
		Id:        ids.New(),
		MachineId: machineID,
		Action:    action,
	}
	waiter := make(chan *agentpb.Report, 1)

	h.mu.Lock()
	h.queues[agentID] = append(h.queues[agentID], cmd)
	h.pending[cmd.Id] = waiter
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.pending, cmd.Id)
		h.mu.Unlock()
	}()

	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case r := <-waiter:
		return r, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, fmt.Errorf("hub: dispatch timeout for command %s", cmd.Id)
	}
}

// Drain returns all pending commands for an agent and clears the queue.
func (h *Hub) Drain(agentID string) []*agentpb.Command {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := h.queues[agentID]
	h.queues[agentID] = nil
	return out
}

// Resolve completes a pending command waiter with the given report.
func (h *Hub) Resolve(commandID string, report *agentpb.Report) {
	h.mu.Lock()
	ch, ok := h.pending[commandID]
	if ok {
		delete(h.pending, commandID)
	}
	h.mu.Unlock()
	if ok {
		select {
		case ch <- report:
		default:
		}
	}
}
