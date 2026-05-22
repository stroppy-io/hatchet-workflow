package agent

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/agent"
	rtagent "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// CommandQueue is the runtime command-queue seam: the queue IS the set of ready
// agent-locus Dag nodes (no separate table, H19). Implemented by services/agentqueue
// over the dagstore: it selects the next ready AGENT-locus node for the machine
// across the tenant's running Dags, leases it (NodeAddress + lease_expires_at),
// resolves binding holes from the deployment Output, advances the node on Report,
// and invalidates in-flight leases on a new boot_id (H20/H21/H26).
type CommandQueue interface {
	// Lease hands out the next ready command for the target machine, or (nil, nil)
	// when there is no work (long-poll timeout).
	Lease(ctx context.Context, tenantID string, target *rtagent.Target) (*agentpb.CommandLease, error)
	// Report applies a command report (RUNNING keepalive / terminal) to the node.
	Report(ctx context.Context, tenantID string, addr *agentpb.NodeAddress, report *rtagent.Report) error
	// InvalidateLeases drops the machine's in-flight leases (new boot_id).
	InvalidateLeases(ctx context.Context, tenantID, machineID string) error
}

// LogsSink ingests agent-shipped log lines into the log store. Implemented by
// services/logs over the VictoriaLogs JSON-stream ingest path (H54/H55).
type LogsSink interface {
	SendLogs(ctx context.Context, tenantID string, lines []*rtagent.LogLine) error
}

// Poll long-polls for the next queued command for the agent's machine.
func (s *AgentService) Poll(ctx context.Context, req *agentpb.PollRequest) (*agentpb.PollResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "Poll",
		func(ctx context.Context, _ trace.Span) (*agentpb.PollResponse, error) {
			if _, err := requireAgentPrincipal(ctx, req.GetTenantId()); err != nil {
				return nil, err
			}
			lease, err := s.queue.Lease(ctx, req.GetTenantId().GetValue(), req.GetTarget())
			if err != nil {
				return nil, err
			}
			return &agentpb.PollResponse{Lease: lease}, nil
		})
}

// Report applies a command result or keepalive from the agent.
func (s *AgentService) Report(ctx context.Context, req *agentpb.ReportRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "Report",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			if _, err := requireAgentPrincipal(ctx, req.GetTenantId()); err != nil {
				return nil, err
			}
			if err := s.queue.Report(ctx, req.GetTenantId().GetValue(), req.GetAddress(), req.GetReport()); err != nil {
				return nil, err
			}
			return &emptypb.Empty{}, nil
		})
}

// SendLogs ships a batch of agent log lines to the log store.
func (s *AgentService) SendLogs(ctx context.Context, req *agentpb.SendLogsRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "SendLogs",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			if _, err := requireAgentPrincipal(ctx, req.GetTenantId()); err != nil {
				return nil, err
			}
			if err := s.logs.SendLogs(ctx, req.GetTenantId().GetValue(), req.GetLines()); err != nil {
				return nil, err
			}
			return &emptypb.Empty{}, nil
		})
}
