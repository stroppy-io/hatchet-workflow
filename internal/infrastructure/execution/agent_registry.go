package execution

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent/agentconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	domainpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
)

const agentHeartbeatInterval = 15 * time.Second

type agentClaimsKey struct{}

type AgentRegistryService struct {
	mu                sync.RWMutex
	records           map[string]agentPresenceRecord
	heartbeatInterval time.Duration
	now               func() time.Time
	log               *slog.Logger
}

type agentPresenceRecord struct {
	machineID         string
	runID             string
	host              string
	agentVersion      string
	registeredAt      time.Time
	lastSeenAt        time.Time
	heartbeatInterval time.Duration
}

var _ agentconnect.AgentRegistryServiceHandler = (*AgentRegistryService)(nil)
var _ AgentPresenceReader = (*AgentRegistryService)(nil)

func NewAgentRegistryService(log *slog.Logger) *AgentRegistryService {
	if log == nil {
		log = slog.Default()
	}
	return &AgentRegistryService{
		records:           make(map[string]agentPresenceRecord),
		heartbeatInterval: agentHeartbeatInterval,
		now:               time.Now,
		log:               log,
	}
}

func NewAgentAuthInterceptor(verifier AgentTokenVerifier) connect.Interceptor {
	return agentAuthInterceptor{verifier: verifier}
}

func (s *AgentRegistryService) Register(ctx context.Context, req *agentpb.RegisterRequest) (*agentpb.RegisterResponse, error) {
	claims, err := agentClaimsFromContext(ctx)
	if err != nil {
		return nil, err
	}
	info := req.GetInfo()
	if info == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("agent info is required"))
	}
	if machineID := strings.TrimSpace(info.GetMachineId()); machineID == "" || machineID != claims.MachineID {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("agent machine_id %q does not match token machine_id %q", machineID, claims.MachineID))
	}
	if runID := strings.TrimSpace(info.GetRunId()); runID != "" && runID != claims.RunID {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("agent run_id %q does not match token run_id %q", runID, claims.RunID))
	}

	now := s.now().UTC()
	rec := agentPresenceRecord{
		machineID:         claims.MachineID,
		runID:             claims.RunID,
		host:              strings.TrimSpace(info.GetHost()),
		agentVersion:      strings.TrimSpace(info.GetAgentVersion()),
		registeredAt:      now,
		lastSeenAt:        now,
		heartbeatInterval: s.heartbeatInterval,
	}
	s.mu.Lock()
	s.records[claims.MachineID] = rec
	s.mu.Unlock()

	s.log.DebugContext(ctx, "agent registered",
		slog.String("run_id", claims.RunID),
		slog.String("machine_id", claims.MachineID),
		slog.String("host", rec.host),
		slog.String("agent_version", rec.agentVersion))

	return &agentpb.RegisterResponse{
		RegisteredAt:             timestamppb.New(now),
		HeartbeatIntervalSeconds: uint32(s.heartbeatInterval / time.Second),
	}, nil
}

func (s *AgentRegistryService) Heartbeat(ctx context.Context, req *agentpb.HeartbeatRequest) (*agentpb.HeartbeatResponse, error) {
	claims, err := agentClaimsFromContext(ctx)
	if err != nil {
		return nil, err
	}
	machineID := strings.TrimSpace(req.GetMachineId())
	if machineID == "" || machineID != claims.MachineID {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("agent machine_id %q does not match token machine_id %q", machineID, claims.MachineID))
	}

	now := s.now().UTC()
	s.mu.Lock()
	rec := s.records[claims.MachineID]
	if rec.machineID == "" || rec.runID != claims.RunID {
		rec = agentPresenceRecord{
			machineID:         claims.MachineID,
			runID:             claims.RunID,
			registeredAt:      now,
			heartbeatInterval: s.heartbeatInterval,
		}
	}
	rec.lastSeenAt = now
	if rec.heartbeatInterval <= 0 {
		rec.heartbeatInterval = s.heartbeatInterval
	}
	s.records[claims.MachineID] = rec
	s.mu.Unlock()

	return &agentpb.HeartbeatResponse{}, nil
}

