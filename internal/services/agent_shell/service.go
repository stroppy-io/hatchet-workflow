package agent_shell

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/gopherex/pgtx/pkg/tx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

/*
	agent_shell wires the ADMIN/USER-FACING half of the interactive reverse shell
	(api/agent_shell.proto) to the AGENT-FACING control stream (agent/shell.proto).

	One OpenShell call == one terminal session. The handler:
	  - reads the REQUIRED first client frame (ShellStart) which carries the target
	    + tenant the streaming auth interceptor already scoped on,
	  - resolves the caller identity for ownership/audit and re-validates that the
	    declared tenant is the one the caller may act in,
	  - opens a single session on the target host via the ShellHub (which owns the
	    agent control stream and session multiplexing),
	  - writes one immutable audit record (who/host/when),
	  - then bridges bytes both ways until either side closes, the shell exits, or
	    the stream errors.

	The service owns no transport or process state of its own: the agent control
	stream, session keying and PTY lifecycle all live behind ShellHub.

	===== Dependency interfaces (constructor-injected) =====
*/

// TenantGuard verifies that the caller may operate inside the named tenant. It
// returns derrors.ErrNotFound when the tenant does not exist and a
// permission-denied domain error when the caller is neither a platform admin nor
// a member of it. (RBAC for the RESOURCE_AGENT_SHELL permission itself is already
// enforced by the streaming interceptor; this only fences the tenant scope.)
type TenantGuard interface {
	EnsureMember(ctx context.Context, accountID, tenantID string) error
}

// MachineLocator verifies the target machine exists, is registered under the
// given tenant, and currently has a live agent control stream the hub can reach.
// It returns derrors.ErrNotFound for an unknown machine and a
// failed-precondition domain error when the agent is offline.
type MachineLocator interface {
	Locate(ctx context.Context, tenantID, machineID, componentID string) error
}

// ShellSession is one live PTY session bridged over the agent control stream. It
// is the hub's per-session handle: the handler pumps client frames in (Stdin,
// Resize, Close) and agent frames out (Stdout, Stderr, Exit). Recv blocks until
// the next agent frame and returns io.EOF when the session ends cleanly. Close is
// idempotent and releases the session on the agent side.
type ShellSession interface {
	SessionID() string
	Stdin(ctx context.Context, data []byte) error
	Resize(ctx context.Context, cols, rows uint32) error
	Recv(ctx context.Context) (*agent.AgentShellMsg, error)
	Close() error
}

// ShellHub owns the long-lived agent control streams (agent/shell.proto Connect)
// and multiplexes per-session frames over them keyed by session_id. Open asks the
// located agent to spawn a fresh PTY and returns a session handle the handler
// bridges to the admin websocket. The session_id is hub-assigned.
type ShellHub interface {
	Open(ctx context.Context, machineID string, spec *agent.OpenShell) (ShellSession, error)
}

// ShellAudit records one immutable audit row per opened session (who opened a
// shell on which host, in which tenant/run, when). Recording happens inside the
// ambient ctx transaction so it commits atomically. End stamps the session's
// termination (exit code / error) for the audit trail; a missing record is
// ignored (derrors.ErrNotFound).
type ShellAudit interface {
	Record(ctx context.Context, entry *ShellAuditEntry) error
	End(ctx context.Context, sessionID string, exitCode int32, exitErr string, at time.Time) error
}

// ShellAuditEntry is the immutable record of a session being opened.
type ShellAuditEntry struct {
	ID          string
	SessionID   string
	AccountID   string
	TenantID    string
	RunID       string
	MachineID   string
	ComponentID string
	Shell       string
	OpenedAt    time.Time
}

// AgentShellDeps bundles every dependency for the constructor.
type AgentShellDeps struct {
	Authn    utils.Authn
	Tenants  TenantGuard
	Machines MachineLocator
	Hub      ShellHub
	Audit    ShellAudit
	Tx       tx.Trm
}

type AgentShellService struct {
	*api.UnimplementedAgentShellServiceServer
	tx.Trm
	d AgentShellDeps
}

var _ api.AgentShellServiceServer = (*AgentShellService)(nil)

func NewAgentShellService(deps AgentShellDeps) *AgentShellService {
	return &AgentShellService{d: deps}
}

/*
	===== helpers =====
*/

