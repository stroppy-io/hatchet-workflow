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

// ConfigApplyHandler applies a ConfigApplyTask: sends each config file via PutFile,
// then optionally runs a restart action via RunShell if RestartService is set.
type ConfigApplyHandler struct {
	hub       HubPort
	agentID   string
	machineID string
	timeout   time.Duration
}

func NewConfigApplyHandler(hub HubPort, agentID, machineID string, timeout time.Duration) *ConfigApplyHandler {
	if timeout == 0 {
		timeout = 30 * time.Minute
	}
	return &ConfigApplyHandler{hub: hub, agentID: agentID, machineID: machineID, timeout: timeout}
}

func (h *ConfigApplyHandler) Kind() string { return "ConfigApplyTask" }

func (h *ConfigApplyHandler) Execute(ctx context.Context, node *systempb.NodeRun, spec *anypb.Any, state nodeworker.StateStore) (*anypb.Any, error) {
	var task taskspb.ConfigApplyTask
	if err := anypb.UnmarshalTo(spec, &task, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("ConfigApplyHandler: unmarshal: %w", err)
	}
	// PutFile for each config file (inline content only in skeleton).
	for _, f := range task.GetFiles() {
		putAction := &agentpb.Action{
			Verb: &agentpb.Action_PutFile{
				PutFile: &agentpb.PutFile{
					Path:    f.GetPath(),
					Content: &agentpb.PutFile_Inline{Inline: []byte(f.GetInline())},
				},
			},
		}
		report, err := h.hub.Dispatch(ctx, h.agentID, h.machineID, putAction, h.timeout)
		if err != nil {
			return nil, fmt.Errorf("ConfigApplyHandler: put_file %s: %w", f.GetPath(), err)
		}
		if report.GetStatus() != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
			return nil, fmt.Errorf("ConfigApplyHandler: put_file %s failed: %s", f.GetPath(), report.GetError())
		}
	}
	// Restart service via systemctl if requested.
	if task.GetRestartService() && task.GetUnitName() != "" {
		restartAction := &agentpb.Action{
			Verb: &agentpb.Action_Systemctl{
				Systemctl: &agentpb.SystemctlOp{
					Unit: task.GetUnitName(),
					Verb: agentpb.SystemctlOp_VERB_RESTART,
				},
			},
		}
		report, err := h.hub.Dispatch(ctx, h.agentID, h.machineID, restartAction, h.timeout)
		if err != nil {
			return nil, fmt.Errorf("ConfigApplyHandler: systemctl restart %s: %w", task.GetUnitName(), err)
		}
		if report.GetStatus() != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
			return nil, fmt.Errorf("ConfigApplyHandler: systemctl restart %s failed: %s", task.GetUnitName(), report.GetError())
		}
	}
	return nil, nil
}
