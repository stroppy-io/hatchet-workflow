package agent_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/agent"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
)

func TestHub_DispatchResolve(t *testing.T) {
	hub := agentsvc.NewHub()
	ctx := context.Background()

	action := &agentpb.Action{
		Verb: &agentpb.Action_RunShell{
			RunShell: &agentpb.RunShell{Argv: []string{"echo", "hello"}, Shell: true},
		},
	}

	var gotReport *agentpb.Report
	var dispatchErr error
	done := make(chan struct{})

	go func() {
		defer close(done)
		gotReport, dispatchErr = hub.Dispatch(ctx, "agent-1", "machine-1", action, 5*time.Second)
	}()

	// Give the goroutine time to enqueue
	time.Sleep(20 * time.Millisecond)

	// Drain and verify one command
	cmds := hub.Drain("agent-1")
	require.Len(t, cmds, 1, "expected one command in queue")
	cmd := cmds[0]
	require.NotEmpty(t, cmd.GetId())
	require.Equal(t, "machine-1", cmd.GetMachineId())

	// Simulate agent posting a report
	report := &agentpb.Report{
		CommandId:  cmd.GetId(),
		MachineId:  "machine-1",
		Status:     agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED,
		FinishedAt: timestamppb.Now(),
	}
	hub.Resolve(cmd.GetId(), report)

	<-done
	require.NoError(t, dispatchErr)
	require.NotNil(t, gotReport)
	require.Equal(t, agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED, gotReport.GetStatus())
}

func TestHub_DispatchContextCancel(t *testing.T) {
	hub := agentsvc.NewHub()
	ctx, cancel := context.WithCancel(context.Background())

	action := &agentpb.Action{
		Verb: &agentpb.Action_RunShell{
			RunShell: &agentpb.RunShell{Argv: []string{"sleep", "100"}},
		},
	}

	done := make(chan error, 1)
	go func() {
		_, err := hub.Dispatch(ctx, "agent-2", "machine-2", action, 30*time.Second)
		done <- err
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	err := <-done
	require.ErrorIs(t, err, context.Canceled)
}

func TestHub_DrainEmpty(t *testing.T) {
	hub := agentsvc.NewHub()
	cmds := hub.Drain("unknown-agent")
	require.Nil(t, cmds, "drain on unknown agent should return nil")
}