func (s *AgentShellService) caller(ctx context.Context) (*iam.AccessClaims, error) {
	c, err := s.d.Authn.Caller(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return c, nil
}

func (s *AgentShellService) now() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// doTx runs fn in a top-level serializable transaction, retrying on transient
// serialization failures/deadlocks (DefaultRetryPolicy). fn is re-run from
// scratch on retry, so it MUST be safe to repeat: DB-only side effects and never
// synchronous external IO.
func (s *AgentShellService) doTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tx.DoSerializable(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// doTxRet is doTx for a transaction that returns a value. Same retry semantics:
// fn must be safe to re-run.
func doTxRet[T any](ctx context.Context, s *AgentShellService, fn func(ctx context.Context) (T, error)) (T, error) {
	return tx.DoSerializableRet(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

/*
	===== OpenShell =====
*/

// OpenShell attaches an interactive terminal to an agent. The REQUIRED first
// client frame is ShellStart; it carries the tenant + target the streaming auth
// interceptor already read. High-privilege (RESOURCE_AGENT_SHELL): every session
// is audited and scoped to the caller's tenant.
func (s *AgentShellService) OpenShell(stream grpc.BidiStreamingServer[api.ShellClientFrame, api.ShellServerFrame]) error {
	ctx := stream.Context()

	// 1. The first frame MUST be a ShellStart.
	first, err := stream.Recv()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return status.Error(codes.InvalidArgument, "stream closed before ShellStart")
		}
		return err
	}
	start := first.GetStart()
	if start == nil {
		return status.Error(codes.InvalidArgument, "first frame must be ShellStart")
	}
	if err := validateStart(start); err != nil {
		return err
	}

	// 2. Resolve caller identity for ownership/audit/scoping.
	c, err := s.caller(ctx)
	if err != nil {
		return err
	}

	// 3. Verify the declared tenant is one the caller may act in (admin bypasses),
	//    and that the target host exists under that tenant and has a live agent.
	if err := s.d.Tenants.EnsureMember(ctx, c.GetAccountId(), start.GetTenantId()); err != nil {
		return utils.MapErr(err)
	}
	if err := s.d.Machines.Locate(ctx, start.GetTenantId(), start.GetMachineId(), start.GetComponentId()); err != nil {
		return utils.MapErr(err)
	}

	// 4. Open the PTY session on the located agent.
	session, err := s.d.Hub.Open(ctx, start.GetMachineId(), &agent.OpenShell{
		RunId:       start.GetRunId(),
		ComponentId: start.GetComponentId(),
		Cols:        start.GetCols(),
		Rows:        start.GetRows(),
		Shell:       start.GetShell(),
	})
	if err != nil {
		return utils.MapErr(err)
	}
	defer func() { _ = session.Close() }()

	// 5. Audit the opened session (immutable record, commits transactionally).
	entry := &ShellAuditEntry{
		ID:          uuid.NewString(),
		SessionID:   session.SessionID(),
		AccountID:   c.GetAccountId(),
		TenantID:    start.GetTenantId(),
		RunID:       start.GetRunId(),
		MachineID:   start.GetMachineId(),
		ComponentID: start.GetComponentId(),
		Shell:       start.GetShell(),
		OpenedAt:    time.Now(),
	}
	if err := s.doTx(ctx, func(ctx context.Context) error {
		return utils.MapErr(s.d.Audit.Record(ctx, entry))
	}); err != nil {
		return err
	}

	// 6. Bridge bytes both directions until either side ends.
	return s.bridge(ctx, stream, session)
}

func validateStart(start *api.ShellStart) error {
	if start.GetTenantId() == "" {
		return status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	if start.GetMachineId() == "" {
		return status.Error(codes.InvalidArgument, "machine_id is required")
	}
	return nil
}

// bridge pumps admin->agent (stdin/resize/close) and agent->admin (stdout/stderr/
// exit) concurrently until one side terminates, then unwinds the other and stamps
// the audit row with the session outcome.
func (s *AgentShellService) bridge(
	ctx context.Context,
	stream grpc.BidiStreamingServer[api.ShellClientFrame, api.ShellServerFrame],
	session ShellSession,
) error {
	// agent -> admin: forward output frames; exit terminates the session.
	outErr := make(chan error, 1)
	var (
		exitCode int32
		exitMsg  string
	)
	go func() {
		outErr <- func() error {
			for {
				msg, err := session.Recv(ctx)
				if err != nil {
					if errors.Is(err, io.EOF) {
						return nil
					}
					return err
				}
				switch m := msg.GetMsg().(type) {
				case *agent.AgentShellMsg_Stdout:
					if err := stream.Send(&api.ShellServerFrame{
						Frame: &api.ShellServerFrame_Stdout{Stdout: m.Stdout},
					}); err != nil {
						return err
					}
				case *agent.AgentShellMsg_Stderr:
					if err := stream.Send(&api.ShellServerFrame{
						Frame: &api.ShellServerFrame_Stderr{Stderr: m.Stderr},
					}); err != nil {
						return err
					}
				case *agent.AgentShellMsg_Exit:
					exitCode = m.Exit.GetCode()
					exitMsg = m.Exit.GetError()
					_ = stream.Send(&api.ShellServerFrame{
						Frame: &api.ShellServerFrame_Exit{Exit: m.Exit},
					})
					return nil
				default:
					// Register and unknown frames are not part of the bridged
					// session output; ignore them.
				}
			}
		}()
	}()

	// admin -> agent: forward input/resize until close, EOF, or error.
	inErr := s.pumpClient(ctx, stream, session)

	// Whichever direction finished first, tear down and drain the other.
	_ = session.Close()
	bridgeErr := inErr
	if oerr := <-outErr; oerr != nil && bridgeErr == nil {
		bridgeErr = oerr
	}

	// Stamp the audit row with the outcome. Best-effort: never override the
	// bridge result with an audit write failure.
	_ = s.doTx(ctx, func(ctx context.Context) error {
		return s.d.Audit.End(ctx, session.SessionID(), exitCode, exitMsg, time.Now())
	})

	return bridgeErr
}

// pumpClient forwards admin client frames to the agent session. A `close` frame
// or a clean EOF ends the pump with no error; a second `start` is a protocol
// violation.
func (s *AgentShellService) pumpClient(
	ctx context.Context,
	stream grpc.BidiStreamingServer[api.ShellClientFrame, api.ShellServerFrame],
	session ShellSession,
) error {
	for {
		frame, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		switch f := frame.GetFrame().(type) {
		case *api.ShellClientFrame_Stdin:
			if err := session.Stdin(ctx, f.Stdin); err != nil {
				return utils.MapErr(err)
			}
		case *api.ShellClientFrame_Resize:
			if err := session.Resize(ctx, f.Resize.GetCols(), f.Resize.GetRows()); err != nil {
				return utils.MapErr(err)
			}
		case *api.ShellClientFrame_Close:
			return nil
		case *api.ShellClientFrame_Start:
			return status.Error(codes.InvalidArgument, "ShellStart already received")
		default:
			return status.Error(codes.InvalidArgument, "empty client frame")
		}
	}
}
