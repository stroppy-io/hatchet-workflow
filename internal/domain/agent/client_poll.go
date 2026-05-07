package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
)

// PollClient implements the Client interface using a server-side command queue.
// Instead of pushing commands to agents, it places commands in a per-machine
// queue. Agents poll the server for pending commands via /api/agent/poll.
//
// Durability: when a postgres pool is wired via SetPool, every dispatched
// command is persisted in agent_commands BEFORE being made visible to the
// agent. If the server dies mid-flight, the orphan-reaper running on
// another (or restarted) server picks the row up via FOR UPDATE SKIP
// LOCKED. The in-memory queues + result channels stay as the hot path for
// in-process delivery; DB is the durability backstop.
type PollClient struct {
	mu      sync.Mutex
	queues  map[string]chan Command  // machineID → pending command
	results map[string]chan Report   // commandID → report channel
	healthy map[string]chan struct{} // machineID → closed when agent first polls
	logger  *zap.Logger
	cmdSeq  atomic.Int64 // auto-incrementing command ID

	// Optional durable backing. Setting both enables DB-persisted
	// dispatch + cross-restart pickup. Tests/CLI mode leaves them nil and
	// run pure in-memory.
	pool       *pgxpool.Pool
	instanceID string
}

// SetPool wires the durable agent_commands queue. Server.NewServer calls
// this once after constructing the scheduler so PollClient and scheduler
// share the same instance UUID for orphan detection.
func (c *PollClient) SetPool(pool *pgxpool.Pool, instanceID string) {
	c.pool = pool
	c.instanceID = instanceID
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

	// Persist the command BEFORE making it claimable in-memory. If the
	// server dies before the in-memory enqueue, the row stays
	// state='pending' and a recovered server's worker (after RecoverRun
	// re-issues commands) just bypasses it. If the server dies AFTER the
	// agent picked it up but before the report arrives, the row is
	// state='claimed' with a dead claimed_by — the next server's reaper
	// either revives or marks cancelled, and the recovered DAG re-issues a
	// fresh one. Either way: no silent drop.
	runID := extractRunIDFromMachine(target.ID)
	if c.pool != nil {
		payload, _ := json.Marshal(cmd)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_, err := c.pool.Exec(ctx, `
			INSERT INTO agent_commands (run_id, machine_id, payload, state, created_at)
			VALUES ($1, $2, $3, 'pending', NOW())`, runID, target.ID, string(payload))
		cancel()
		if err != nil {
			log.Warn("agent_commands persist failed (proceeding via in-memory)", zap.Error(err))
		}
	}

	// Enqueue command — the agent's next poll will pick it up.
	q := c.getQueue(target.ID)
	select {
	case q <- cmd:
	case <-nc.Done():
		c.markCmdCancelled(cmd.ID)
		return nc.Err()
	}

	// Wait for the report. Channel-based hot path (this server delivered
	// it) OR DB-poll fallback (cross-server delivery — another instance
	// processed the report).
	pollT := time.NewTicker(2 * time.Second)
	defer pollT.Stop()
	for {
		select {
		case report := <-resultCh:
			if report.Status == ReportFailed {
				log.Error("command failed on agent", zap.String("error", report.Error))
				return fmt.Errorf("agent %s: command %s failed: %s", target.ID, cmd.ID, report.Error)
			}
			log.Info("command completed", zap.String("status", string(report.Status)))
			return nil
		case <-pollT.C:
			if c.pool == nil {
				continue
			}
			st, errStr, ok := c.lookupCommandState(cmd.ID)
			if !ok {
				continue
			}
			switch st {
			case "done":
				log.Info("command completed (DB-detected)")
				return nil
			case "failed":
				return fmt.Errorf("agent %s: command %s failed: %s", target.ID, cmd.ID, errStr)
			case "cancelled":
				return fmt.Errorf("agent %s: command %s cancelled", target.ID, cmd.ID)
			}
		case <-nc.Done():
			c.markCmdCancelled(cmd.ID)
			return nc.Err()
		}
	}
}

