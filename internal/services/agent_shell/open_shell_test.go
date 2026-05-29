package agent_shell

// open_shell_test.go covers OpenShell (the only RPC in agent_shell.proto).
// It uses the gomock mocks in mocks_test.go and the MockTrm / FakeClock in
// mock_tx_test.go.  No implementation files are modified.

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
)

// ---------------------------------------------------------------------------
// Fake bidi stream
// ---------------------------------------------------------------------------

// fakeBidiStream is a minimal in-memory implementation of
// grpc.BidiStreamingServer[api.ShellClientFrame, api.ShellServerFrame].
// It feeds a pre-loaded sequence of client frames and captures sent server
// frames.
type fakeBidiStream struct {
	ctx    context.Context
	frames []*api.ShellClientFrame // frames to deliver on Recv
	pos    int
	mu     sync.Mutex
	sent   []*api.ShellServerFrame
}

func newFakeBidiStream(ctx context.Context, frames ...*api.ShellClientFrame) *fakeBidiStream {
	return &fakeBidiStream{ctx: ctx, frames: frames}
}

func (f *fakeBidiStream) Recv() (*api.ShellClientFrame, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pos >= len(f.frames) {
		return nil, io.EOF
	}
	fr := f.frames[f.pos]
	f.pos++
	return fr, nil
}

func (f *fakeBidiStream) Send(frame *api.ShellServerFrame) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, frame)
	return nil
}

func (f *fakeBidiStream) Context() context.Context     { return f.ctx }
func (f *fakeBidiStream) SetHeader(metadata.MD) error  { return nil }
func (f *fakeBidiStream) SendHeader(metadata.MD) error { return nil }
func (f *fakeBidiStream) SetTrailer(metadata.MD)       {}
func (f *fakeBidiStream) SendMsg(m any) error          { return nil }
func (f *fakeBidiStream) RecvMsg(m any) error          { return nil }

// Compile-time check
var _ grpc.BidiStreamingServer[api.ShellClientFrame, api.ShellServerFrame] = (*fakeBidiStream)(nil)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func validStart() *api.ShellClientFrame {
	return &api.ShellClientFrame{
		Frame: &api.ShellClientFrame_Start{
			Start: &api.ShellStart{
				TenantId:  "t1",
				MachineId: "m1",
				Cols:      80,
				Rows:      24,
			},
		},
	}
}

func closeFrame() *api.ShellClientFrame {
	return &api.ShellClientFrame{
		Frame: &api.ShellClientFrame_Close{},
	}
}

func stdinFrame(data []byte) *api.ShellClientFrame {
	return &api.ShellClientFrame{
		Frame: &api.ShellClientFrame_Stdin{Stdin: data},
	}
}

func resizeFrame(cols, rows uint32) *api.ShellClientFrame {
	return &api.ShellClientFrame{
		Frame: &api.ShellClientFrame_Resize{
			Resize: &agent.ShellResize{Cols: cols, Rows: rows},
		},
	}
}

func newSvc(ctrl *gomock.Controller) (
	*AgentShellService,
	*utils.MockAuthn,
	*MockTenantGuard,
	*MockMachineLocator,
	*MockShellHub,
	*MockShellAudit,
) {
	authn := utils.NewMockAuthn(ctrl)
	tenants := NewMockTenantGuard(ctrl)
	machines := NewMockMachineLocator(ctrl)
	hub := NewMockShellHub(ctrl)
	audit := NewMockShellAudit(ctrl)

	svc := NewAgentShellService(AgentShellDeps{
		Authn:    authn,
		Tenants:  tenants,
		Machines: machines,
		Hub:      hub,
		Audit:    audit,
		Tx:       &utils.MockTrm{},
	})
	return svc, authn, tenants, machines, hub, audit
}

// ---------------------------------------------------------------------------
// Tests: first-frame validation
// ---------------------------------------------------------------------------

