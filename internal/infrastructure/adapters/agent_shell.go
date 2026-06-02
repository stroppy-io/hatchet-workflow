package adapters

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/services/agent_shell"
)

// sessionRecvBuffer bounds the per-session inbound frame queue so a slow admin
// websocket cannot make the agent control stream grow without limit.
const sessionRecvBuffer = 256

/*
InMemoryShellHub is the in-process reverse-shell broker. It plays two roles:

  - It is the AGENT-FACING control-stream server (agent.AgentShellAgentServiceServer):
    each agent dials Connect, sends a Register{machine_id} frame, and the hub
    keeps that bidi stream as the machine's live control channel.
  - It is the ADMIN-FACING agent_shell.ShellHub: Open assigns a fresh session_id,
    asks the located agent to spawn a PTY (ServerShellMsg_Open), and hands back a
    ShellSession the OpenShell handler bridges to the admin websocket.

Agent->server frames are demultiplexed by session_id and pushed onto the matching
session's inbound channel. Everything is concurrency-safe via a single mutex
guarding the connection + session maps; per-stream sends are serialized by a
per-connection send mutex.
*/
type InMemoryShellHub struct {
	agent.UnimplementedAgentShellAgentServiceServer

	mu       sync.Mutex
	conns    map[string]*agentConn    // machineID -> live control stream
	sessions map[string]*shellSession // sessionID -> session
	log      *slog.Logger
}

var (
	_ agent_shell.ShellHub               = (*InMemoryShellHub)(nil)
	_ agent.AgentShellAgentServiceServer = (*InMemoryShellHub)(nil)
)

// NewInMemoryShellHub builds the hub. A nil logger falls back to slog.Default().
func NewInMemoryShellHub(log *slog.Logger) *InMemoryShellHub {
	if log == nil {
		log = slog.Default()
	}
	return &InMemoryShellHub{
		conns:    make(map[string]*agentConn),
		sessions: make(map[string]*shellSession),
		log:      log,
	}
}

// agentConn is one live agent control stream plus a send mutex serializing writes.
type agentConn struct {
	machineID string
	stream    grpc.BidiStreamingServer[agent.AgentShellMsg, agent.ServerShellMsg]
	sendMu    sync.Mutex
}

// send serializes a server->agent frame on the connection.
func (c *agentConn) send(msg *agent.ServerShellMsg) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	return c.stream.Send(msg)
}

// Connect is the agent control stream. The first frame MUST be a Register; the
// hub then keeps the stream as the machine's control channel until it ends,
// routing per-session agent frames to their sessions.
func (h *InMemoryShellHub) Connect(stream grpc.BidiStreamingServer[agent.AgentShellMsg, agent.ServerShellMsg]) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	reg := first.GetRegister()
	if reg == nil || reg.GetMachineId() == "" {
		return derrors.Invalid("register", "first agent frame must be Register with machine_id")
	}
	machineID := reg.GetMachineId()
	conn := &agentConn{machineID: machineID, stream: stream}

	h.mu.Lock()
	if existing, ok := h.conns[machineID]; ok {
		// A newer registration supersedes the old stream for this machine.
		h.log.Warn("agent_shell: replacing existing control stream", slog.String("machine_id", machineID))
		_ = existing
	}
	h.conns[machineID] = conn
	h.mu.Unlock()

	defer h.dropConn(machineID, conn)

	for {
		msg, rerr := stream.Recv()
		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				return nil
			}
			return rerr
		}
		h.route(msg)
	}
}

// route delivers an agent->server frame to its session by session_id.
func (h *InMemoryShellHub) route(msg *agent.AgentShellMsg) {
	if msg.GetSessionId() == "" {
		return // register / unkeyed frames carry no session
	}
	h.mu.Lock()
	sess := h.sessions[msg.GetSessionId()]
	h.mu.Unlock()
	if sess == nil {
		return
	}
	sess.deliver(msg)
}

// dropConn removes a control stream (only if it is still the registered one) and
// tears down every session bound to that machine.
func (h *InMemoryShellHub) dropConn(machineID string, conn *agentConn) {
	h.mu.Lock()
	if h.conns[machineID] == conn {
		delete(h.conns, machineID)
	}
	var orphaned []*shellSession
	for id, s := range h.sessions {
		if s.machineID == machineID {
			orphaned = append(orphaned, s)
			delete(h.sessions, id)
		}
	}
	h.mu.Unlock()
	for _, s := range orphaned {
		s.terminate()
	}
}

