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
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AgentLogLine mirrors system.AgentLogLine without an import cycle.
type AgentLogLine struct {
	DagRunID  string
	NodeRunID string
	CommandID string
	Timestamp time.Time
	Stream    string
	Line      string
}

// LogIngester accepts a batch of agent log lines. Implemented by system.Service.
type LogIngester interface {
	IngestLogs(ctx context.Context, lines []AgentLogLine) error
}

// Service handles agent registration, polling and deregistration.
// Bootstrap tokens are JWT-signed; poll tokens are stored in-memory (single-node).
type Service struct {
	repo        *repository.ProtoRepository[agentpb.AgentAlias, agentpb.AgentColumnAlias, *agentpb.AgentScanner, *agentpb.Agent]
	txMgr       pgtx.TxManager
	events      eventing.Bus
	hub         *Hub
	cmdRepo     *CommandsRepo
	bootstrap   *BootstrapTokenStore
	logIngester LogIngester

	mu     sync.Mutex
	tokens map[string]string // poll_token → agent_id
}

// WithLogIngester wires an optional log sink. No-op when ingester is nil.
func (s *Service) WithLogIngester(li LogIngester) *Service { s.logIngester = li; return s }

// dagRunForCommand resolves the parent DagRun id for a command by hitting the
// agent_commands table. Empty string when unknown (best-effort tag).
func (s *Service) dagRunForCommand(ctx context.Context, commandID string) string {
	if s.cmdRepo == nil || commandID == "" {
		return ""
	}
	cmd, err := s.cmdRepo.FindByID(ctx, commandID)
	if err != nil || cmd == nil {
		return ""
	}
	// agent_commands has no dag_run_id column; the NodeRun id maps back via
	// the DAG runtime. Resolution at this layer is best-effort: return the
	// node_run_id as a hint and let the log row carry it instead.
	return ""
}

// nodeRunForCommand resolves the NodeRun id for a command by hitting the
// agent_commands table. Empty string when unknown.
func (s *Service) nodeRunForCommand(ctx context.Context, commandID string) string {
	if s.cmdRepo == nil || commandID == "" {
		return ""
	}
	cmd, err := s.cmdRepo.FindByID(ctx, commandID)
	if err != nil || cmd == nil {
		return ""
	}
	return cmd.GetNodeRunId()
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
// (tenant, dag_run, machine_id, role). The agent presents this token in
// Register; the server treats the JWT claims as the trusted source of
// machine identity.
func (s *Service) IssueBootstrap(ctx context.Context, tenantID, dagRunID, machineID, role string) (string, error) {
	_ = ctx
	tok, err := s.bootstrap.Issue(tenantID, dagRunID, machineID, role)
	if err != nil {
		return "", fmt.Errorf("agent.IssueBootstrap: %w", err)
	}
	return tok, nil
}

// roleFromString maps a string MachineRole name back to the enum. Empty /
// unknown → UNSPECIFIED.
func roleFromString(s string) catalogpb.MachineRole {
	if v, ok := catalogpb.MachineRole_value[s]; ok {
		return catalogpb.MachineRole(v)
	}
	return catalogpb.MachineRole_MACHINE_ROLE_UNSPECIFIED
}

// Register validates the bootstrap token, inserts an agents row, and returns
// the Agent proto plus a long-lived poll_token. The JWT claims (tenant_id,
// dag_run_id, machine_id, role) are the only trusted source — request body
// fields with the same names are ignored.
func (s *Service) Register(ctx context.Context, req *agentpb.RegisterRequest) (*agentpb.RegisterResponse, error) {
	claims, err := s.bootstrap.Verify(req.GetBootstrapToken())
	if err != nil {
		return nil, fmt.Errorf("agent.Register: invalid bootstrap_token: %w", err)
	}

	now := time.Now()
	nowTs := timestamppb.New(now)
	agent := &agentpb.Agent{
		Id:           &agentpb.AgentId{Value: ids.New()},
		TenantId:     &iampb.TenantId{Value: claims.TenantID},
		DagRunId:     claims.DagRunID,
		MachineId:    claims.MachineID,
		Role:         roleFromString(claims.Role),
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

	// Bind this (dag_run, machine_id) to the new agent_id so handlers can
	// resolve targets by machine_id at execute time.
	if s.hub != nil {
		s.hub.Bind(claims.DagRunID, claims.MachineID, agent.GetId().GetValue())
	}

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
		// Forward log lines into the log sink (best-effort; no Poll back-pressure).
		if s.logIngester != nil && len(rep.GetLogLines()) > 0 {
			batch := make([]AgentLogLine, 0, len(rep.GetLogLines()))
			for _, l := range rep.GetLogLines() {
				batch = append(batch, AgentLogLine{
					DagRunID:  s.dagRunForCommand(ctx, l.GetCommandId()),
					NodeRunID: s.nodeRunForCommand(ctx, l.GetCommandId()),
					CommandID: l.GetCommandId(),
					Timestamp: l.GetTs().AsTime(),
					Stream:    l.GetStream().String(),
					Line:      l.GetLine(),
				})
			}
			_ = s.logIngester.IngestLogs(ctx, batch)
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

// Deregister marks the agent as TERMINATED and clears every machine binding
// it held in the Hub, so handlers no longer resolve to a dead agent.
func (s *Service) Deregister(ctx context.Context, id *agentpb.AgentId) error {
	now := time.Now()
	if _, err := s.repo.Execute(ctx,
		agentpb.Agents.Update().
			Set(
				agentpb.Agents.Status.Set(agentpb.AgentStatus_AGENT_STATUS_TERMINATED.String()),
				agentpb.Agents.UpdatedAt.Set(now),
			).
			Where(agentpb.Agents.Id.Eq(id.GetValue())),
	); err != nil {
		return err
	}
	s.hub.ClearByAgent(id.GetValue())
	return nil
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
