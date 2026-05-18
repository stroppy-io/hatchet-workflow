package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"

	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// bootEntry holds metadata encoded in a bootstrap token.
type bootEntry struct {
	tenantID string
	dagRunID string
}

// Service handles agent registration, polling and deregistration.
// Bootstrap tokens are JWT-signed; poll tokens are stored in-memory (single-node).
type Service struct {
	repo      *repository.ProtoRepository[agentpb.AgentAlias, agentpb.AgentColumnAlias, *agentpb.AgentScanner, *agentpb.Agent]
	txMgr     pgtx.TxManager
	events    eventing.Bus
	hub       *Hub
	cmdRepo   *CommandsRepo
	bootstrap *BootstrapTokenStore

	mu     sync.Mutex
	tokens map[string]string // poll_token → agent_id
}

func New(executor exec.DB, txMgr pgtx.TxManager, events eventing.Bus, hub *Hub, cmdRepo *CommandsRepo, bootstrap *BootstrapTokenStore) *Service {
	return &Service{
		repo: repository.NewProtoRepository(
			repository.NewScannerRepository(agentpb.Agents.Table, executor),
			agentpb.AgentConverter,
		),
		txMgr:     txMgr,
		events:    events,
		hub:       hub,
		cmdRepo:   cmdRepo,
		bootstrap: bootstrap,
		tokens:    map[string]string{},
	}
}

// IssueBootstrap returns a signed JWT bootstrap token bound to the given
// tenant and dag-run. The agent presents this token in Register.
func (s *Service) IssueBootstrap(tenantID, dagRunID string) string {
	tok, err := s.bootstrap.Issue(tenantID, dagRunID, "", "")
	if err != nil {
		return ""
	}
	return tok
}

// Register validates the bootstrap token, inserts an agents row, and returns
// the Agent proto plus a long-lived poll_token.
func (s *Service) Register(ctx context.Context, req *agentpb.RegisterRequest) (*agentpb.RegisterResponse, error) {
	claims, err := s.bootstrap.Verify(req.GetBootstrapToken())
	if err != nil {
		return nil, fmt.Errorf("agent.Register: invalid bootstrap_token: %w", err)
	}
	entry := bootEntry{tenantID: claims.TenantID, dagRunID: claims.DagRunID}

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
			if s.cmdRepo != nil {
				reportBytes, _ := json.Marshal(r)
				_ = s.cmdRepo.MarkReported(ctx, r.GetCommandId(), reportBytes)
			}
		}
	}

	cmds := s.hub.Drain(agentID)
	if s.cmdRepo != nil {
		for _, cmd := range cmds {
			// Insert PENDING row (using hub cmd.Id as the DB row id for correlation),
			// then immediately transition to DELIVERED.
			payload, _ := json.Marshal(cmd.GetAction())
			if _, err := s.cmdRepo.Insert(ctx, cmd.GetId(), agentID, cmd.GetRunId(), payload); err == nil {
				_ = s.cmdRepo.MarkDelivered(ctx, cmd.GetId())
			}
		}
	}
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

// MarkStaleAgents flips any HEALTHY agent whose last_heartbeat is older than
// threshold to STALE. Called once at startup by the recovery worker.
// Returns the number of rows updated.
func (s *Service) MarkStaleAgents(ctx context.Context, threshold time.Duration) (int64, error) {
	cutoff := time.Now().Add(-threshold)
	ct, err := s.repo.Execute(ctx,
		agentpb.Agents.Update().
			Set(
				agentpb.Agents.Status.Set(agentpb.AgentStatus_AGENT_STATUS_STALE.String()),
				agentpb.Agents.UpdatedAt.Set(time.Now()),
			).
			Where(
				agentpb.Agents.Status.Eq(agentpb.AgentStatus_AGENT_STATUS_HEALTHY.String()),
				agentpb.Agents.LastHeartbeat.Lt(&cutoff),
			),
	)
	return ct, err
}
