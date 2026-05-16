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

// OneShotHandler dispatches a RunShell action for an OneShotTask via Hub.
type OneShotHandler struct {
	hub       HubPort
	agentID   string
	machineID string
	timeout   time.Duration
}

func NewOneShotHandler(hub HubPort, agentID, machineID string, timeout time.Duration) *OneShotHandler {
	if timeout == 0 {
		timeout = 30 * time.Minute
	}
	return &OneShotHandler{hub: hub, agentID: agentID, machineID: machineID, timeout: timeout}
}

func (h *OneShotHandler) Kind() string { return "OneShotTask" }

func (h *OneShotHandler) Execute(ctx context.Context, node *systempb.NodeRun, spec *anypb.Any, state nodeworker.StateStore) (*anypb.Any, error) {
	var task taskspb.OneShotTask
	if err := anypb.UnmarshalTo(spec, &task, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("OneShotHandler: unmarshal: %w", err)
	}
	action := &agentpb.Action{
		Verb: &agentpb.Action_RunShell{
			RunShell: &agentpb.RunShell{
				Argv:  []string{"sh", "-c", task.GetCommandTemplate()},
				Shell: true,
			},
		},
	}
	report, err := h.hub.Dispatch(ctx, h.agentID, h.machineID, action, h.timeout)
	if err != nil {
		return nil, fmt.Errorf("OneShotHandler: dispatch: %w", err)
	}
	if report.GetStatus() != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
		return nil, fmt.Errorf("OneShotHandler: %s", report.GetError())
	}
	return nil, nil
}