// extractRunIDFromMachine pulls the leading "run-…" prefix off a machine
// identifier (e.g. "run-1777999-stroppy-0" → "run-1777999"). Used as the
// agent_commands.run_id correlator without needing the full RunConfig
// here. Falls back to the whole id if no obvious prefix is found.
func extractRunIDFromMachine(machineID string) string {
	// Reuses the same convention as the existing extractRunID in api/.
	for i := len(machineID) - 1; i >= 0; i-- {
		if machineID[i] == '-' {
			// strip the trailing "-stroppy-0" / "-database-0" / etc
			suffix := machineID[i+1:]
			isNum := suffix != ""
			for _, r := range suffix {
				if r < '0' || r > '9' {
					isNum = false
					break
				}
			}
			if isNum {
				return extractRunIDFromMachine(machineID[:i])
			}
			return machineID[:i]
		}
	}
	return machineID
}

func (c *PollClient) markCmdCancelled(cmdID string) {
	if c.pool == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = c.pool.Exec(ctx, `
		UPDATE agent_commands SET state='cancelled', completed_at=NOW()
		WHERE payload::text LIKE '%' || $1 || '%' AND state IN ('pending','claimed')`, cmdID)
}

// lookupCommandState polls agent_commands for a terminal state of cmd. The
// LIKE on payload::text is cheap relative to scrape rate and avoids
// requiring a separate cmd_id column.
func (c *PollClient) lookupCommandState(cmdID string) (state, errStr string, ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := c.pool.QueryRow(ctx, `
		SELECT state, COALESCE(error,'') FROM agent_commands
		WHERE payload::text LIKE '%' || $1 || '%'
		ORDER BY id DESC LIMIT 1`, cmdID,
	).Scan(&state, &errStr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", false
		}
		return "", "", false
	}
	return state, errStr, true
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
	// Mark agent as healthy on first poll. Auto-creates the channel if it
	// doesn't exist yet — this matters after a server restart, where
	// agents resume polling before any task has called waitForAgent for
	// them. Without auto-create, Send() would wait 120s before noticing
	// the agent already came back.
	c.mu.Lock()
	ch, ok := c.healthy[machineID]
	if !ok {
		ch = make(chan struct{})
		c.healthy[machineID] = ch
	}
	select {
	case <-ch:
		// already closed
	default:
		close(ch)
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

// waitForAgentTimeout sets the upper bound on how long Send() blocks waiting
// for the target agent to start polling. Generous because of the recovery
// path: when the orchestrator restarts mid-run, an agent that was OOM-killed
// or hit by a botched teardown needs systemd to restart it, the new process
// to come up, and the poll loop to reach the new server. 3 minutes covers
// the slow cases without making fresh provisions feel broken.
const waitForAgentTimeout = 3 * time.Minute

// firstWarnAfter is when we log a heads-up that the agent still hasn't
// connected — useful when triaging "did not poll" failures to know whether
// the connect was instant-but-slow or simply absent.
const firstWarnAfter = 30 * time.Second

func (c *PollClient) waitForAgent(ctx context.Context, machineID string) error {
	c.mu.Lock()
	ch, ok := c.healthy[machineID]
	if !ok {
		ch = make(chan struct{})
		c.healthy[machineID] = ch
	}
	c.mu.Unlock()

	warn := time.After(firstWarnAfter)
	deadline := time.After(waitForAgentTimeout)
	for {
		select {
		case <-ch:
			return nil
		case <-warn:
			c.logger.Warn("still waiting for agent to poll",
				zap.String("machine_id", machineID),
				zap.Duration("waited", firstWarnAfter),
				zap.Duration("deadline", waitForAgentTimeout),
			)
			warn = nil
		case <-deadline:
			return fmt.Errorf("agent %s did not poll within %s", machineID, waitForAgentTimeout)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
