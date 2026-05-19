package handlers

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	taskspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks"
)

// StroppyRunHandler dispatches `stroppy run --config <path>` on the
// stroppy worker resolved via TargetMachineIds. The config path is read
// from the DagRun state-store under ConfigPathStateKey (the upstream
// render-stroppy-config node writes it).
type StroppyRunHandler struct {
	hub     HubPort
	log     *zap.Logger
	timeout time.Duration
}

func NewStroppyRunHandler(hub HubPort, log *zap.Logger, timeout time.Duration) *StroppyRunHandler {
	if timeout == 0 {
		timeout = 2 * time.Hour
	}
	return &StroppyRunHandler{hub: hub, log: log, timeout: timeout}
}

func (h *StroppyRunHandler) Kind() string { return "StroppyRunTask" }

func (h *StroppyRunHandler) Execute(ctx context.Context, node *systempb.NodeRun, spec *anypb.Any, state nodeworker.StateStore) (*anypb.Any, error) {
	var task taskspb.StroppyRunTask
	if err := anypb.UnmarshalTo(spec, &task, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("StroppyRunHandler: unmarshal: %w", err)
	}
	targets := task.GetTargetMachineIds()
	if len(targets) == 0 {
		return nil, fmt.Errorf("StroppyRunHandler: target_machine_ids required")
	}

	// Default config path when the upstream render node didn't publish one.
	configPath := "/tmp/stroppy.cfg.json"
	if key := task.GetConfigPathStateKey(); key != "" {
		if v, ok, err := state.Get(ctx, key); err == nil && ok && v != nil {
			var sv structpb.Value
			if err := anypb.UnmarshalTo(v, &sv, proto.UnmarshalOptions{}); err == nil {
				if s := sv.GetStringValue(); s != "" {
					configPath = s
				}
			}
		}
	}

	argv := []string{"stroppy", "run", "--config", configPath}
	action := &agentpb.Action{
		Verb: &agentpb.Action_RunShell{
			RunShell: &agentpb.RunShell{Argv: argv, Shell: false},
		},
	}

	dagRunID := node.GetDagRunId().GetValue()
	for _, machineID := range targets {
		agentID, ok := h.hub.ResolveByMachine(dagRunID, machineID)
		if !ok {
			return nil, fmt.Errorf("StroppyRunHandler: no agent for machine_id=%s", machineID)
		}
		report, err := h.hub.Dispatch(ctx, agentID, machineID, action, h.timeout)
		if err != nil {
			return nil, fmt.Errorf("StroppyRunHandler: dispatch %s: %w", machineID, err)
		}
		if report.GetStatus() != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
			return nil, fmt.Errorf("StroppyRunHandler: %s failed: %s", machineID, report.GetError())
		}
	}
	return nil, nil
}
