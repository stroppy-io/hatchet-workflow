// Package agent implements the agent-facing AgentService (machine principal,
// poll-only lifecycle, D16). Register/Heartbeat/List/Get manage the Agent row;
// Poll/Report/SendLogs are the runtime command-queue surface and are delegated to
// injected seams (the Dag-node leasing engine is not built yet — see
// agent_external.go). Canon: api/agent/agent.proto, features/agent/lifecycle.feature.
package agent

import (
	"context"
	"errors"

	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/gopherex/xlog"
	"github.com/jackc/pgx/v5"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentapi "github.com/stroppy-io/stroppy-cloud/internal/api/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/api/caller"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	rtagent "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/services/authz"
	"github.com/stroppy-io/stroppy-cloud/internal/services/svcutil"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// AgentService implements agent.AgentActions.
type AgentService struct {
	*tracing.Entity
	agents *repository.ProtoRepository[
		models.AgentAlias,
		models.AgentColumnAlias,
		*models.AgentScanner,
		*models.Agent,
	]
	queue CommandQueue
	logs  LogsSink
	authz *authz.Authz
	txm   tx.Trm
}

var _ agentapi.AgentActions = (*AgentService)(nil)

// New builds an AgentService.
func New(logger *xlog.Logger, executor exec.DB, txm tx.Trm, az *authz.Authz, queue CommandQueue, logs LogsSink) *AgentService {
	return &AgentService{
		Entity: tracing.NewEntity(logger.AppendName("AgentService")),
		agents: repository.NewProtoRepository(
			repository.NewScannerRepository(models.Agents.Table, executor),
			models.AgentConverter,
		),
		queue: queue,
		logs:  logs,
		authz: az,
		txm:   txm,
	}
}

// Register is idempotent per (tenant, machine_id): it upserts the Agent and, on a
// new boot_id, invalidates the agent's in-flight leases (fast reboot recovery).
func (s *AgentService) Register(ctx context.Context, req *agentpb.RegisterRequest) (*models.Agent, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "Register",
		func(ctx context.Context, _ trace.Span) (*models.Agent, error) {
			c, err := requireAgentPrincipal(ctx, req.GetTenantId())
			if err != nil {
				return nil, err
			}
			machineID := req.GetTarget().GetMachineId()
			now := timestamppb.Now()

			return tx.DoReadCommittedRet(ctx, s.txm, func(ctx context.Context) (*models.Agent, error) {
				existing, err := s.findByMachine(ctx, req.GetTenantId().GetValue(), machineID)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "lookup agent: %v", err)
				}
				if existing != nil {
					if existing.GetBootId() != req.GetBootId() {
						if err := s.queue.InvalidateLeases(ctx, req.GetTenantId().GetValue(), machineID); err != nil {
							return nil, status.Errorf(codes.Internal, "invalidate leases: %v", err)
						}
					}
					if _, err := s.agents.Execute(ctx, models.Agents.Update().Set(
						models.Agents.BootId.Set(req.GetBootId()),
						models.Agents.Host.Set(req.GetHost()),
						models.Agents.Port.Set(int32(req.GetPort())),
						models.Agents.Version.Set(req.GetVersion()),
						models.Agents.Status.Set(rtagent.AgentStatus_AGENT_STATUS_REGISTERED.String()),
						models.Agents.LastSeenAt.Set(now.AsTime()),
						models.Agents.UpdatedAt.Set(now.AsTime()),
					).Where(models.Agents.Id.Eq(existing.GetId().GetValue()))); err != nil {
						return nil, status.Errorf(codes.Internal, "update agent: %v", err)
					}
					return s.loadByID(ctx, existing.GetId().GetValue())
				}

				agentID := c.AgentID
				if agentID.GetValue() == "" {
					agentID = &models.AgentId{Value: ids.New()}
				}
				ag := &models.Agent{
					Id:               agentID,
					Owned:            &models.Own{TenantId: req.GetTenantId()},
					Timestamps:       &models.Timestamps{CreatedAt: now, UpdatedAt: now},
					MachineId:        machineID,
					AgentComponentId: req.GetTarget().GetAgentComponentId(),
					Status:           rtagent.AgentStatus_AGENT_STATUS_REGISTERED,
					Host:             req.GetHost(),
					Port:             req.GetPort(),
					Version:          req.GetVersion(),
					BootId:           req.GetBootId(),
					RegisteredAt:     now,
					LastSeenAt:       now,
				}
				if _, err := s.agents.Execute(ctx, models.Agents.Insert().From(ag.IntoPlain().AllSetters()...)); err != nil {
					return nil, status.Errorf(codes.Internal, "insert agent: %v", err)
				}
				return ag, nil
			})
		})
}

