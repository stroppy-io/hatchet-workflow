package agent

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
)

// PollClient implements the Client interface using a server-side command queue.
// Instead of pushing commands to agents, it places commands in a per-machine
// queue. Agents poll the server for pending commands via /api/agent/poll.
type PollClient struct {
	mu      sync.Mutex
	queues  map[string]chan Command  // machineID → pending command
	results map[string]chan Report   // commandID → report channel
	healthy map[string]chan struct{} // machineID → closed when agent first polls
	logger  *zap.Logger
	cmdSeq  atomic.Int64 // auto-incrementing command ID
}

// NewPollClient creates a poll-based client.
func NewPollClient(logger *zap.Logger) *PollClient {
	return &PollClient{
		queues:  make(map[string]chan Command),
		results: make(map[string]chan Report),
		healthy: make(map[string]chan struct{}),
		logger:  logger,
	}
}

// Send enqueues a command for the target agent and waits for the report.
func (c *PollClient) Send(nc *dag.NodeContext, target Target, cmd Command) error {
	log := nc.Log().With(
		zap.String("target", target.ID),
		zap.String("action", string(cmd.Action)),
	)

	// Auto-generate command ID if not set.
	if cmd.ID == "" {
		cmd.ID = fmt.Sprintf("cmd-%s-%d", target.ID, c.cmdSeq.Add(1))
	}

	// Wait for agent to start polling (i.e. become healthy).
	log.Info("waiting for agent to start polling")
	if err := c.waitForAgent(nc, target.ID); err != nil {
		return fmt.Errorf("agent %s did not connect: %w", target.ID, err)
	}
	log.Info("agent connected, sending command")

	// Create result channel for this command.
	resultCh := make(chan Report, 1)
	c.mu.Lock()
	c.results[cmd.ID] = resultCh
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.results, cmd.ID)
		c.mu.Unlock()
	}()

	// Enqueue command — the agent's next poll will pick it up.
	q := c.getQueue(target.ID)
	select {
	case q <- cmd:
	case <-nc.Done():
		return nc.Err()
	}

	// Wait for the report from the agent.
	select {
	case report := <-resultCh:
		if report.Status == ReportFailed {
			log.Error("command failed on agent", zap.String("error", report.Error))
			return fmt.Errorf("agent %s: command %s failed: %s", target.ID, cmd.ID, report.Error)
		}
		log.Info("command completed",
			zap.String("status", string(report.Status)),
		)
		return nil
	case <-nc.Done():
		return nc.Err()
	}
}

// SendAll dispatches the same command to all targets in parallel.
func (c *PollClient) SendAll(nc *dag.NodeContext, targets []Target, cmd Command) error {
	if len(targets) == 0 {
		return nil
	}
	if len(targets) == 1 {
		return c.Send(nc, targets[0], cmd)
	}

	ctx, cancel := context.WithCancel(nc)
	defer cancel()

	var (
		once     sync.Once
		firstErr error
		wg       sync.WaitGroup
	)

	childNC := nc.WithContext(ctx)

	for _, t := range targets {
		wg.Add(1)
		go func(target Target) {
			defer wg.Done()
			if err := c.Send(childNC, target, cmd); err != nil {
				once.Do(func() {
					firstErr = err
					cancel()
				})
			}
		}(t)
	}

	wg.Wait()
	return firstErr
}

// Poll is called by the server's /api/agent/poll handler.
// Returns the next pending command for the machine, or nil if none.
// Blocks for up to timeout waiting for a command (long-poll).
func (c *PollClient) Poll(machineID string, timeout time.Duration) *Command {
	// Mark agent as healthy on first poll.
	c.mu.Lock()
	if ch, ok := c.healthy[machineID]; ok {
		select {
		case <-ch:
			// already closed
		default:
			close(ch)
		}
	}
	c.mu.Unlock()

	q := c.getQueue(machineID)
	select {
	case cmd := <-q:
		return &cmd
	case <-time.After(timeout):
		return nil
	}
}

// DeliverReport routes a report from an agent to the waiting Send() call.
func (c *PollClient) DeliverReport(report Report) {
	c.mu.Lock()
	ch, ok := c.results[report.CommandID]
	c.mu.Unlock()

	if ok {
		select {
		case ch <- report:
		default:
		}
	}
}

// MarkAgentReady pre-creates the healthy channel so waitForAgent can detect the agent.
func (c *PollClient) MarkAgentReady(machineID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.healthy[machineID]; !ok {
		c.healthy[machineID] = make(chan struct{})
	}
}

func (c *PollClient) getQueue(machineID string) chan Command {
	c.mu.Lock()
	defer c.mu.Unlock()
	q, ok := c.queues[machineID]
	if !ok {
		q = make(chan Command, 8)
		c.queues[machineID] = q
	}
	return q
}

func (c *PollClient) waitForAgent(ctx context.Context, machineID string) error {
	c.mu.Lock()
	ch, ok := c.healthy[machineID]
	if !ok {
		ch = make(chan struct{})
		c.healthy[machineID] = ch
	}
	c.mu.Unlock()

	deadline := time.After(120 * time.Second)
	select {
	case <-ch:
		return nil
	case <-deadline:
		return fmt.Errorf("agent %s did not poll within 120s", machineID)
	case <-ctx.Done():
		return ctx.Err()
	}
}
