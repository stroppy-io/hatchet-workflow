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

// ConfigApplyHandler ships pre-rendered config files via PutFile to every
// target machine, then optionally issues a systemctl restart on each.
type ConfigApplyHandler struct {
	hub     HubPort
	timeout time.Duration
}

func NewConfigApplyHandler(hub HubPort, timeout time.Duration) *ConfigApplyHandler {
	if timeout == 0 {
		timeout = 30 * time.Minute
	}
	return &ConfigApplyHandler{hub: hub, timeout: timeout}
}

func (h *ConfigApplyHandler) Kind() string { return "ConfigApplyTask" }

func (h *ConfigApplyHandler) Execute(ctx context.Context, node *systempb.NodeRun, spec *anypb.Any, state nodeworker.StateStore) (*anypb.Any, error) {
	var task taskspb.ConfigApplyTask
	if err := anypb.UnmarshalTo(spec, &task, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("ConfigApplyHandler: unmarshal: %w", err)
	}
	targets := task.GetTargetMachineIds()
	if len(targets) == 0 {
		return nil, fmt.Errorf("ConfigApplyHandler: target_machine_ids required")
	}
	dagRunID := node.GetDagRunId().GetValue()

	for _, machineID := range targets {
		agentID, ok := h.hub.ResolveByMachine(dagRunID, machineID)
		if !ok {
			return nil, fmt.Errorf("ConfigApplyHandler: no agent for machine_id=%s", machineID)
		}
		for _, f := range task.GetFiles() {
			action := &agentpb.Action{
				Verb: &agentpb.Action_PutFile{
					PutFile: &agentpb.PutFile{
						Path:    f.GetPath(),
						Content: &agentpb.PutFile_Inline{Inline: []byte(f.GetInline())},
						Mode:    f.GetMode(),
						Owner:   f.GetOwner(),
					},
				},
			}
			report, err := h.hub.Dispatch(ctx, agentID, machineID, action, h.timeout)
			if err != nil {
				return nil, fmt.Errorf("ConfigApplyHandler: put_file %s on %s: %w", f.GetPath(), machineID, err)
			}
			if report.GetStatus() != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
				return nil, fmt.Errorf("ConfigApplyHandler: put_file %s on %s failed: %s", f.GetPath(), machineID, report.GetError())
			}
		}
		if task.GetRestartService() && task.GetUnitName() != "" {
			action := &agentpb.Action{
				Verb: &agentpb.Action_Systemctl{
					Systemctl: &agentpb.SystemctlOp{
						Unit: task.GetUnitName(),
						Verb: agentpb.SystemctlOp_VERB_RESTART,
					},
				},
			}
			report, err := h.hub.Dispatch(ctx, agentID, machineID, action, h.timeout)
			if err != nil {
				return nil, fmt.Errorf("ConfigApplyHandler: restart %s on %s: %w", task.GetUnitName(), machineID, err)
			}
			if report.GetStatus() != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
				return nil, fmt.Errorf("ConfigApplyHandler: restart %s on %s failed: %s", task.GetUnitName(), machineID, report.GetError())
			}
		}
	}
	return nil, nil
}
