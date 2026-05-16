package agent_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yaroher/ratel/pkg/pgx-ext/sqlexec"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/iam"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/core/configurator"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

type agentFixture struct {
	svc      *agentsvc.Service
	hub      *agentsvc.Hub
	iamSvc   *iam.Service
	tenantID string
}

func setupAgentFixture(t *testing.T) *agentFixture {
	t.Helper()
	f := fixture.New(t)
	executor := f.Executor.(*sqlexec.TxExecutor)

	iamSvc := iam.New(
		executor,
		f.TxMgr,
		f.Events,
		configurator.AuthConfig{AccessTTL: 15 * time.Minute, RefreshTTL: 720 * time.Hour},
		[]byte("test-jwt-secret-32-bytes-long!!"),
	)

	// Create user + tenant for FK satisfaction — use unique IDs to avoid conflicts across tests.
	uid := ids.New()
	ctx := context.Background()
	user, err := iamSvc.CreateUser(ctx, &iampb.User{
		Email:    fmt.Sprintf("agent-owner-%s@test.com", uid),
		Nickname: fmt.Sprintf("agentowner-%s", uid),
	}, "P@ssw0rd!")
	if err != nil {
		t.Fatalf("setupAgentFixture: create user: %v", err)
	}
	tenant, err := iamSvc.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: fmt.Sprintf("AgentTestTenant-%s", uid)}}, user.GetId())
	if err != nil {
		t.Fatalf("setupAgentFixture: create tenant: %v", err)
	}

	hub := agentsvc.NewHub()
	svc := agentsvc.New(executor, f.TxMgr, f.Events, hub)

	return &agentFixture{
		svc:      svc,
		hub:      hub,
		iamSvc:   iamSvc,
		tenantID: tenant.GetId().GetValue(),
	}
}

func TestService_RegisterAndPoll(t *testing.T) {
	af := setupAgentFixture(t)
	ctx := context.Background()

	token := af.svc.IssueBootstrap(af.tenantID, "dag-run-001")
	require.NotEmpty(t, token)

	ip := "1.2.3.4"
	resp, err := af.svc.Register(ctx, &agentpb.RegisterRequest{
		BootstrapToken: token,
		MachineId:      "machine-abc",
		Role:           catalogpb.MachineRole_MACHINE_ROLE_DATABASE,
		InternalIp:     "10.0.0.1",
		PublicIp:       &ip,
		AgentVersion:   "v0.1.0",
		Capabilities:   []string{"systemd", "apt"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.GetPollToken(), "poll_token must be non-empty")
	require.NotNil(t, resp.GetAgent())
	agentID := resp.GetAgent().GetId().GetValue()
	require.NotEmpty(t, agentID)

	// Poll with empty report → no commands
	batch, err := af.svc.Poll(ctx, &agentpb.PollRequest{
		AgentId: resp.GetAgent().GetId(),
		Report:  &agentpb.AgentReport{},
	})
	require.NoError(t, err)
	require.Empty(t, batch.GetCommands(), "no commands expected on empty queue")

	// Dispatch a command concurrently, then Poll returns it, then resolve.
	action := &agentpb.Action{
		Verb: &agentpb.Action_RunShell{
			RunShell: &agentpb.RunShell{Argv: []string{"hostname"}, Shell: true},
		},
	}

	dispatchDone := make(chan *agentpb.Report, 1)
	dispatchErrCh := make(chan error, 1)
	go func() {
		r, e := af.hub.Dispatch(ctx, agentID, "machine-abc", action, 10*time.Second)
		dispatchDone <- r
		dispatchErrCh <- e
	}()

	// Wait for the goroutine to enqueue
	time.Sleep(30 * time.Millisecond)

	// Poll — should return one command
	batch2, err := af.svc.Poll(ctx, &agentpb.PollRequest{
		AgentId: resp.GetAgent().GetId(),
		Report:  &agentpb.AgentReport{},
	})
	require.NoError(t, err)
	require.Len(t, batch2.GetCommands(), 1, "expected one pending command")
	cmd := batch2.GetCommands()[0]

	// Simulate agent completing the command by sending a report in next Poll
	report := &agentpb.Report{
		CommandId:  cmd.GetId(),
		MachineId:  "machine-abc",
		Status:     agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED,
		FinishedAt: timestamppb.Now(),
	}
	_, err = af.svc.Poll(ctx, &agentpb.PollRequest{
		AgentId: resp.GetAgent().GetId(),
		Report: &agentpb.AgentReport{
			Reports: []*agentpb.Report{report},
		},
	})
	require.NoError(t, err)

	// Dispatch should have completed
	select {
	case r := <-dispatchDone:
		require.NoError(t, <-dispatchErrCh)
		require.Equal(t, agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED, r.GetStatus())
	case <-time.After(5 * time.Second):
		t.Fatal("dispatch did not complete in time")
	}
}

func TestService_InvalidBootstrapToken(t *testing.T) {
	af := setupAgentFixture(t)
	ctx := context.Background()

	_, err := af.svc.Register(ctx, &agentpb.RegisterRequest{
		BootstrapToken: "invalid-token",
		MachineId:      "m-1",
		Role:           catalogpb.MachineRole_MACHINE_ROLE_DATABASE,
		InternalIp:     "10.0.0.1",
		AgentVersion:   "v0.1.0",
		Capabilities:   []string{},
	})
	require.Error(t, err, "invalid bootstrap token should fail")
}

func TestService_Deregister(t *testing.T) {
	af := setupAgentFixture(t)
	ctx := context.Background()

	token := af.svc.IssueBootstrap(af.tenantID, "dag-run-002")
	resp, err := af.svc.Register(ctx, &agentpb.RegisterRequest{
		BootstrapToken: token,
		MachineId:      "machine-deregister",
		Role:           catalogpb.MachineRole_MACHINE_ROLE_DATABASE,
		InternalIp:     "10.0.0.2",
		AgentVersion:   "v0.1.0",
		Capabilities:   []string{},
	})
	require.NoError(t, err)

	err = af.svc.Deregister(ctx, resp.GetAgent().GetId())
	require.NoError(t, err)
}