// Heartbeat updates the agent's status and liveness.
func (s *AgentService) Heartbeat(ctx context.Context, req *agentpb.HeartbeatRequest) (*models.Agent, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "Heartbeat",
		func(ctx context.Context, _ trace.Span) (*models.Agent, error) {
			if _, err := requireAgentPrincipal(ctx, req.GetTenantId()); err != nil {
				return nil, err
			}
			now := timestamppb.Now()
			existing, err := s.findByMachine(ctx, req.GetTenantId().GetValue(), req.GetTarget().GetMachineId())
			if err != nil {
				return nil, status.Errorf(codes.Internal, "lookup agent: %v", err)
			}
			if existing == nil {
				return nil, status.Error(codes.NotFound, "agent not registered")
			}
			if _, err := s.agents.Execute(ctx, models.Agents.Update().Set(
				models.Agents.Status.Set(req.GetStatus().String()),
				models.Agents.LastSeenAt.Set(now.AsTime()),
				models.Agents.UpdatedAt.Set(now.AsTime()),
			).Where(models.Agents.Id.Eq(existing.GetId().GetValue()))); err != nil {
				return nil, status.Errorf(codes.Internal, "heartbeat: %v", err)
			}
			return s.loadByID(ctx, existing.GetId().GetValue())
		})
}

// ListAgents returns the tenant's agents (human read).
//
// TODO(agent): models.CommonQuery filter/pagination ignored. Reported.
func (s *AgentService) ListAgents(ctx context.Context, req *agentpb.ListAgentsRequest) (*models.Agent_List, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListAgents",
		func(ctx context.Context, _ trace.Span) (*models.Agent_List, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			agents, err := s.agents.Query(ctx, models.Agents.SelectAll().Where(
				models.Agents.TenantId.Eq(req.GetTenantId().GetValue()),
				models.Agents.DeletedAt.IsNull(),
			))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "list agents: %v", err)
			}
			return &models.Agent_List{Agents: agents}, nil
		})
}

// GetAgent returns a single agent by id (human read; authorized against the
// agent's own tenant since the request carries no tenant_id).
func (s *AgentService) GetAgent(ctx context.Context, id *models.AgentId) (*models.Agent, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "GetAgent",
		func(ctx context.Context, _ trace.Span) (*models.Agent, error) {
			ag, err := s.loadByID(ctx, id.GetValue())
			if err != nil {
				return nil, err
			}
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), ag.GetOwned().GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			return ag, nil
		})
}

func (s *AgentService) findByMachine(ctx context.Context, tenantID, machineID string) (*models.Agent, error) {
	ag, err := s.agents.QueryRow(ctx, models.Agents.SelectAll().Where(
		models.Agents.TenantId.Eq(tenantID),
		models.Agents.MachineId.Eq(machineID),
		models.Agents.DeletedAt.IsNull(),
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return ag, nil
}

func (s *AgentService) loadByID(ctx context.Context, id string) (*models.Agent, error) {
	ag, err := s.agents.QueryRow(ctx, models.Agents.SelectAll().Where(
		models.Agents.Id.Eq(id),
		models.Agents.DeletedAt.IsNull(),
	))
	if err != nil {
		return nil, svcutil.NotFound(err, "agent")
	}
	return ag, nil
}

// requireAgentPrincipal enforces that the caller is the machine principal acting
// in the given tenant (worker RPCs).
func requireAgentPrincipal(ctx context.Context, tenantID *models.TenantId) (*caller.Caller, error) {
	c, ok := caller.FromContext(ctx)
	if !ok || c.Kind != caller.PrincipalAgent {
		return nil, status.Error(codes.Unauthenticated, "agent principal required")
	}
	if c.TenantID.GetValue() != tenantID.GetValue() {
		return nil, status.Error(codes.PermissionDenied, "agent not scoped to this tenant")
	}
	return c, nil
}
