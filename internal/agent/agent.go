// Package agent is the agent process: a poll-only loop that registers, then pulls
// commands (Poll), executes them on-host (opexec), and reports results — the
// server never pushes (D16). A long command keeps its lease alive with periodic
// Report{RUNNING}. The control-plane client is an injected interface (the gRPC
// client is built at the application level).
package agent

import (
	"context"
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

// LogIngester ships command output to the log store with full dag labels (the
// agent knows dag_id/node_execution_id from the lease). Optional; nil disables
// direct ingest. Implemented by internal/infrastructure/victoria.LogsClient via
// a thin adapter at wiring.
type LogIngester interface {
	Ingest(ctx context.Context, records []map[string]any) error
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
	logs   LogIngester
	cfg    Config
}

// New builds an Agent. logs may be nil (direct log ingest disabled).
func New(logger *xlog.Logger, client Client, logs LogIngester, cfg Config) *Agent {
	return &Agent{
		Entity: tracing.NewEntity(logger.AppendName("Agent")),
		client: client,
		exec:   opexec.New(),
		logs:   logs,
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
			if o.err != nil {
				a.report(ctx, lease, agent.CommandStatus_COMMAND_STATUS_FAILED, nil, o.err.Error())
			} else {
				a.ingestLogs(ctx, lease, o.result)
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
	if a.logs == nil {
		return
	}
	cmd := result.GetRunCmd()
	if cmd == nil {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	addr := lease.GetAddress()
	var records []map[string]any
	emit := func(data []byte, stream string) {
		for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
			if line == "" {
				continue
			}
			records = append(records, map[string]any{
				"_msg":              line,
				"_time":             now,
				"dag_id":            addr.GetDagId().GetValue(),
				"node_execution_id": addr.GetNodeExecutionId(),
				"machine_id":        a.cfg.MachineID,
				"stream":            stream,
			})
		}
	}
	emit(cmd.GetStdout(), "stdout")
	emit(cmd.GetStderr(), "stderr")
	if len(records) == 0 {
		return
	}
	if err := a.logs.Ingest(ctx, records); err != nil {
		a.Logger().Warn("log ingest failed", xlog.Error("error", err))
	}
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