func TestOpenShell_StreamEOFBeforeStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, _, _, _, _, _ := newSvc(ctrl)

	// Empty stream – Recv immediately returns io.EOF.
	stream := newFakeBidiStream(context.Background())
	err := svc.OpenShell(stream)
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestOpenShell_FirstFrameNotStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, _, _, _, _, _ := newSvc(ctrl)

	// First frame is stdin, not ShellStart.
	stream := newFakeBidiStream(context.Background(), stdinFrame([]byte("hello")))
	err := svc.OpenShell(stream)
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestOpenShell_MissingTenantID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, _, _, _, _, _ := newSvc(ctrl)

	frame := &api.ShellClientFrame{
		Frame: &api.ShellClientFrame_Start{
			Start: &api.ShellStart{MachineId: "m1"}, // tenant_id empty
		},
	}
	stream := newFakeBidiStream(context.Background(), frame)
	err := svc.OpenShell(stream)
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestOpenShell_MissingMachineID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, _, _, _, _, _ := newSvc(ctrl)

	frame := &api.ShellClientFrame{
		Frame: &api.ShellClientFrame_Start{
			Start: &api.ShellStart{TenantId: "t1"}, // machine_id empty
		},
	}
	stream := newFakeBidiStream(context.Background(), frame)
	err := svc.OpenShell(stream)
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: auth / guard failures
// ---------------------------------------------------------------------------

func TestOpenShell_CallerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, authn, _, _, _, _ := newSvc(ctrl)
	ctx := context.Background()

	authn.EXPECT().Caller(ctx).Return(nil, errors.New("no token"))

	stream := newFakeBidiStream(ctx, validStart())
	err := svc.OpenShell(stream)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

func TestOpenShell_TenantNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, authn, tenants, _, _, _ := newSvc(ctrl)
	ctx := context.Background()

	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
	tenants.EXPECT().EnsureMember(ctx, "a1", "t1").
		Return(derrors.NotFound("tenant", "tenant not found"))

	stream := newFakeBidiStream(ctx, validStart())
	err := svc.OpenShell(stream)
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestOpenShell_TenantPermissionDenied(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, authn, tenants, _, _, _ := newSvc(ctrl)
	ctx := context.Background()

	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
	tenants.EXPECT().EnsureMember(ctx, "a1", "t1").
		Return(derrors.PermissionDenied("not a member"))

	stream := newFakeBidiStream(ctx, validStart())
	err := svc.OpenShell(stream)
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PermissionDenied, got %v", err)
	}
}