func (s *AgentRegistryService) AgentPresence(ctx context.Context, runID string, machineIDs []string) (map[string]*monitor.WorkerInfo, error) {
	if s == nil {
		return nil, nil
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	now := s.now().UTC()
	records := make(map[string]agentPresenceRecord, len(machineIDs))
	s.mu.RLock()
	for _, machineID := range machineIDs {
		machineID = strings.TrimSpace(machineID)
		if machineID == "" {
			continue
		}
		rec, ok := s.records[machineID]
		if !ok || rec.runID != runID {
			continue
		}
		records[machineID] = rec
	}
	s.mu.RUnlock()

	out := make(map[string]*monitor.WorkerInfo, len(records))
	for machineID, rec := range records {
		out[machineID] = workerInfoFromPresence(rec, now)
	}
	return out, nil
}

func workerInfoFromPresence(rec agentPresenceRecord, now time.Time) *monitor.WorkerInfo {
	presence, reason := classifyAgentPresence(rec, now)
	return &monitor.WorkerInfo{
		Id:                       "agent/" + rec.machineID,
		Kind:                     domainpb.Worker_KIND_AGENT,
		MachineId:                rec.machineID,
		Host:                     rec.host,
		Online:                   presence == monitor.WorkerPresence_WORKER_PRESENCE_ONLINE,
		Status:                   common.Status_STATUS_UNSPECIFIED,
		Presence:                 presence,
		RegisteredAt:             timestamppb.New(rec.registeredAt),
		LastSeenAt:               timestamppb.New(rec.lastSeenAt),
		HeartbeatIntervalSeconds: uint32(rec.heartbeatInterval / time.Second),
		AgentVersion:             rec.agentVersion,
		RunId:                    rec.runID,
		StatusReason:             reason,
		Source:                   monitor.ObservationSource_OBSERVATION_SOURCE_AGENT_REGISTRY,
	}
}

func classifyAgentPresence(rec agentPresenceRecord, now time.Time) (monitor.WorkerPresence, string) {
	if rec.lastSeenAt.IsZero() {
		return monitor.WorkerPresence_WORKER_PRESENCE_UNKNOWN, "agent_registered_without_heartbeat"
	}
	interval := rec.heartbeatInterval
	if interval <= 0 {
		interval = agentHeartbeatInterval
	}
	age := now.Sub(rec.lastSeenAt)
	switch {
	case age <= 2*interval:
		return monitor.WorkerPresence_WORKER_PRESENCE_ONLINE, "heartbeat_recent"
	case age <= 6*interval:
		return monitor.WorkerPresence_WORKER_PRESENCE_STALE, fmt.Sprintf("heartbeat_stale_%s", age.Round(time.Second))
	default:
		return monitor.WorkerPresence_WORKER_PRESENCE_OFFLINE, fmt.Sprintf("heartbeat_missed_%s", age.Round(time.Second))
	}
}

type agentAuthInterceptor struct {
	verifier AgentTokenVerifier
}

func (i agentAuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		claims, err := verifyAgentAuthorization(i.verifier, req.Header().Get("Authorization"))
		if err != nil {
			return nil, err
		}
		return next(context.WithValue(ctx, agentClaimsKey{}, claims), req)
	}
}

func (i agentAuthInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i agentAuthInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		claims, err := verifyAgentAuthorization(i.verifier, conn.RequestHeader().Get("Authorization"))
		if err != nil {
			return err
		}
		return next(context.WithValue(ctx, agentClaimsKey{}, claims), conn)
	}
}

func verifyAgentAuthorization(verifier AgentTokenVerifier, header string) (*agentdomain.TokenClaims, error) {
	if verifier == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("agent token verifier is not configured"))
	}
	token := agentdomain.BearerToken(header)
	if token == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing agent token"))
	}
	claims, err := verifier.VerifyAgentToken(token)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid agent token"))
	}
	return claims, nil
}

func agentClaimsFromContext(ctx context.Context) (*agentdomain.TokenClaims, error) {
	claims, ok := ctx.Value(agentClaimsKey{}).(*agentdomain.TokenClaims)
	if !ok || claims == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing agent token claims"))
	}
	return claims, nil
}
