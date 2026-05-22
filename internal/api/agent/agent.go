package agent

import (
	"context"

	"github.com/gopherex/xlog"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// AgentActions is the dependency the AgentService is built on. Agents only poll —
// Register/Heartbeat/Poll/Report/SendLogs are the machine-principal worker API;
// ListAgents/GetAgent are tenant-scoped read. internal/services implements it.
type AgentActions interface {
	Register(ctx context.Context, req *agentpb.RegisterRequest) (*models.Agent, error)
	Heartbeat(ctx context.Context, req *agentpb.HeartbeatRequest) (*models.Agent, error)
	Poll(ctx context.Context, req *agentpb.PollRequest) (*agentpb.PollResponse, error)
	Report(ctx context.Context, req *agentpb.ReportRequest) (*emptypb.Empty, error)
	SendLogs(ctx context.Context, req *agentpb.SendLogsRequest) (*emptypb.Empty, error)
	ListAgents(ctx context.Context, req *agentpb.ListAgentsRequest) (*agentpb.ListAgentsResponse, error)
	GetAgent(ctx context.Context, id *models.AgentId) (*models.Agent, error)
}

// AgentService is the gRPC handler for cloud.v1.api.agent.AgentService. Pure
// transport: trace the call and delegate to svc.
type AgentService struct {
	agentpb.UnimplementedAgentServiceServer
	*tracing.Entity
	svc AgentActions
}

var _ agentpb.AgentServiceServer = (*AgentService)(nil)

func NewAgentService(logger *xlog.Logger, svc AgentActions) *AgentService {
	return &AgentService{
		Entity: tracing.NewEntity(logger.AppendName("AgentService")),
		svc:    svc,
	}
}

func (s *AgentService) Register(ctx context.Context, req *agentpb.RegisterRequest) (*models.Agent, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "Register",
		func(ctx context.Context, _ trace.Span) (*models.Agent, error) {
			return s.svc.Register(ctx, req)
		})
}

func (s *AgentService) Heartbeat(ctx context.Context, req *agentpb.HeartbeatRequest) (*models.Agent, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "Heartbeat",
		func(ctx context.Context, _ trace.Span) (*models.Agent, error) {
			return s.svc.Heartbeat(ctx, req)
		})
}

func (s *AgentService) Poll(ctx context.Context, req *agentpb.PollRequest) (*agentpb.PollResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "Poll",
		func(ctx context.Context, _ trace.Span) (*agentpb.PollResponse, error) {
			return s.svc.Poll(ctx, req)
		})
}

func (s *AgentService) Report(ctx context.Context, req *agentpb.ReportRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "Report",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			return s.svc.Report(ctx, req)
		})
}

func (s *AgentService) SendLogs(ctx context.Context, req *agentpb.SendLogsRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "SendLogs",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			return s.svc.SendLogs(ctx, req)
		})
}

func (s *AgentService) ListAgents(ctx context.Context, req *agentpb.ListAgentsRequest) (*agentpb.ListAgentsResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListAgents",
		func(ctx context.Context, _ trace.Span) (*agentpb.ListAgentsResponse, error) {
			return s.svc.ListAgents(ctx, req)
		})
}

func (s *AgentService) GetAgent(ctx context.Context, id *models.AgentId) (*models.Agent, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "GetAgent",
		func(ctx context.Context, _ trace.Span) (*models.Agent, error) {
			return s.svc.GetAgent(ctx, id)
		})
}
