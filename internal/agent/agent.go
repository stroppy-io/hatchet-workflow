// Package agent is the agent process: a poll-only loop that registers, then pulls
// commands (Poll), executes them on-host (opexec), and reports results — the
// server never pushes (D16). A long command keeps its lease alive with periodic
// Report{RUNNING}. The control-plane client is an injected interface (the gRPC
// client is built at the application level).
package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gopherex/xlog"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/agent/opexec"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// Client is the control-plane AgentService surface the loop calls (poll-only).
type Client interface {
	Register(ctx context.Context, req *agentpb.RegisterRequest) (*models.Agent, error)
	Heartbeat(ctx context.Context, req *agentpb.HeartbeatRequest) (*models.Agent, error)
	Poll(ctx context.Context, req *agentpb.PollRequest) (*agentpb.PollResponse, error)
	Report(ctx context.Context, req *agentpb.ReportRequest) (*emptypb.Empty, error)
	SendLogs(ctx context.Context, req *agentpb.SendLogsRequest) (*emptypb.Empty, error)
}

// Config identifies this agent and tunes the loop cadence.
type Config struct {
	TenantID         string
	MachineID        string
	AgentComponentID string
	Host             string
	Port             uint32
	Version          string
	BootID           string

	PollInterval      time.Duration // wait between empty polls
	HeartbeatInterval time.Duration // liveness cadence
	Keepalive         time.Duration // Report{RUNNING} cadence (< server lease TTL)
}

func (c Config) withDefaults() Config {
	if c.PollInterval <= 0 {
		c.PollInterval = 2 * time.Second
	}
	if c.HeartbeatInterval <= 0 {
		c.HeartbeatInterval = 15 * time.Second
	}
	if c.Keepalive <= 0 {
		c.Keepalive = 20 * time.Second
	}
	return c
}

// Agent runs the poll loop.
type Agent struct {
	*tracing.Entity
	client Client
	exec   *opexec.Executor
	cfg    Config
}

// New builds an Agent over the control-plane client.
func New(logger *xlog.Logger, client Client, cfg Config) *Agent {
	return &Agent{
		Entity: tracing.NewEntity(logger.AppendName("Agent")),
		client: client,
		exec:   opexec.New(),
		cfg:    cfg.withDefaults(),
	}
}

func (a *Agent) tenant() *models.TenantId { return &models.TenantId{Value: a.cfg.TenantID} }

func (a *Agent) target() *agent.Target {
	return &agent.Target{MachineId: a.cfg.MachineID, AgentComponentId: a.cfg.AgentComponentID}
}

// Run registers the agent and then polls until ctx is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	if _, err := a.client.Register(ctx, &agentpb.RegisterRequest{
		TenantId: a.tenant(),
		Target:   a.target(),
		Host:     a.cfg.Host,
		Port:     a.cfg.Port,
		Version:  a.cfg.Version,
		BootId:   a.cfg.BootID,
	}); err != nil {
		return err
	}

	go a.heartbeatLoop(ctx)

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		resp, err := a.client.Poll(ctx, &agentpb.PollRequest{
			TenantId: a.tenant(),
			Target:   a.target(),
			Host:     a.cfg.Host,
			Port:     a.cfg.Port,
		})
		if err != nil {
			a.Logger().Warn("poll failed", xlog.Error("error", err))
			if sleep(ctx, a.cfg.PollInterval) != nil {
				return ctx.Err()
			}
			continue
		}
		if resp.GetLease() == nil {
			if sleep(ctx, a.cfg.PollInterval) != nil {
				return ctx.Err()
			}
			continue
		}
		a.handleLease(ctx, resp.GetLease())
	}
}

func (a *Agent) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(a.cfg.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := a.client.Heartbeat(ctx, &agentpb.HeartbeatRequest{
				TenantId: a.tenant(),
				Target:   a.target(),
				Status:   agent.AgentStatus_AGENT_STATUS_READY,
			}); err != nil {
				a.Logger().Warn("heartbeat failed", xlog.Error("error", err))
			}
		}
	}
}

