package connect

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	agentsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/agent"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
)

// AgentHandler implements AgentServiceHandler for the Connect mux.
type AgentHandler struct{ svc *agentsvc.Service }

func NewAgentHandler(svc *agentsvc.Service) *AgentHandler { return &AgentHandler{svc: svc} }

func (h *AgentHandler) Register(ctx context.Context, req *connect.Request[agentpb.RegisterRequest]) (*connect.Response[agentpb.RegisterResponse], error) {
	out, err := h.svc.Register(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(out), nil
}

func (h *AgentHandler) Poll(ctx context.Context, req *connect.Request[agentpb.PollRequest]) (*connect.Response[agentpb.CommandBatch], error) {
	out, err := h.svc.Poll(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(out), nil
}

func (h *AgentHandler) Deregister(ctx context.Context, req *connect.Request[agentpb.AgentId]) (*connect.Response[emptypb.Empty], error) {
	if err := h.svc.Deregister(ctx, req.Msg); err != nil {
		return nil, err
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}