// Open asks the located agent to spawn a fresh PTY and returns a session handle.
// derrors.FailedPrecondition when no live control stream exists for the machine.
func (h *InMemoryShellHub) Open(_ context.Context, machineID string, spec *agent.OpenShell) (agent_shell.ShellSession, error) {
	h.mu.Lock()
	conn := h.conns[machineID]
	h.mu.Unlock()
	if conn == nil {
		return nil, derrors.FailedPrecondition("agent_offline", "no live agent control stream for machine")
	}

	sessionID := uuid.NewString()
	sess := &shellSession{
		id:        sessionID,
		machineID: machineID,
		conn:      conn,
		hub:       h,
		recv:      make(chan *agent.AgentShellMsg, sessionRecvBuffer),
		done:      make(chan struct{}),
	}

	h.mu.Lock()
	h.sessions[sessionID] = sess
	h.mu.Unlock()

	if err := conn.send(&agent.ServerShellMsg{
		SessionId: sessionID,
		Msg:       &agent.ServerShellMsg_Open{Open: spec},
	}); err != nil {
		h.removeSession(sessionID)
		return nil, derrors.Internal("failed to open shell on agent").Wrap(err)
	}
	h.log.Info("agent_shell: session opened",
		slog.String("session_id", sessionID), slog.String("machine_id", machineID))
	return sess, nil
}

// removeSession unregisters a session by id.
func (h *InMemoryShellHub) removeSession(sessionID string) {
	h.mu.Lock()
	delete(h.sessions, sessionID)
	h.mu.Unlock()
}

// shellSession is one live PTY session bridged over an agent control stream.
type shellSession struct {
	id        string
	machineID string
	conn      *agentConn
	hub       *InMemoryShellHub

	recv      chan *agent.AgentShellMsg
	done      chan struct{}
	closeOnce sync.Once
}

var _ agent_shell.ShellSession = (*shellSession)(nil)

// SessionID returns the hub-assigned session id.
func (s *shellSession) SessionID() string { return s.id }

// Stdin forwards keystroke bytes to the agent PTY.
func (s *shellSession) Stdin(ctx context.Context, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.conn.send(&agent.ServerShellMsg{
		SessionId: s.id,
		Msg:       &agent.ServerShellMsg_Stdin{Stdin: data},
	})
}

// Resize updates the agent PTY window size.
func (s *shellSession) Resize(ctx context.Context, cols, rows uint32) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.conn.send(&agent.ServerShellMsg{
		SessionId: s.id,
		Msg:       &agent.ServerShellMsg_Resize{Resize: &agent.ShellResize{Cols: cols, Rows: rows}},
	})
}

// Recv blocks until the next agent frame for this session, returning io.EOF when
// the session ends (close, exit, or torn-down control stream) or the context is
// cancelled.
func (s *shellSession) Recv(ctx context.Context) (*agent.AgentShellMsg, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.done:
		// Drain any buffered frame before reporting EOF so the final Exit is seen.
		select {
		case msg := <-s.recv:
			return msg, nil
		default:
			return nil, io.EOF
		}
	case msg := <-s.recv:
		return msg, nil
	}
}

// deliver pushes an agent frame onto the session's inbound queue (best-effort: a
// full buffer drops the frame rather than blocking the shared control stream).
func (s *shellSession) deliver(msg *agent.AgentShellMsg) {
	select {
	case <-s.done:
	case s.recv <- msg:
	default:
		s.hub.log.Warn("agent_shell: dropping frame, session buffer full",
			slog.String("session_id", s.id))
	}
}

// Close idempotently releases the session on the agent side and stops Recv.
func (s *shellSession) Close() error {
	s.closeOnce.Do(func() {
		close(s.done)
		s.hub.removeSession(s.id)
		_ = s.conn.send(&agent.ServerShellMsg{
			SessionId: s.id,
			Msg:       &agent.ServerShellMsg_Close{Close: &emptypb.Empty{}},
		})
	})
	return nil
}

// terminate stops Recv without sending a Close to the agent (used when the agent
// stream itself went away).
func (s *shellSession) terminate() {
	s.closeOnce.Do(func() { close(s.done) })
}

// MachineAddressResolver is the consumer interface the MachineLocator verifies a
// target machine through. The gormstore machine registry satisfies it: Lookup
// reports whether (tenantID, machineID) is a registered machine. The hub itself
// answers liveness (live control stream), so the locator combines both checks.
type MachineAddressResolver interface {
	// Lookup returns derrors.ErrNotFound when the machine is unknown or not
	// registered under tenantID.
	Lookup(ctx context.Context, tenantID, machineID, componentID string) error
}