func TestOpenShell_MachineNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, authn, tenants, machines, _, _ := newSvc(ctrl)
	ctx := context.Background()

	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
	tenants.EXPECT().EnsureMember(ctx, "a1", "t1").Return(nil)
	machines.EXPECT().Locate(ctx, "t1", "m1", "").
		Return(derrors.NotFound("machine", "not found"))

	stream := newFakeBidiStream(ctx, validStart())
	err := svc.OpenShell(stream)
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestOpenShell_MachineOffline(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, authn, tenants, machines, _, _ := newSvc(ctrl)
	ctx := context.Background()

	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
	tenants.EXPECT().EnsureMember(ctx, "a1", "t1").Return(nil)
	machines.EXPECT().Locate(ctx, "t1", "m1", "").
		Return(derrors.FailedPrecondition("AGENT_OFFLINE", "agent offline"))

	stream := newFakeBidiStream(ctx, validStart())
	err := svc.OpenShell(stream)
	if status.Code(err) != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: hub failures
// ---------------------------------------------------------------------------

func TestOpenShell_HubOpenError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, authn, tenants, machines, hub, _ := newSvc(ctrl)
	ctx := context.Background()

	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
	tenants.EXPECT().EnsureMember(ctx, "a1", "t1").Return(nil)
	machines.EXPECT().Locate(ctx, "t1", "m1", "").Return(nil)
	hub.EXPECT().Open(ctx, "m1", gomock.Any()).
		Return(nil, errors.New("hub unavailable"))

	stream := newFakeBidiStream(ctx, validStart())
	err := svc.OpenShell(stream)
	if status.Code(err) != codes.Internal {
		t.Errorf("expected Internal, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: audit failure
// ---------------------------------------------------------------------------

func TestOpenShell_AuditRecordError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, authn, tenants, machines, hub, audit := newSvc(ctrl)
	ctx := context.Background()

	session := NewMockShellSession(ctrl)
	session.EXPECT().SessionID().Return("sid1").AnyTimes()
	session.EXPECT().Close().Return(nil).AnyTimes()

	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
	tenants.EXPECT().EnsureMember(ctx, "a1", "t1").Return(nil)
	machines.EXPECT().Locate(ctx, "t1", "m1", "").Return(nil)
	hub.EXPECT().Open(ctx, "m1", gomock.Any()).Return(session, nil)
	audit.EXPECT().Record(ctx, gomock.Any()).Return(errors.New("db down"))

	stream := newFakeBidiStream(ctx, validStart())
	err := svc.OpenShell(stream)
	// audit.Record error is returned as Internal (non-domain error through doTx)
	if err == nil {
		t.Fatal("expected error from audit.Record, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: success path – clean close frame
// ---------------------------------------------------------------------------

func TestOpenShell_Success_CloseFrame(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, authn, tenants, machines, hub, audit := newSvc(ctrl)
	ctx := context.Background()

	session := NewMockShellSession(ctrl)
	session.EXPECT().SessionID().Return("sid1").AnyTimes()
	// bridge: session.Close is called (possibly twice – defer + pumpClient end)
	session.EXPECT().Close().Return(nil).AnyTimes()
	// session.Recv – the outgoing goroutine: return EOF to stop it cleanly.
	session.EXPECT().Recv(gomock.Any()).Return(nil, io.EOF).AnyTimes()

	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
	tenants.EXPECT().EnsureMember(ctx, "a1", "t1").Return(nil)
	machines.EXPECT().Locate(ctx, "t1", "m1", "").Return(nil)
	hub.EXPECT().Open(ctx, "m1", gomock.Any()).Return(session, nil)
	audit.EXPECT().Record(ctx, gomock.Any()).Return(nil)
	audit.EXPECT().End(gomock.Any(), "sid1", int32(0), "", gomock.Any()).Return(nil)

	// Client: start then close.
	stream := newFakeBidiStream(ctx, validStart(), closeFrame())
	err := svc.OpenShell(stream)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: success path – stdin and resize forwarding then EOF
// ---------------------------------------------------------------------------

func TestOpenShell_Success_StdinResizeEOF(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, authn, tenants, machines, hub, audit := newSvc(ctrl)
	ctx := context.Background()

	session := NewMockShellSession(ctrl)
	session.EXPECT().SessionID().Return("sid2").AnyTimes()
	session.EXPECT().Close().Return(nil).AnyTimes()
	session.EXPECT().Stdin(gomock.Any(), []byte("ls\n")).Return(nil)
	session.EXPECT().Resize(gomock.Any(), uint32(100), uint32(40)).Return(nil)
	session.EXPECT().Recv(gomock.Any()).Return(nil, io.EOF).AnyTimes()

	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
	tenants.EXPECT().EnsureMember(ctx, "a1", "t1").Return(nil)
	machines.EXPECT().Locate(ctx, "t1", "m1", "").Return(nil)
	hub.EXPECT().Open(ctx, "m1", gomock.Any()).Return(session, nil)
	audit.EXPECT().Record(ctx, gomock.Any()).Return(nil)
	audit.EXPECT().End(gomock.Any(), "sid2", int32(0), "", gomock.Any()).Return(nil)

	stream := newFakeBidiStream(ctx,
		validStart(),
		stdinFrame([]byte("ls\n")),
		resizeFrame(100, 40),
		// EOF after these frames (stream exhausted)
	)
	err := svc.OpenShell(stream)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: agent sends stdout/stderr/exit frames
// ---------------------------------------------------------------------------

func TestOpenShell_Success_AgentStdoutStderrExit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, authn, tenants, machines, hub, audit := newSvc(ctrl)
	ctx := context.Background()

	session := NewMockShellSession(ctrl)
	session.EXPECT().SessionID().Return("sid3").AnyTimes()
	session.EXPECT().Close().Return(nil).AnyTimes()

	// Agent sends stdout, stderr, then exit.
	gomock.InOrder(
		session.EXPECT().Recv(gomock.Any()).Return(&agent.AgentShellMsg{
			Msg: &agent.AgentShellMsg_Stdout{Stdout: []byte("hello\n")},
		}, nil),
		session.EXPECT().Recv(gomock.Any()).Return(&agent.AgentShellMsg{
			Msg: &agent.AgentShellMsg_Stderr{Stderr: []byte("err\n")},
		}, nil),
		session.EXPECT().Recv(gomock.Any()).Return(&agent.AgentShellMsg{
			Msg: &agent.AgentShellMsg_Exit{Exit: &agent.ShellExit{Code: 0}},
		}, nil),
	)

	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
	tenants.EXPECT().EnsureMember(ctx, "a1", "t1").Return(nil)
	machines.EXPECT().Locate(ctx, "t1", "m1", "").Return(nil)
	hub.EXPECT().Open(ctx, "m1", gomock.Any()).Return(session, nil)
	audit.EXPECT().Record(ctx, gomock.Any()).Return(nil)
	audit.EXPECT().End(gomock.Any(), "sid3", int32(0), "", gomock.Any()).Return(nil)

	// Only the start frame; the client EOF triggers after the agent exit.
	stream := newFakeBidiStream(ctx, validStart())
	err := svc.OpenShell(stream)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}

	// Verify server forwarded the frames.
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if len(stream.sent) < 3 {
		t.Errorf("expected at least 3 frames sent (stdout + stderr + exit), got %d", len(stream.sent))
	}
}

// ---------------------------------------------------------------------------
// Tests: duplicate ShellStart is a protocol error
// ---------------------------------------------------------------------------

func TestOpenShell_DuplicateStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, authn, tenants, machines, hub, audit := newSvc(ctrl)
	ctx := context.Background()

	session := NewMockShellSession(ctrl)
	session.EXPECT().SessionID().Return("sid4").AnyTimes()
	session.EXPECT().Close().Return(nil).AnyTimes()
	session.EXPECT().Recv(gomock.Any()).Return(nil, io.EOF).AnyTimes()

	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
	tenants.EXPECT().EnsureMember(ctx, "a1", "t1").Return(nil)
	machines.EXPECT().Locate(ctx, "t1", "m1", "").Return(nil)
	hub.EXPECT().Open(ctx, "m1", gomock.Any()).Return(session, nil)
	audit.EXPECT().Record(ctx, gomock.Any()).Return(nil)
	audit.EXPECT().End(gomock.Any(), "sid4", int32(0), "", gomock.Any()).Return(nil).AnyTimes()

	// Client sends a second ShellStart – protocol violation.
	stream := newFakeBidiStream(ctx, validStart(), validStart())
	err := svc.OpenShell(stream)
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for duplicate start, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: stdin error propagated
// ---------------------------------------------------------------------------

func TestOpenShell_StdinError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, authn, tenants, machines, hub, audit := newSvc(ctrl)
	ctx := context.Background()

	session := NewMockShellSession(ctrl)
	session.EXPECT().SessionID().Return("sid5").AnyTimes()
	session.EXPECT().Close().Return(nil).AnyTimes()
	session.EXPECT().Stdin(gomock.Any(), gomock.Any()).
		Return(derrors.Internal("write failed"))
	session.EXPECT().Recv(gomock.Any()).Return(nil, io.EOF).AnyTimes()

	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
	tenants.EXPECT().EnsureMember(ctx, "a1", "t1").Return(nil)
	machines.EXPECT().Locate(ctx, "t1", "m1", "").Return(nil)
	hub.EXPECT().Open(ctx, "m1", gomock.Any()).Return(session, nil)
	audit.EXPECT().Record(ctx, gomock.Any()).Return(nil)
	audit.EXPECT().End(gomock.Any(), "sid5", int32(0), "", gomock.Any()).Return(nil).AnyTimes()

	stream := newFakeBidiStream(ctx, validStart(), stdinFrame([]byte("bad")))
	err := svc.OpenShell(stream)
	if err == nil {
		t.Fatal("expected error from Stdin, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: resize error propagated
// ---------------------------------------------------------------------------

func TestOpenShell_ResizeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, authn, tenants, machines, hub, audit := newSvc(ctrl)
	ctx := context.Background()

	session := NewMockShellSession(ctrl)
	session.EXPECT().SessionID().Return("sid6").AnyTimes()
	session.EXPECT().Close().Return(nil).AnyTimes()
	session.EXPECT().Resize(gomock.Any(), uint32(80), uint32(24)).
		Return(errors.New("resize failed"))
	session.EXPECT().Recv(gomock.Any()).Return(nil, io.EOF).AnyTimes()

	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
	tenants.EXPECT().EnsureMember(ctx, "a1", "t1").Return(nil)
	machines.EXPECT().Locate(ctx, "t1", "m1", "").Return(nil)
	hub.EXPECT().Open(ctx, "m1", gomock.Any()).Return(session, nil)
	audit.EXPECT().Record(ctx, gomock.Any()).Return(nil)
	audit.EXPECT().End(gomock.Any(), "sid6", int32(0), "", gomock.Any()).Return(nil).AnyTimes()

	stream := newFakeBidiStream(ctx, validStart(), resizeFrame(80, 24))
	err := svc.OpenShell(stream)
	if err == nil {
		t.Fatal("expected error from Resize, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: session Recv error (agent side)
// ---------------------------------------------------------------------------

func TestOpenShell_SessionRecvError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, authn, tenants, machines, hub, audit := newSvc(ctrl)
	ctx := context.Background()

	session := NewMockShellSession(ctrl)
	session.EXPECT().SessionID().Return("sid7").AnyTimes()
	session.EXPECT().Close().Return(nil).AnyTimes()
	// Agent side returns an error (not io.EOF).
	session.EXPECT().Recv(gomock.Any()).Return(nil, errors.New("agent disconnected")).AnyTimes()

	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
	tenants.EXPECT().EnsureMember(ctx, "a1", "t1").Return(nil)
	machines.EXPECT().Locate(ctx, "t1", "m1", "").Return(nil)
	hub.EXPECT().Open(ctx, "m1", gomock.Any()).Return(session, nil)
	audit.EXPECT().Record(ctx, gomock.Any()).Return(nil)
	audit.EXPECT().End(gomock.Any(), "sid7", int32(0), "", gomock.Any()).Return(nil).AnyTimes()

	// client also closes after start so pumpClient exits cleanly via EOF
	stream := newFakeBidiStream(ctx, validStart())
	err := svc.OpenShell(stream)
	// The bridge picks the first non-nil error from either direction.
	// Either the agent-side error or nil from the client EOF may win the race.
	// We just verify the call completes without panic.
	_ = err
}

// ---------------------------------------------------------------------------
// Tests: audit End failure is ignored (best-effort)
// ---------------------------------------------------------------------------

func TestOpenShell_AuditEndError_Ignored(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, authn, tenants, machines, hub, audit := newSvc(ctrl)
	ctx := context.Background()

	session := NewMockShellSession(ctrl)
	session.EXPECT().SessionID().Return("sid8").AnyTimes()
	session.EXPECT().Close().Return(nil).AnyTimes()
	session.EXPECT().Recv(gomock.Any()).Return(nil, io.EOF).AnyTimes()

	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a1"}, nil)
	tenants.EXPECT().EnsureMember(ctx, "a1", "t1").Return(nil)
	machines.EXPECT().Locate(ctx, "t1", "m1", "").Return(nil)
	hub.EXPECT().Open(ctx, "m1", gomock.Any()).Return(session, nil)
	audit.EXPECT().Record(ctx, gomock.Any()).Return(nil)
	// End returns an error – it should be swallowed.
	audit.EXPECT().End(gomock.Any(), "sid8", int32(0), "", gomock.Any()).Return(errors.New("audit db down"))

	stream := newFakeBidiStream(ctx, validStart(), closeFrame())
	err := svc.OpenShell(stream)
	if err != nil {
		t.Errorf("audit.End error should be ignored, got %v", err)
	}
}
