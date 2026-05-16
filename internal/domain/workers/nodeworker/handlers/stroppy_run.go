package handlers

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	taskspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks"
)

// StroppyRunHandler dispatches a RunShell action for a StroppyRunTask via Hub.
type StroppyRunHandler struct {
	hub       HubPort
	agentID   string
	machineID string
	timeout   time.Duration
}

func NewStroppyRunHandler(hub HubPort, agentID, machineID string, timeout time.Duration) *StroppyRunHandler {
	if timeout == 0 {
		timeout = 2 * time.Hour
	}
	return &StroppyRunHandler{hub: hub, agentID: agentID, machineID: machineID, timeout: timeout}
}

func (h *StroppyRunHandler) Kind() string { return "StroppyRunTask" }

func (h *StroppyRunHandler) Execute(ctx context.Context, node *systempb.NodeRun, spec *anypb.Any, state nodeworker.StateStore) (*anypb.Any, error) {
	var task taskspb.StroppyRunTask
	if err := anypb.UnmarshalTo(spec, &task, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("StroppyRunHandler: unmarshal: %w", err)
	}
	// Build stroppy command from workload + version fields.
	// Full implementation resolves binary path from stroppybin and constructs argv.
	// Skeleton: run "stroppy" with version as argument.
	argv := []string{"stroppy", "--version=" + task.GetVersion()}
	action := &agentpb.Action{
		Verb: &agentpb.Action_RunShell{
			RunShell: &agentpb.RunShell{
				Argv:  argv,
				Shell: false,
			},
		},
	}
	report, err := h.hub.Dispatch(ctx, h.agentID, h.machineID, action, h.timeout)
	if err != nil {
		return nil, fmt.Errorf("StroppyRunHandler: dispatch: %w", err)
	}
	if report.GetStatus() != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
		return nil, fmt.Errorf("StroppyRunHandler: %s", report.GetError())
	}
	return nil, nil
}
