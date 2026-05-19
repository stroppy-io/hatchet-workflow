package handlers

import (
	"bytes"
	"context"
	"fmt"
	"text/template"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	taskspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks"
)

// OneShotHandler renders a shell template and runs it on every machine in
// TargetMachineIds. Templating uses text/template with TemplateVars; the
// rendered command is dispatched via Action_RunShell{shell:true}.
type OneShotHandler struct {
	hub     HubPort
	timeout time.Duration
}

func NewOneShotHandler(hub HubPort, timeout time.Duration) *OneShotHandler {
	if timeout == 0 {
		timeout = 30 * time.Minute
	}
	return &OneShotHandler{hub: hub, timeout: timeout}
}

func (h *OneShotHandler) Kind() string { return "OneShotTask" }

func (h *OneShotHandler) Execute(ctx context.Context, node *systempb.NodeRun, spec *anypb.Any, state nodeworker.StateStore) (*anypb.Any, error) {
	var task taskspb.OneShotTask
	if err := anypb.UnmarshalTo(spec, &task, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("OneShotHandler: unmarshal: %w", err)
	}
	targets := task.GetTargetMachineIds()
	if len(targets) == 0 {
		return nil, fmt.Errorf("OneShotHandler: target_machine_ids required")
	}

	mergedVars, err := mergeStateVars(ctx, task.GetTemplateVars(), task.GetStateVarRefs(), state)
	if err != nil {
		return nil, fmt.Errorf("OneShotHandler: resolve state vars: %w", err)
	}

	cmd, err := renderTemplate(task.GetCommandTemplate(), mergedVars)
	if err != nil {
		return nil, fmt.Errorf("OneShotHandler: render: %w", err)
	}

	action := &agentpb.Action{
		Verb: &agentpb.Action_RunShell{
			RunShell: &agentpb.RunShell{
				Argv:  []string{"sh", "-c", cmd},
				Shell: true,
			},
		},
	}

	dagRunID := node.GetDagRunId().GetValue()
	for _, machineID := range targets {
		agentID, ok := h.hub.ResolveByMachine(dagRunID, machineID)
		if !ok {
			return nil, fmt.Errorf("OneShotHandler: no agent for machine_id=%s", machineID)
		}
		report, err := h.hub.Dispatch(ctx, agentID, machineID, action, h.timeout)
		if err != nil {
			return nil, fmt.Errorf("OneShotHandler: dispatch %s: %w", machineID, err)
		}
		if report.GetStatus() != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
			return nil, fmt.Errorf("OneShotHandler: %s failed: %s", machineID, report.GetError())
		}
	}

	if pc := task.GetPostCheckCommand(); pc != "" {
		pcCmd, err := renderTemplate(pc, mergedVars)
		if err != nil {
			return nil, fmt.Errorf("OneShotHandler: render post_check: %w", err)
		}
		pcAction := &agentpb.Action{
			Verb: &agentpb.Action_RunShell{
				RunShell: &agentpb.RunShell{
					Argv:  []string{"sh", "-c", pcCmd},
					Shell: true,
				},
			},
		}
		retries := int(task.GetPostCheckRetries())
		delay := time.Duration(task.GetPostCheckDelaySeconds()) * time.Second
		for _, machineID := range targets {
			agentID, ok := h.hub.ResolveByMachine(dagRunID, machineID)
			if !ok {
				return nil, fmt.Errorf("OneShotHandler: no agent for machine_id=%s (post_check)", machineID)
			}
			var lastErr error
			for attempt := 0; attempt <= retries; attempt++ {
				if attempt > 0 && delay > 0 {
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-time.After(delay):
					}
				}
				report, err := h.hub.Dispatch(ctx, agentID, machineID, pcAction, h.timeout)
				if err == nil && report.GetStatus() == agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
					lastErr = nil
					break
				}
				if err != nil {
					lastErr = err
				} else {
					lastErr = fmt.Errorf("agent reported %s: %s", report.GetStatus(), report.GetError())
				}
			}
			if lastErr != nil {
				return nil, fmt.Errorf("OneShotHandler: post_check failed on %s after %d retries: %w", machineID, retries, lastErr)
			}
		}
	}
	return nil, nil
}

// mergeStateVars returns a fresh map that combines the static template_vars
// with values read from the state-store under the configured keys. State-store
// values are decoded as structpb.Value and stringified via GetStringValue;
// missing keys silently resolve to the empty string.
func mergeStateVars(ctx context.Context, static map[string]string, refs map[string]string, state nodeworker.StateStore) (map[string]string, error) {
	out := make(map[string]string, len(static)+len(refs))
	for k, v := range static {
		out[k] = v
	}
	for varName, stateKey := range refs {
		v, ok, err := state.Get(ctx, stateKey)
		if err != nil {
			return nil, fmt.Errorf("read state %s: %w", stateKey, err)
		}
		if !ok || v == nil {
			out[varName] = ""
			continue
		}
		var sv structpb.Value
		if err := anypb.UnmarshalTo(v, &sv, proto.UnmarshalOptions{}); err != nil {
			out[varName] = ""
			continue
		}
		out[varName] = sv.GetStringValue()
	}
	return out, nil
}

func renderTemplate(tpl string, vars map[string]string) (string, error) {
	if len(vars) == 0 {
		return tpl, nil
	}
	t, err := template.New("oneshot").Option("missingkey=zero").Parse(tpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return "", err
	}
	return buf.String(), nil
}