// handleLease executes the leased command, holding the lease with Report{RUNNING}
// keepalives until it finishes, then reports the terminal status.
func (a *Agent) handleLease(ctx context.Context, lease *agentpb.CommandLease) {
	type outcome struct {
		result *ops.Operation_Result
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := a.exec.Execute(ctx, lease.GetCommand().GetOperation())
		done <- outcome{result: res, err: err}
	}()

	ticker := time.NewTicker(a.cfg.Keepalive)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.report(ctx, lease, agent.CommandStatus_COMMAND_STATUS_RUNNING, nil, "")
		case o := <-done:
			// Ship stdout/stderr to the control plane regardless of outcome, so a
			// FAILED command is diagnosable from the run logs.
			a.ingestLogs(ctx, lease, o.result)
			switch {
			case o.err != nil:
				a.report(ctx, lease, agent.CommandStatus_COMMAND_STATUS_FAILED, o.result, o.err.Error())
			case o.result.GetRunCmd().GetExitCode() != 0:
				rc := o.result.GetRunCmd()
				a.report(ctx, lease, agent.CommandStatus_COMMAND_STATUS_FAILED, o.result,
					fmt.Sprintf("command exited %d: %s", rc.GetExitCode(), lastLines(rc.GetStderr(), 400)))
			default:
				a.report(ctx, lease, agent.CommandStatus_COMMAND_STATUS_COMPLETED, o.result, "")
			}
			return
		}
	}
}

func (a *Agent) report(ctx context.Context, lease *agentpb.CommandLease, st agent.CommandStatus, result *ops.Operation_Result, errMsg string) {
	if _, err := a.client.Report(ctx, &agentpb.ReportRequest{
		TenantId: a.tenant(),
		Address:  lease.GetAddress(),
		Report: &agent.Report{
			CommandId:  lease.GetCommand().GetId(),
			Target:     a.target(),
			Status:     st,
			Result:     result,
			Error:      errMsg,
			ReportedAt: timestamppb.Now(),
		},
	}); err != nil {
		a.Logger().Warn("report failed", xlog.Error("error", err))
	}
}

// ingestLogs ships a finished command's stdout/stderr to the log store, labeled
// with the dag/node from the lease so the control plane can query by run.
func (a *Agent) ingestLogs(ctx context.Context, lease *agentpb.CommandLease, result *ops.Operation_Result) {
	cmd := result.GetRunCmd()
	if cmd == nil {
		return
	}
	// Correlate every line to its run (dag) + node + machine/component so logs are
	// filterable along every axis. node_execution_id is the always-non-empty command id.
	addr := lease.GetAddress()
	nodeExecID := addr.GetNodeExecutionId()
	dagID := addr.GetDagId().GetValue()
	now := timestamppb.Now()
	var lines []*agent.LogLine
	emit := func(data []byte, stream agent.LogLine_Stream) {
		for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
			if line == "" {
				continue
			}
			lines = append(lines, &agent.LogLine{
				CommandId:       nodeExecID,
				DagId:           dagID,
				NodeExecutionId: nodeExecID,
				Target:          a.target(),
				Stream:          stream,
				Line:            line,
				ObservedAt:      now,
			})
		}
	}
	emit(cmd.GetStdout(), agent.LogLine_STREAM_STDOUT)
	emit(cmd.GetStderr(), agent.LogLine_STREAM_STDERR)
	if len(lines) == 0 {
		return
	}
	// Ship to the control plane (server → VictoriaLogs); the agent never touches VL.
	if _, err := a.client.SendLogs(ctx, &agentpb.SendLogsRequest{TenantId: a.tenant(), Lines: lines}); err != nil {
		a.Logger().Warn("send logs failed", xlog.Error("error", err))
	}
}

// lastLines returns the trailing n bytes of b (the most relevant part of stderr).
func lastLines(b []byte, n int) string {
	if len(b) > n {
		b = b[len(b)-n:]
	}
	return strings.TrimSpace(string(b))
}

// sleep waits d or returns ctx.Err() if cancelled first.
func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
