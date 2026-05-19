//go:build e2e

package e2e_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
)

func TestE2E_Agent_IssueBootstrap_RegisterPollDeregister_HappyPath(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "agent-flow")

	// Issue bootstrap token directly via service (no transport).
	const dagRunID = "01HZAGENT0E2EFLOW0AAAAAAAA"
	const machineID = "machine-A"
	tok, err := env.F.Agent.IssueBootstrap(ctx, tnt.GetValue(), dagRunID, machineID, "MACHINE_ROLE_STROPPY")
	require.NoError(t, err)
	require.NotEmpty(t, tok)

	cli := env.AnonClient
	reg, err := cli.Agent.Register(ctx, connect.NewRequest(&agentpb.RegisterRequest{
		BootstrapToken: tok, MachineId: machineID, Role: catalogpb.MachineRole_MACHINE_ROLE_STROPPY,
	}))
	require.NoError(t, err)
	require.NotEmpty(t, reg.Msg.GetPollToken())
	agentID := reg.Msg.GetAgent().GetId()
	require.NotEmpty(t, agentID.GetValue())

	// Initial poll returns empty command batch.
	pollResp, err := cli.Agent.Poll(ctx, connect.NewRequest(&agentpb.PollRequest{AgentId: agentID}))
	require.NoError(t, err)
	require.Empty(t, pollResp.Msg.GetCommands())

	// Dispatch from a separate goroutine; the next Poll should pick it up.
	dispatchDone := make(chan struct{})
	go func() {
		_, _ = env.F.AgentHub.Dispatch(ctx, agentID.GetValue(), machineID, &agentpb.Action{
			Verb: &agentpb.Action_RunShell{RunShell: &agentpb.RunShell{Argv: []string{"echo", "hi"}}},
		}, 5*time.Second)
		close(dispatchDone)
	}()

	// Poll a few times briefly until a command appears.
	deadline := time.Now().Add(3 * time.Second)
	var cmds []*agentpb.Command
	for time.Now().Before(deadline) {
		pp, err := cli.Agent.Poll(ctx, connect.NewRequest(&agentpb.PollRequest{AgentId: agentID}))
		require.NoError(t, err)
		if len(pp.Msg.GetCommands()) > 0 {
			cmds = pp.Msg.GetCommands()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.NotEmpty(t, cmds, "expected at least one dispatched command after Dispatch")

	// Send a report for the first command so Dispatch unblocks.
	report := &agentpb.Report{
		CommandId: cmds[0].GetId(),
		MachineId: machineID,
		Status:    agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED,
	}
	_, err = cli.Agent.Poll(ctx, connect.NewRequest(&agentpb.PollRequest{
		AgentId: agentID,
		Report:  &agentpb.AgentReport{Reports: []*agentpb.Report{report}},
	}))
	require.NoError(t, err)
	select {
	case <-dispatchDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Dispatch did not unblock after Report")
	}

	_, err = cli.Agent.Deregister(ctx, connect.NewRequest(agentID))
	require.NoError(t, err)
}

func TestE2E_Agent_AgentHub_ResolveByMachine_AfterRegister(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "hub-resolve")
	const dagRunID = "01HZHUB0RESOLVE000000AAAAA"
	const machineID = "m-resolve"
	tok, err := env.F.Agent.IssueBootstrap(ctx, tnt.GetValue(), dagRunID, machineID, "MACHINE_ROLE_STROPPY")
	require.NoError(t, err)
	cli := env.AnonClient
	reg, err := cli.Agent.Register(ctx, connect.NewRequest(&agentpb.RegisterRequest{
		BootstrapToken: tok, MachineId: machineID,
	}))
	require.NoError(t, err)

	got, ok := env.F.AgentHub.ResolveByMachine(dagRunID, machineID)
	require.True(t, ok)
	require.Equal(t, reg.Msg.GetAgent().GetId().GetValue(), got)
}

func TestE2E_Agent_Deregister_BindingClears(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "agent-dereg")
	const dagRunID = "01HZAGENT0DEREG00000000000"
	const machineID = "m-dereg"
	tok, err := env.F.Agent.IssueBootstrap(ctx, tnt.GetValue(), dagRunID, machineID, "MACHINE_ROLE_STROPPY")
	require.NoError(t, err)
	cli := env.AnonClient
	reg, err := cli.Agent.Register(ctx, connect.NewRequest(&agentpb.RegisterRequest{
		BootstrapToken: tok, MachineId: machineID,
	}))
	require.NoError(t, err)

	// Pre-deregister: binding exists.
	_, okBefore := env.F.AgentHub.ResolveByMachine(dagRunID, machineID)
	require.True(t, okBefore, "binding must exist before deregister")

	_, err = cli.Agent.Deregister(ctx, connect.NewRequest(reg.Msg.GetAgent().GetId()))
	require.NoError(t, err)

	// Post-deregister: Hub binding cleared so handlers don't dispatch to a
	// dead agent.
	_, okAfter := env.F.AgentHub.ResolveByMachine(dagRunID, machineID)
	require.False(t, okAfter, "binding must be cleared after Deregister")
}
