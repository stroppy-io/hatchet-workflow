package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"

	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// bootEntry holds metadata encoded in a bootstrap token.
type bootEntry struct {
	tenantID string
	dagRunID string
}

// Service handles agent registration, polling and deregistration.
// Bootstrap tokens and poll tokens are stored in-memory (single-node).
type Service struct {
	repo   *repository.ProtoRepository[agentpb.AgentAlias, agentpb.AgentColumnAlias, *agentpb.AgentScanner, *agentpb.Agent]
	txMgr  pgtx.TxManager
	events eventing.Bus
	hub    *Hub

	mu     sync.Mutex
	tokens map[string]string    // poll_token → agent_id
	boots  map[string]bootEntry // bootstrap_token → metadata
}

func New(executor exec.DB, txMgr pgtx.TxManager, events eventing.Bus, hub *Hub) *Service {
	return &Service{
		repo: repository.NewProtoRepository(
			repository.NewScannerRepository(agentpb.Agents.Table, executor),
			agentpb.AgentConverter,
		),
		txMgr:  txMgr,
		events: events,
		hub:    hub,
		tokens: map[string]string{},
		boots:  map[string]bootEntry{},
	}
}

// IssueBootstrap returns a one-shot bootstrap token bound to the given
// tenant and dag-run. The agent presents this token in Register.
func (s *Service) IssueBootstrap(tenantID, dagRunID string) string {
	t := ids.New()
	s.mu.Lock()
	s.boots[t] = bootEntry{tenantID: tenantID, dagRunID: dagRunID}
	s.mu.Unlock()
	return t
}

// Register validates the bootstrap token, inserts an agents row, and returns
// the Agent proto plus a long-lived poll_token.
func (s *Service) Register(ctx context.Context, req *agentpb.RegisterRequest) (*agentpb.RegisterResponse, error) {
	s.mu.Lock()
	entry, ok := s.boots[req.GetBootstrapToken()]
	if ok {
		delete(s.boots, req.GetBootstrapToken())
	}
	s.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("agent.Register: invalid or already-used bootstrap_token")
	}

	now := time.Now()
	nowTs := timestamppb.New(now)
	agent := &agentpb.Agent{
		Id:           &agentpb.AgentId{Value: ids.New()},
		TenantId:     &iampb.TenantId{Value: entry.tenantID},
		DagRunId:     entry.dagRunID,
		MachineId:    req.GetMachineId(),
		Role:         req.GetRole(),
		InternalIp:   req.GetInternalIp(),
		PublicIp:     req.PublicIp,
		AgentVersion: req.GetAgentVersion(),
		Capabilities: req.GetCapabilities(),
		Status:       agentpb.AgentStatus_AGENT_STATUS_REGISTERED,
		Timestamps: &commonpb.Timestamps{
			CreatedAt: nowTs,
			UpdatedAt: nowTs,
		},
	}

	scanner := agent.IntoPlain()
	if _, err := s.repo.Execute(ctx, agentpb.Agents.Insert().From(scanner.AllSetters()...)); err != nil {
		return nil, err
	}

	token := ids.New()
	s.mu.Lock()
	s.tokens[token] = agent.GetId().GetValue()
	s.mu.Unlock()

	return &agentpb.RegisterResponse{Agent: agent, PollToken: token}, nil
}

// Poll updates the heartbeat, resolves reports into hub waiters, and returns
// pending commands for this agent.
func (s *Service) Poll(ctx context.Context, req *agentpb.PollRequest) (*agentpb.CommandBatch, error) {
	agentID := req.GetAgentId().GetValue()

	now := time.Now()
	_, _ = s.repo.Execute(ctx,
		agentpb.Agents.Update().
			Set(
				agentpb.Agents.LastHeartbeat.Set(&now),
				agentpb.Agents.UpdatedAt.Set(now),
				agentpb.Agents.Status.Set(agentpb.AgentStatus_AGENT_STATUS_HEALTHY.String()),
			).
			Where(agentpb.Agents.Id.Eq(agentID)),
	)

	if rep := req.GetReport(); rep != nil {
		for _, r := range rep.GetReports() {
			s.hub.Resolve(r.GetCommandId(), r)
		}
	}

	cmds := s.hub.Drain(agentID)
	return &agentpb.CommandBatch{Commands: cmds}, nil
}

// Deregister marks the agent as TERMINATED.
func (s *Service) Deregister(ctx context.Context, id *agentpb.AgentId) error {
	now := time.Now()
	_, err := s.repo.Execute(ctx,
		agentpb.Agents.Update().
			Set(
				agentpb.Agents.Status.Set(agentpb.AgentStatus_AGENT_STATUS_TERMINATED.String()),
				agentpb.Agents.UpdatedAt.Set(now),
			).
			Where(agentpb.Agents.Id.Eq(id.GetValue())),
	)
	return err
}

// AgentIDForToken resolves a poll_token to an agent_id.
// Returns empty string if the token is unknown.
func (s *Service) AgentIDForToken(token string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tokens[token]
}