// MachineLocator implements agent_shell.MachineLocator: it confirms the target
// machine exists under the tenant (via the resolver) and currently has a live
// agent control stream the hub can reach.
type MachineLocator struct {
	resolver MachineAddressResolver
	hub      *InMemoryShellHub
}

var _ agent_shell.MachineLocator = (*MachineLocator)(nil)

// NewMachineLocator builds the locator over the registry resolver and the hub.
func NewMachineLocator(resolver MachineAddressResolver, hub *InMemoryShellHub) *MachineLocator {
	return &MachineLocator{resolver: resolver, hub: hub}
}

// Locate verifies registration then liveness. derrors.ErrNotFound for an unknown
// machine; derrors.FailedPrecondition when the agent is offline.
func (l *MachineLocator) Locate(ctx context.Context, tenantID, machineID, componentID string) error {
	if l.resolver != nil {
		if err := l.resolver.Lookup(ctx, tenantID, machineID, componentID); err != nil {
			return err
		}
	}
	l.hub.mu.Lock()
	live := l.hub.conns[machineID] != nil
	l.hub.mu.Unlock()
	if !live {
		return derrors.FailedPrecondition("agent_offline", "agent for machine is not connected")
	}
	return nil
}

// TenantMembershipChecker is the consumer interface the TenantGuard checks tenant
// ownership through. The gormstore/iam tenant membership repo satisfies it:
// IsMember reports whether the account is a member of the tenant (or a platform
// admin). Returns derrors.ErrNotFound when the tenant does not exist.
type TenantMembershipChecker interface {
	IsMember(ctx context.Context, accountID, tenantID string) (bool, error)
}

// TenantGuard implements agent_shell.TenantGuard: it fences the tenant scope of a
// shell session (RBAC for the permission itself is enforced upstream).
type TenantGuard struct{ members TenantMembershipChecker }

var _ agent_shell.TenantGuard = (*TenantGuard)(nil)

// NewTenantGuard builds the guard over the membership checker.
func NewTenantGuard(members TenantMembershipChecker) *TenantGuard {
	return &TenantGuard{members: members}
}

// EnsureMember returns nil when the account may operate in the tenant,
// derrors.ErrNotFound for an unknown tenant, and a permission-denied error
// otherwise.
func (g *TenantGuard) EnsureMember(ctx context.Context, accountID, tenantID string) error {
	ok, err := g.members.IsMember(ctx, accountID, tenantID)
	if err != nil {
		return err
	}
	if !ok {
		return derrors.PermissionDenied("not a member of tenant")
	}
	return nil
}

// SlogShellAudit implements agent_shell.ShellAudit by emitting one immutable
// structured log record per opened session and one per session end. It owns no
// storage; the audit trail is the structured log stream.
type SlogShellAudit struct{ log *slog.Logger }

var _ agent_shell.ShellAudit = (*SlogShellAudit)(nil)

// NewSlogShellAudit builds the slog-backed audit. A nil logger uses slog.Default().
func NewSlogShellAudit(log *slog.Logger) *SlogShellAudit {
	if log == nil {
		log = slog.Default()
	}
	return &SlogShellAudit{log: log.With(slog.String("audit", "agent_shell"))}
}

// Record logs the opened-session audit entry.
func (a *SlogShellAudit) Record(_ context.Context, entry *agent_shell.ShellAuditEntry) error {
	a.log.Info("shell session opened",
		slog.String("id", entry.ID),
		slog.String("session_id", entry.SessionID),
		slog.String("account_id", entry.AccountID),
		slog.String("tenant_id", entry.TenantID),
		slog.String("run_id", entry.RunID),
		slog.String("machine_id", entry.MachineID),
		slog.String("component_id", entry.ComponentID),
		slog.String("shell", entry.Shell),
		slog.Time("opened_at", entry.OpenedAt),
	)
	return nil
}

// End logs the session-termination audit entry.
func (a *SlogShellAudit) End(_ context.Context, sessionID string, exitCode int32, exitErr string, at time.Time) error {
	a.log.Info("shell session ended",
		slog.String("session_id", sessionID),
		slog.Int("exit_code", int(exitCode)),
		slog.String("exit_error", exitErr),
		slog.Time("ended_at", at),
	)
	return nil
}
